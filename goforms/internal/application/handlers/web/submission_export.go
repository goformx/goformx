package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/goformx/goforms/internal/application/middleware/serviceauth"
	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/submission"
)

func decodeExportRequest(c echo.Context) (submission.ExportFormat, submission.ExportFilters, error) {
	bad := func() (submission.ExportFormat, submission.ExportFilters, error) {
		return "", submission.ExportFilters{}, submission.ErrExportRequest
	}
	if c.Request().URL.RawQuery != "" {
		return bad()
	}
	decoder := json.NewDecoder(http.MaxBytesReader(c.Response(), c.Request().Body, submission.MaxExportRequestBytes))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return bad()
	}
	values := url.Values{}
	seen := map[string]bool{}
	var format submission.ExportFormat
	for decoder.More() {
		keyToken, err := decoder.Token()
		key, ok := keyToken.(string)
		if err != nil || !ok || seen[key] {
			return bad()
		}
		seen[key] = true
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil || string(raw) == "null" {
			return bad()
		}
		var value string
		switch key {
		case "format", "receivedFrom", "receivedBefore", "status":
			if err := json.Unmarshal(raw, &value); err != nil || value == "" {
				return bad()
			}
		case "schemaVersion":
			value, err = exportSchemaVersion(raw)
			if err != nil {
				return bad()
			}
		default:
			return bad()
		}
		if key == "format" {
			format = submission.ExportFormat(value)
		} else {
			values.Set(key, value)
		}
	}
	if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
		return bad()
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) || !format.Valid() {
		return bad()
	}
	options, err := submissionOptionsFromParameters(values)
	if err != nil {
		return bad()
	}
	return format, submission.ExportFilters{ReceivedFrom: options.ReceivedFrom, ReceivedBefore: options.ReceivedBefore,
		Status: options.Status, SchemaVersion: options.SchemaVersion}, nil
}

func exportSchemaVersion(raw json.RawMessage) (string, error) {
	// Use the bounded lossless codec before rational parsing: large exponents
	// must not allocate arbitrary big integers. JSON Schema integers include
	// integral decimal/exponent spellings, but not rounded near-integers.
	var object model.JSON
	if err := json.Unmarshal([]byte(`{"value":`+string(raw)+`}`), &object); err != nil {
		return "", submission.ErrExportRequest
	}
	number, ok := object["value"].(json.Number)
	if !ok {
		return "", submission.ErrExportRequest
	}
	value, ok := new(big.Rat).SetString(number.String())
	if !ok || !value.IsInt() || !value.Num().IsInt64() {
		return "", submission.ErrExportRequest
	}
	version := value.Num().Int64()
	if version < 1 || version > submission.MaxSchemaVersion {
		return "", submission.ErrExportRequest
	}
	return strconv.FormatInt(version, 10), nil
}

func (h *V1APIHandler) exportSubmissions(c echo.Context) error {
	// A bounded artifact can still use several times its wire size while JSON
	// is decoded/projected. Admit one export per API instance, without a queue.
	if !h.exportActive.CompareAndSwap(false, true) {
		c.Response().Header().Set(echo.HeaderRetryAfter, "1")
		return h.writeError(c, http.StatusTooManyRequests, "export_busy", "Another export is being prepared; retry later.", nil)
	}
	defer h.exportActive.Store(false)
	ctx, cancel := context.WithTimeout(c.Request().Context(), submission.ExportTimeout)
	defer cancel()
	c.SetRequest(c.Request().WithContext(ctx))
	form, ok := h.ownedForm(c)
	if !ok {
		return nil
	}
	format, filters, err := decodeExportRequest(c)
	if err != nil {
		return h.writeError(c, http.StatusBadRequest, "invalid_request", "Export requires a valid format and supported body filters.", nil)
	}
	records, err := h.repository.ReadSubmissionExport(ctx, form.OrganizationID, form.ID, filters)
	if err != nil {
		if ctx.Err() != nil {
			return h.writeExportError(c, ctx.Err())
		}
		return h.writeExportError(c, err)
	}
	for _, record := range records {
		if record.Submission == nil || record.Submission.FormID != form.ID {
			return h.writeExportError(c, submission.ErrRedactionPolicy)
		}
	}
	meta := submission.ExportMeta{ExportID: uuid.NewString(), PreparedAt: time.Now().UTC(), RowCount: len(records)}
	encoded, err := submission.EncodeExport(ctx, format, records, meta)
	if err != nil {
		return h.writeExportError(c, err)
	}
	principal, _ := serviceauth.PrincipalFrom(c)
	audit := submission.ExportAudit{ID: meta.ExportID, OrganizationID: form.OrganizationID, FormID: form.ID,
		SubjectID: principal.SubjectID, CredentialClass: string(principal.CredentialClass), CredentialID: principal.CredentialID,
		RequestID: requestID(c), Format: format, RowCount: len(records), ByteCount: len(encoded), PreparedAt: meta.PreparedAt}
	if err := ctx.Err(); err != nil {
		return h.writeExportError(c, err)
	}
	if err := h.repository.SaveSubmissionExportAudit(ctx, audit); err != nil {
		if ctx.Err() != nil {
			return h.writeExportError(c, ctx.Err())
		}
		return h.writeError(c, http.StatusServiceUnavailable, "export_audit_unavailable", "Export could not be durably audited; no download was released.", nil)
	}
	if err := ctx.Err(); err != nil {
		return h.writeExportError(c, err)
	}
	c.Response().Header().Set("X-GoFormX-Export-ID", meta.ExportID)
	c.Response().Header().Set(echo.HeaderContentLength, strconv.Itoa(len(encoded)))
	c.Response().Header().Set("Content-Disposition", `attachment; filename="goformx-submissions-`+meta.ExportID+`.`+string(format)+`"`)
	contentType := "application/json"
	if format == submission.ExportCSV {
		contentType = "text/csv; charset=utf-8"
	}
	return c.Blob(http.StatusOK, contentType, encoded)
}

func (h *V1APIHandler) writeExportError(c echo.Context, err error) error {
	if errors.Is(err, submission.ErrExportLimit) {
		return h.writeError(c, http.StatusRequestEntityTooLarge, "export_limit_exceeded", "Export exceeds a resource limit; narrow the filters.", nil)
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return h.writeError(c, http.StatusGatewayTimeout, "export_timeout", "Export did not finish within its time budget.", nil)
	}
	return h.writeRepositoryError(c, err)
}
