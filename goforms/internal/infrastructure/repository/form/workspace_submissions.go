package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/submission"
	"github.com/goformx/goforms/internal/infrastructure/repository/common"
)

type workspaceSubmissionRecord struct {
	model.FormSubmission `gorm:"embedded"`
	SiteID               *string `gorm:"column:site_id"`
	FormName             string  `gorm:"column:form_name"`
	FormTitle            string  `gorm:"column:form_title"`
}

// ListWorkspaceSubmissionsPage reads existing rows through their authoritative
// organization-owned forms. Selector checks and page selection use one snapshot.
func (s *Store) ListWorkspaceSubmissionsPage(ctx context.Context, organizationID string, options submission.WorkspaceListOptions) ([]submission.WorkspaceRow, bool, error) {
	if err := options.Validate(); err != nil {
		return nil, false, err
	}
	var result []submission.WorkspaceRow
	var hasMore bool
	err := s.db.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if options.SiteID != "" {
			var count int64
			if err := tx.Table("sites").Where("organization_id = ? AND uuid = ?", organizationID, options.SiteID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return common.NewNotFoundError("list", "submissions", "")
			}
		}
		if options.FormID != "" {
			query := tx.Table("forms").Where("organization_id = ? AND uuid = ? AND deleted_at IS NULL", organizationID, options.FormID)
			if options.SiteID != "" {
				query = query.Where("site_id = ?", options.SiteID)
			}
			var count int64
			if err := query.Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return common.NewNotFoundError("list", "submissions", "")
			}
		}
		query := tx.Table("form_submissions").
			Select("form_submissions.*, forms.site_id AS site_id, forms.name AS form_name, forms.title AS form_title").
			Joins("JOIN forms ON forms.uuid = form_submissions.form_id").
			Where("forms.organization_id = ? AND forms.deleted_at IS NULL", organizationID)
		if options.SiteID != "" {
			query = query.Where("forms.site_id = ?", options.SiteID)
		}
		if options.FormID != "" {
			query = query.Where("forms.uuid = ?", options.FormID)
		}
		if options.ReceivedFrom != nil {
			query = query.Where("form_submissions.submitted_at >= ?", options.ReceivedFrom.UTC())
		}
		if options.ReceivedBefore != nil {
			query = query.Where("form_submissions.submitted_at < ?", options.ReceivedBefore.UTC())
		}
		if options.Status != "" {
			query = query.Where("form_submissions.status = ?", options.Status)
		}
		if options.SchemaVersion != 0 {
			query = query.Where("form_submissions.schema_version = ?", options.SchemaVersion)
		}
		if !options.Before.IsZero() {
			query = query.Where("(form_submissions.submitted_at < ? OR (form_submissions.submitted_at = ? AND form_submissions.uuid < ?))", options.Before, options.Before, options.BeforeID)
		}
		var records []workspaceSubmissionRecord
		if err := query.Order("form_submissions.submitted_at DESC, form_submissions.uuid DESC").Limit(options.Limit + 1).Scan(&records).Error; err != nil {
			return err
		}
		hasMore = len(records) > options.Limit
		if hasMore {
			records = records[:options.Limit]
		}
		result = make([]submission.WorkspaceRow, 0, len(records))
		for i := range records {
			row := &records[i]
			result = append(result, submission.WorkspaceRow{Submission: &row.FormSubmission, SiteID: row.SiteID, FormName: row.FormName, FormTitle: row.FormTitle})
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return nil, false, err
		}
		return nil, false, fmt.Errorf("list workspace submissions: %w", common.NewDatabaseError("list", "submissions", "", err))
	}
	return result, hasMore, nil
}
