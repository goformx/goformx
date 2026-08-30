package submission

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/goformx/goforms/internal/domain/form/model"
)

const (
	MaxExportRows         = 1000
	MaxExportBytes        = 8 << 20
	MaxExportSourceBytes  = 8 << 20
	MaxExportColumns      = 256
	MaxExportDepth        = 32
	MaxExportRequestBytes = 4096
	MaxConcurrentExports  = 1
	ExportTimeout         = 10 * time.Second
)

var (
	ErrExportLimit   = errors.New("export exceeds a resource limit; narrow the filters")
	ErrExportRequest = errors.New("invalid export request")
	ErrExportAudit   = errors.New("invalid export audit record")
	auditIdentifier  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)

type ExportFormat string

const (
	ExportJSON ExportFormat = "json"
	ExportCSV  ExportFormat = "csv"
)

func (f ExportFormat) Valid() bool { return f == ExportJSON || f == ExportCSV }

// ExportFilters intentionally has no tenant, cursor, or caller-controlled budget.
type ExportFilters struct {
	ReceivedFrom   *time.Time
	ReceivedBefore *time.Time
	Status         model.SubmissionStatus
	SchemaVersion  int
}

func (f ExportFilters) Validate() error {
	return (ListOptions{Limit: 1, ReceivedFrom: f.ReceivedFrom, ReceivedBefore: f.ReceivedBefore,
		Status: f.Status, SchemaVersion: f.SchemaVersion}).Validate()
}

// ExportRecord carries only the root privacy policy from the exact schema join,
// not an entire repeated form definition. The independent join keys are checked
// again before projection; repositories must not substitute the current version.
type ExportRecord struct {
	Submission      *model.FormSubmission
	SchemaFormID    string
	AcceptedVersion int
	Policy          model.JSON
}

type Projection struct {
	ID            string                 `json:"id"`
	FormID        string                 `json:"formId"`
	SchemaVersion int                    `json:"schemaVersion"`
	RequestID     string                 `json:"requestId"`
	Status        model.SubmissionStatus `json:"status"`
	Data          model.JSON             `json:"data"`
	RedactedPaths []string               `json:"redactedPaths"`
	SubmittedAt   string                 `json:"submittedAt"`
}

func Project(row *model.FormSubmission, schemaFormID string, version int, policy model.JSON) (*Projection, error) {
	if row == nil || schemaFormID != row.FormID || version != row.SchemaVersion {
		return nil, ErrRedactionPolicy
	}
	data, paths, err := Redact(policy, row.Data)
	if err != nil {
		return nil, err
	}
	return &Projection{ID: row.ID, FormID: row.FormID, SchemaVersion: row.SchemaVersion, RequestID: row.RequestID,
		Status: row.Status, Data: data, RedactedPaths: paths, SubmittedAt: row.SubmittedAt.UTC().Format(time.RFC3339Nano)}, nil
}

// ExportAudit records preparation, not delivery. It contains no values, filters,
// credential material, schema, or body digest. It survives form/token deletion.
type ExportAudit struct {
	ID              string
	OrganizationID  string
	FormID          string
	SubjectID       string
	CredentialClass string
	CredentialID    string
	RequestID       string
	Format          ExportFormat
	RowCount        int
	ByteCount       int
	PreparedAt      time.Time
}

func (a ExportAudit) Validate() error {
	for _, id := range []string{a.ID, a.OrganizationID, a.FormID} {
		if _, err := uuid.Parse(id); err != nil {
			return ErrExportAudit
		}
	}
	for _, id := range []string{a.SubjectID, a.CredentialID, a.RequestID} {
		if !auditIdentifier.MatchString(id) {
			return ErrExportAudit
		}
	}
	if a.CredentialClass != "service_token" && a.CredentialClass != "first_party_assertion" {
		return ErrExportAudit
	}
	if !a.Format.Valid() || a.RowCount < 0 || a.RowCount > MaxExportRows || a.ByteCount < 1 || a.ByteCount > MaxExportBytes ||
		a.PreparedAt.IsZero() {
		return ErrExportAudit
	}
	return nil
}

type ExportMeta struct {
	ExportID   string    `json:"exportId"`
	PreparedAt time.Time `json:"preparedAt"`
	RowCount   int       `json:"rowCount"`
}

type exportBuffer struct{ bytes.Buffer }

func (b *exportBuffer) Write(p []byte) (int, error) {
	if len(p) > MaxExportBytes-b.Len() {
		return 0, ErrExportLimit
	}
	return b.Buffer.Write(p)
}

// EncodeExport projects before encoding. No partial buffer is returned on error.
// The source repository must independently enforce its input-byte/snapshot bound.
func EncodeExport(ctx context.Context, format ExportFormat, records []ExportRecord, meta ExportMeta) ([]byte, error) {
	if !format.Valid() {
		return nil, ErrExportRequest
	}
	if len(records) > MaxExportRows {
		return nil, ErrExportLimit
	}
	projections := make([]*Projection, 0, len(records))
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		projection, err := Project(record.Submission, record.SchemaFormID, record.AcceptedVersion, record.Policy)
		if err != nil {
			return nil, err
		}
		projections = append(projections, projection)
	}
	meta.RowCount = len(projections)
	var buffer exportBuffer
	var err error
	if format == ExportJSON {
		err = encodeJSONExport(ctx, &buffer, projections, meta)
	} else {
		err = encodeCSVExport(ctx, &buffer, projections)
	}
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func encodeJSONExport(ctx context.Context, buffer *exportBuffer, rows []*Projection, meta ExportMeta) error {
	if _, err := buffer.Write([]byte(`{"data":[`)); err != nil {
		return err
	}
	for index, row := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		if index > 0 {
			if _, err := buffer.Write([]byte(",")); err != nil {
				return err
			}
		}
		encoded, err := json.Marshal(row)
		if err != nil {
			return err
		}
		if _, err := buffer.Write(encoded); err != nil {
			return err
		}
	}
	encoded, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	if _, err := buffer.Write([]byte(`],"meta":`)); err != nil {
		return err
	}
	if _, err := buffer.Write(encoded); err != nil {
		return err
	}
	_, err = buffer.Write([]byte("}\n"))
	return err
}

func encodeCSVExport(ctx context.Context, buffer *exportBuffer, rows []*Projection) error {
	headers := []string{"id", "formId", "schemaVersion", "requestId", "status", "submittedAt", "redactedPaths"}
	columns := map[string]bool{}
	values := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		fields := map[string]string{}
		if err := flattenExportObject(map[string]any(row.Data), "", 0, fields); err != nil {
			return err
		}
		for path := range fields {
			columns[path] = true
			if len(columns)+len(headers) > MaxExportColumns {
				return ErrExportLimit
			}
		}
		values = append(values, fields)
	}
	paths := make([]string, 0, len(columns))
	for path := range columns {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		headers = append(headers, "data:"+path)
	}
	if err := writeCSVRecord(buffer, headers); err != nil {
		return err
	}
	for index, row := range rows {
		if err := ctx.Err(); err != nil {
			return err
		}
		policy, err := json.Marshal(row.RedactedPaths)
		if err != nil {
			return err
		}
		cells := []string{row.ID, row.FormID, strconv.Itoa(row.SchemaVersion), row.RequestID, string(row.Status), row.SubmittedAt, string(policy)}
		for _, path := range paths {
			cells = append(cells, values[index][path])
		}
		if err := writeCSVRecord(buffer, cells); err != nil {
			return err
		}
	}
	return nil
}

// Flatten objects by escaped JSON Pointer; arrays and empty objects retain JSON.
func flattenExportObject(object map[string]any, prefix string, depth int, fields map[string]string) error {
	if depth > MaxExportDepth {
		return ErrExportLimit
	}
	for key, value := range object {
		path := prefix + "/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
		if child, ok := value.(map[string]any); ok && len(child) > 0 {
			if err := flattenExportObject(child, path, depth+1, fields); err != nil {
				return err
			}
			continue
		}
		if text, ok := value.(string); ok {
			fields[path] = text
		} else {
			encoded, err := json.Marshal(value)
			if err != nil {
				return err
			}
			fields[path] = string(encoded)
		}
		if len(fields) > MaxExportColumns {
			return ErrExportLimit
		}
	}
	return nil
}

// Quote every cell, escape quotes, and prefix all cells as spreadsheet text,
// including headers and numbers. JSON, not CSV, is the machine round-trip format.
func writeCSVRecord(buffer *exportBuffer, cells []string) error {
	for index, cell := range cells {
		if index > 0 {
			if _, err := buffer.Write([]byte(",")); err != nil {
				return err
			}
		}
		if _, err := buffer.Write([]byte(`"'`)); err != nil {
			return err
		}
		if _, err := buffer.Write([]byte(strings.ReplaceAll(cell, `"`, `""`))); err != nil {
			return err
		}
		if _, err := buffer.Write([]byte(`"`)); err != nil {
			return err
		}
	}
	_, err := buffer.Write([]byte("\r\n"))
	return err
}
