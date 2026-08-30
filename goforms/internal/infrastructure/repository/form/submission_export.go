package repository

import (
	"context"
	"fmt"

	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/submission"
	"github.com/goformx/goforms/internal/infrastructure/repository/common"
)

// ReadSubmissionExport uses one statement snapshot and returns no payload bytes
// from PostgreSQL when the candidate row/source-byte budget is exceeded. The
// final sentinel row detects overflow; it is never silently truncated.
func (s *Store) ReadSubmissionExport(ctx context.Context, organizationID, formID string, filters submission.ExportFilters) ([]submission.ExportRecord, error) {
	if err := filters.Validate(); err != nil {
		return nil, err
	}
	query := s.db.GetDB().WithContext(ctx).Table("form_submissions AS s").
		Select(`s.uuid, s.form_id, s.schema_version, s.request_id, s.status, s.submitted_at, s.data,
			COALESCE(v.form_id, '') AS schema_form_id, COALESCE(v.version, 0) AS accepted_version,
			CASE WHEN v.schema IS NULL OR jsonb_typeof(v.schema) <> 'object' THEN NULL
			     WHEN jsonb_exists(v.schema, ?) THEN jsonb_build_object(CAST(? AS text), v.schema -> CAST(? AS text))
			     ELSE '{}'::jsonb END AS policy`, submission.SensitiveAnnotation, submission.SensitiveAnnotation, submission.SensitiveAnnotation).
		Joins("JOIN forms f ON f.uuid = s.form_id").
		Joins("LEFT JOIN form_schemas v ON v.form_id = s.form_id AND v.version = s.schema_version").
		Where("f.deleted_at IS NULL AND f.organization_id = ? AND s.form_id = ?", organizationID, formID)
	if filters.ReceivedFrom != nil {
		query = query.Where("s.submitted_at >= ?", filters.ReceivedFrom.UTC())
	}
	if filters.ReceivedBefore != nil {
		query = query.Where("s.submitted_at < ?", filters.ReceivedBefore.UTC())
	}
	if filters.Status != "" {
		query = query.Where("s.status = ?", filters.Status)
	}
	if filters.SchemaVersion != 0 {
		query = query.Where("s.schema_version = ?", filters.SchemaVersion)
	}
	query = query.Order("s.submitted_at DESC, s.uuid DESC").Limit(submission.MaxExportRows + 1)
	// NULL policy/schema is preserved for fail-closed domain validation. Use
	// octet_length on text so JSONB storage compression cannot defeat the budget.
	measured := s.db.GetDB().Table("(?) AS candidates", query).Select(`candidates.*,
		count(*) OVER () AS row_count,
		sum(octet_length(data::text)::bigint + COALESCE(octet_length(policy::text), 0) +
		    octet_length(request_id) + octet_length(status) + 256) OVER () AS source_bytes`)
	rows, err := s.db.GetDB().WithContext(ctx).Table("(?) AS measured", measured).Select(`uuid, form_id, schema_version,
		CASE WHEN row_count <= ? AND source_bytes <= ? THEN request_id ELSE '' END AS request_id,
		status, submitted_at,
		schema_form_id, accepted_version, row_count, source_bytes,
		CASE WHEN row_count <= ? AND source_bytes <= ? THEN data ELSE NULL END AS data,
		CASE WHEN row_count <= ? AND source_bytes <= ? THEN policy ELSE NULL END AS policy`,
		submission.MaxExportRows, submission.MaxExportSourceBytes,
		submission.MaxExportRows, submission.MaxExportSourceBytes, submission.MaxExportRows, submission.MaxExportSourceBytes).
		Order("submitted_at DESC, uuid DESC").Rows()
	if err != nil {
		return nil, fmt.Errorf("read submission export: %w", err)
	}
	defer rows.Close()
	result := make([]submission.ExportRecord, 0)
	for rows.Next() {
		var row model.FormSubmission
		var policy model.JSON
		var schemaFormID string
		var acceptedVersion int
		var rowCount, sourceBytes int64
		if err := rows.Scan(&row.ID, &row.FormID, &row.SchemaVersion, &row.RequestID, &row.Status, &row.SubmittedAt,
			&schemaFormID, &acceptedVersion, &rowCount, &sourceBytes, &row.Data, &policy); err != nil {
			return nil, fmt.Errorf("scan submission export: %w", err)
		}
		if rowCount > submission.MaxExportRows || sourceBytes > submission.MaxExportSourceBytes {
			return nil, submission.ErrExportLimit
		}
		result = append(result, submission.ExportRecord{Submission: &row, SchemaFormID: schemaFormID, AcceptedVersion: acceptedVersion, Policy: policy})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read submission export rows: %w", err)
	}
	return result, nil
}

func (s *Store) SaveSubmissionExportAudit(ctx context.Context, audit submission.ExportAudit) error {
	if err := audit.Validate(); err != nil {
		return err
	}
	// Recheck ownership/deletion at the durable release boundary, without storing
	// the payload, caller selectors, or credentials. No foreign-key cascade can
	// remove the audit when a form or service token is subsequently deleted.
	result := s.db.GetDB().WithContext(ctx).Exec(`INSERT INTO submission_export_audit
		(export_id, organization_id, form_id, subject_id, credential_class, credential_id, request_id, format, row_count, byte_count, prepared_at)
		SELECT ?, f.organization_id, f.uuid, ?, ?, ?, ?, ?, ?, ?, ? FROM forms f
		WHERE f.uuid = ? AND f.organization_id = ? AND f.deleted_at IS NULL`,
		audit.ID, audit.SubjectID, audit.CredentialClass, audit.CredentialID, audit.RequestID, audit.Format,
		audit.RowCount, audit.ByteCount, audit.PreparedAt.UTC(), audit.FormID, audit.OrganizationID)
	if result.Error != nil {
		return fmt.Errorf("save submission export audit: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return common.NewNotFoundError("export", "form", audit.FormID)
	}
	return nil
}
