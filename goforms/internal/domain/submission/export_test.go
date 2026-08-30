package submission_test

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/submission"
)

func exportRecord(data model.JSON) submission.ExportRecord {
	formID := uuid.NewString()
	return submission.ExportRecord{Submission: &model.FormSubmission{ID: uuid.NewString(), FormID: formID,
		SchemaVersion: 1, RequestID: "req_fixture", Status: model.SubmissionStatusAccepted,
		SubmittedAt: time.Date(2026, 8, 30, 1, 2, 3, 123456000, time.UTC), Data: data},
		SchemaFormID: formID, AcceptedVersion: 1, Policy: model.JSON{submission.SensitiveAnnotation: []string{"/secret"}}}
}

func TestExportUsesSharedRedactionAndExactNumbers(t *testing.T) {
	var data model.JSON
	require.NoError(t, json.Unmarshal([]byte(`{"secret":"private-canary","number":9007199254740993,"nested":{"decimal":0.1234567890123456789},"array":[1,null,"ok"]}`), &data))
	record := exportRecord(data)
	meta := submission.ExportMeta{ExportID: uuid.NewString(), PreparedAt: time.Now().UTC()}
	encoded, err := submission.EncodeExport(t.Context(), submission.ExportJSON, []submission.ExportRecord{record}, meta)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private-canary")
	require.Contains(t, string(encoded), "9007199254740993")
	require.Contains(t, string(encoded), "0.1234567890123456789")
	var document struct {
		Data []submission.Projection `json:"data"`
		Meta submission.ExportMeta   `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(encoded, &document))
	require.Len(t, document.Data, 1)
	require.Equal(t, 1, document.Meta.RowCount)
	require.Equal(t, meta.ExportID, document.Meta.ExportID)
	require.Equal(t, []string{"/secret"}, document.Data[0].RedactedPaths)
	require.Equal(t, "private-canary", record.Submission.Data["secret"], "Encoding never rewrites accepted storage")
}

func TestCSVQuotesAndNeutralizesEveryCellIncludingHeaders(t *testing.T) {
	data := model.JSON{"secret": "private-canary", "empty": "", "number": json.Number("9007199254740993"),
		"nested": map[string]any{"a/b~": "visible"}, "array": []any{json.Number("9007199254740993"), nil}}
	for index, value := range []string{"=1+1", "+1", "-1", "@SUM(1)", "\t=1", "\r=1", "\n=1", "＝1", "＋1", "－1", "＠1", `=1+2";=1+2`, "a,\"\n=1"} {
		data[fmt.Sprintf("=header-%d,\"\n", index)] = value
	}
	encoded, err := submission.EncodeExport(t.Context(), submission.ExportCSV, []submission.ExportRecord{exportRecord(data)}, submission.ExportMeta{})
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private-canary")
	rows, err := csv.NewReader(strings.NewReader(string(encoded))).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Len(t, rows[1], len(rows[0]), "Embedded delimiters and newlines must not create new cells or rows")
	for _, row := range rows {
		for _, cell := range row {
			require.True(t, strings.HasPrefix(cell, "'"), "every cell is text-prefixed")
		}
	}
	require.Contains(t, rows[0], "'data:/nested/a~1b~0")
	require.Contains(t, rows[1], "'9007199254740993")
	require.Contains(t, rows[1], "'[9007199254740993,null]")
	require.Contains(t, rows[1], "'=1+1")
}

func TestExportLimitsCancellationAndPrivacyFailuresReturnNoPartialBytes(t *testing.T) {
	_, err := submission.EncodeExport(t.Context(), "xml", nil, submission.ExportMeta{})
	require.ErrorIs(t, err, submission.ErrExportRequest)
	encoded, err := submission.EncodeExport(t.Context(), submission.ExportJSON, make([]submission.ExportRecord, submission.MaxExportRows+1), submission.ExportMeta{})
	require.ErrorIs(t, err, submission.ErrExportLimit)
	require.Nil(t, encoded)
	for _, format := range []submission.ExportFormat{submission.ExportJSON, submission.ExportCSV} {
		record := exportRecord(model.JSON{"value": strings.Repeat("x", submission.MaxExportBytes)})
		encoded, err := submission.EncodeExport(t.Context(), format, []submission.ExportRecord{record}, submission.ExportMeta{})
		require.ErrorIs(t, err, submission.ErrExportLimit)
		require.Nil(t, encoded)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		encoded, err = submission.EncodeExport(ctx, format, nil, submission.ExportMeta{})
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, encoded)
		good := exportRecord(model.JSON{"visible": "must-not-be-released"})
		bad := exportRecord(model.JSON{"secret": "private-canary"})
		bad.AcceptedVersion = 2
		encoded, err = submission.EncodeExport(t.Context(), format, []submission.ExportRecord{good, bad}, submission.ExportMeta{})
		require.ErrorIs(t, err, submission.ErrRedactionPolicy)
		require.Nil(t, encoded)
	}
	fields := model.JSON{}
	for index := range submission.MaxExportColumns {
		fields[fmt.Sprint(index)] = "value"
	}
	encoded, err = submission.EncodeExport(t.Context(), submission.ExportCSV, []submission.ExportRecord{exportRecord(fields)}, submission.ExportMeta{})
	require.ErrorIs(t, err, submission.ErrExportLimit)
	require.Nil(t, encoded)
	nested := map[string]any{"leaf": "value"}
	for range submission.MaxExportDepth + 1 {
		nested = map[string]any{"child": nested}
	}
	encoded, err = submission.EncodeExport(t.Context(), submission.ExportCSV, []submission.ExportRecord{exportRecord(model.JSON(nested))}, submission.ExportMeta{})
	require.ErrorIs(t, err, submission.ErrExportLimit)
	require.Nil(t, encoded)
}

func TestEmptyExportsAndAuditValidation(t *testing.T) {
	for _, format := range []submission.ExportFormat{submission.ExportJSON, submission.ExportCSV} {
		encoded, err := submission.EncodeExport(t.Context(), format, nil, submission.ExportMeta{ExportID: uuid.NewString(), PreparedAt: time.Now()})
		require.NoError(t, err)
		require.NotEmpty(t, encoded)
	}
	audit := submission.ExportAudit{ID: uuid.NewString(), OrganizationID: uuid.NewString(), FormID: uuid.NewString(), SubjectID: "subject-1",
		CredentialClass: "service_token", CredentialID: "credential-1", RequestID: "request-1", Format: submission.ExportJSON,
		RowCount: 0, ByteCount: 20, PreparedAt: time.Now()}
	require.NoError(t, audit.Validate())
	for _, mutate := range []func(*submission.ExportAudit){
		func(a *submission.ExportAudit) { a.ID = "bad" }, func(a *submission.ExportAudit) { a.SubjectID = "private value" },
		func(a *submission.ExportAudit) { a.CredentialClass = "other" }, func(a *submission.ExportAudit) { a.RowCount = -1 },
		func(a *submission.ExportAudit) { a.ByteCount = submission.MaxExportBytes + 1 }, func(a *submission.ExportAudit) { a.PreparedAt = time.Time{} },
	} {
		invalid := audit
		mutate(&invalid)
		require.ErrorIs(t, invalid.Validate(), submission.ErrExportAudit)
	}
}
