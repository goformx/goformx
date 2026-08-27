// Package repository provides the form repository implementation
package repository

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/goformx/goforms/internal/domain/form"
	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/infrastructure/database"
	"github.com/goformx/goforms/internal/infrastructure/logging"
	"github.com/goformx/goforms/internal/infrastructure/repository/common"
)

// Store implements form.Repository interface
type Store struct {
	db     database.DB
	logger logging.Logger
}

type schemaRecord struct {
	ID          string     `gorm:"column:uuid;primaryKey"`
	FormID      string     `gorm:"column:form_id"`
	Schema      model.JSON `gorm:"type:jsonb"`
	Version     int
	State       string
	CreatedAt   time.Time
	PublishedAt *time.Time
}

func (schemaRecord) TableName() string { return "form_schemas" }

// NewStore creates a new form store
func NewStore(db database.DB, logger logging.Logger) form.Repository {
	return &Store{
		db:     db,
		logger: logger,
	}
}

// CreateForm creates a new form
func (s *Store) CreateForm(ctx context.Context, formModel *model.Form) error {
	err := s.db.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(formModel).Error; err != nil {
			return err
		}
		record := schemaRecord{ID: uuid.NewString(), FormID: formModel.ID, Schema: formModel.Schema,
			Version: formModel.CurrentSchemaVersion, State: string(model.SchemaVersionDraft), CreatedAt: time.Now().UTC()}
		return tx.Create(&record).Error
	})
	if err != nil {
		s.logger.Error("failed to create form",
			"form_id", formModel.ID,
			"error", err,
		)

		return fmt.Errorf("create form: %w", common.NewDatabaseError("create", "form", formModel.ID, err))
	}

	return nil
}

// GetFormByID retrieves a form by ID
func (s *Store) GetFormByID(ctx context.Context, id string) (*model.Form, error) {
	// Normalize the UUID by trimming spaces and converting to lowercase
	normalizedID := strings.TrimSpace(strings.ToLower(id))

	// Validate UUID format
	if _, err := uuid.Parse(normalizedID); err != nil {
		s.logger.Warn("invalid form ID format received",
			"id_length", len(id),
			"error_type", "invalid_uuid_format")

		invalidErr := common.NewInvalidInputError("get", "form", id, err)

		return nil, fmt.Errorf("get form by ID: %w", invalidErr)
	}

	var formModel model.Form
	if err := s.db.GetDB().WithContext(ctx).Where("uuid = ?", normalizedID).First(&formModel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Debug("form not found in database",
				"id_length", len(normalizedID),
				"error_type", "not_found")

			return nil, fmt.Errorf("get form by ID: %w", common.NewNotFoundError("get", "form", normalizedID))
		}

		s.logger.Error("database error while getting form",
			"id_length", len(normalizedID),
			"error", err,
			"error_type", "database_error")

		dbErr := common.NewDatabaseError("get", "form", normalizedID, err)

		return nil, fmt.Errorf("get form by ID: %w", dbErr)
	}

	s.logger.Debug("form retrieved successfully",
		"id_length", len(normalizedID),
		"form_title", formModel.Title)

	if err := s.loadCurrentSchema(ctx, &formModel); err != nil {
		return nil, err
	}
	return &formModel, nil
}

// GetFormByPublicKey resolves a browser-safe identifier without exposing internal IDs.
func (s *Store) GetFormByPublicKey(ctx context.Context, publicKey string) (*model.Form, error) {
	var formModel model.Form
	if err := s.db.GetDB().WithContext(ctx).Where("public_key = ?", publicKey).First(&formModel).Error; err != nil {
		return nil, fmt.Errorf("get form by public key: %w", err)
	}
	if err := s.loadCurrentSchema(ctx, &formModel); err != nil {
		return nil, err
	}
	return &formModel, nil
}

func (s *Store) loadCurrentSchema(ctx context.Context, formModel *model.Form) error {
	var record schemaRecord
	err := s.db.GetDB().WithContext(ctx).Where("form_id = ? AND version = ?", formModel.ID, formModel.CurrentSchemaVersion).
		First(&record).Error
	if err != nil {
		return fmt.Errorf("load current schema: %w", err)
	}
	formModel.Schema = record.Schema
	return nil
}

// ListForms retrieves all forms for a user
func (s *Store) ListForms(ctx context.Context, userID string) ([]*model.Form, error) {
	var forms []*model.Form
	if err := s.db.GetDB().WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&forms).Error; err != nil {
		s.logger.Error("failed to list forms",
			"user_id", userID,
			"error", err,
		)

		return nil, fmt.Errorf("list forms: %w", common.NewDatabaseError("list", "form", "", err))
	}

	return forms, nil
}

// UpdateForm updates a form
func (s *Store) UpdateForm(ctx context.Context, formModel *model.Form) error {
	return s.db.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current schemaRecord
		if err := tx.Where("form_id = ? AND version = ?", formModel.ID, formModel.CurrentSchemaVersion).First(&current).Error; err != nil {
			return fmt.Errorf("load schema for update: %w", err)
		}
		if formModel.Schema != nil && !reflect.DeepEqual(map[string]any(formModel.Schema), map[string]any(current.Schema)) {
			formModel.CurrentSchemaVersion++
			next := schemaRecord{ID: uuid.NewString(), FormID: formModel.ID, Schema: formModel.Schema,
				Version: formModel.CurrentSchemaVersion, State: string(model.SchemaVersionDraft), CreatedAt: time.Now().UTC()}
			if err := tx.Create(&next).Error; err != nil {
				return fmt.Errorf("create schema version: %w", err)
			}
		}
		if formModel.Status == string(model.LifecyclePublished) {
			now := time.Now().UTC()
			if err := tx.Model(&schemaRecord{}).Where("form_id = ? AND version = ?", formModel.ID, formModel.CurrentSchemaVersion).
				Updates(map[string]any{"state": string(model.SchemaVersionPublished), "published_at": now}).Error; err != nil {
				return fmt.Errorf("publish schema version: %w", err)
			}
		}
		result := tx.Model(&model.Form{}).Where("uuid = ?", formModel.ID).Updates(formModel)
		if result.Error != nil {
			return fmt.Errorf("update form: %w", common.NewDatabaseError("update", "form", formModel.ID, result.Error))
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("update form: %w", common.NewNotFoundError("update", "form", formModel.ID))
		}
		return nil
	})
}

// DeleteForm deletes a form
func (s *Store) DeleteForm(ctx context.Context, id string) error {
	// Normalize the UUID by trimming spaces and converting to lowercase
	normalizedID := strings.TrimSpace(strings.ToLower(id))

	// Validate UUID format
	if _, err := uuid.Parse(normalizedID); err != nil {
		s.logger.Warn("invalid form ID format received for deletion",
			"id_length", len(id),
			"error_type", "invalid_uuid_format")

		invalidErr := common.NewInvalidInputError("delete", "form", id, err)

		return fmt.Errorf("delete form: %w", invalidErr)
	}

	result := s.db.GetDB().WithContext(ctx).Where("uuid = ?", normalizedID).Delete(&model.Form{})
	if result.Error != nil {
		s.logger.Error("failed to delete form",
			"id_length", len(normalizedID),
			"error", result.Error,
			"error_type", "database_error")

		return fmt.Errorf("delete form: %w", common.NewDatabaseError("delete", "form", normalizedID, result.Error))
	}

	if result.RowsAffected == 0 {
		s.logger.Debug("form not found for deletion",
			"id_length", len(normalizedID),
			"error_type", "not_found")

		return fmt.Errorf("delete form: %w", common.NewNotFoundError("delete", "form", normalizedID))
	}

	s.logger.Debug("form deleted successfully",
		"id_length", len(normalizedID))

	return nil
}

// GetFormsByStatus returns forms by their active status
func (s *Store) GetFormsByStatus(ctx context.Context, status string) ([]*model.Form, error) {
	var forms []*model.Form
	if err := s.db.GetDB().WithContext(ctx).Where("status = ?", status).Find(&forms).Error; err != nil {
		return nil, fmt.Errorf("failed to get forms by status: %w", err)
	}

	return forms, nil
}

// CreateSchemaVersion appends a draft without mutating any existing snapshot.
func (s *Store) CreateSchemaVersion(
	ctx context.Context,
	formID string,
	schema model.JSON,
) (*model.SchemaVersion, error) {
	var created *model.SchemaVersion
	err := s.db.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var formModel model.Form
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("uuid = ?", formID).First(&formModel).Error; err != nil {
			return fmt.Errorf("lock form: %w", err)
		}
		var latest int
		if err := tx.Model(&schemaRecord{}).Where("form_id = ?", formID).
			Select("COALESCE(MAX(version), 0)").Scan(&latest).Error; err != nil {
			return fmt.Errorf("find latest schema version: %w", err)
		}
		version, err := model.NewSchemaVersion(formID, latest+1, schema, schemaDefinitionValidator{})
		if err != nil {
			return err
		}
		record := schemaRecord{ID: uuid.NewString(), FormID: formID, Schema: version.Schema(),
			Version: version.Version(), State: string(version.State()), CreatedAt: version.CreatedAt()}
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("create schema version: %w", err)
		}
		created = version
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create schema version: %w", err)
	}
	return created, nil
}

// schemaDefinitionValidator only checks persistence-bound invariants. Canonical compilation occurs at the application boundary.
type schemaDefinitionValidator struct{}

func (schemaDefinitionValidator) ValidateDefinition(schema model.JSON) error {
	if schema == nil {
		return errors.New("schema is required")
	}
	return nil
}

func restoreSchema(record schemaRecord) (*model.SchemaVersion, error) {
	return model.RestoreSchemaVersion(record.FormID, record.Version, record.Schema,
		model.SchemaVersionState(record.State), record.CreatedAt, record.PublishedAt)
}

func (s *Store) GetSchemaVersion(ctx context.Context, formID string, version int) (*model.SchemaVersion, error) {
	var record schemaRecord
	if err := s.db.GetDB().WithContext(ctx).Where("form_id = ? AND version = ?", formID, version).
		First(&record).Error; err != nil {
		return nil, fmt.Errorf("get schema version: %w", err)
	}
	return restoreSchema(record)
}

// GetPublishedSchemaVersion resolves only an active published form and an explicitly published snapshot.
func (s *Store) GetPublishedSchemaVersion(
	ctx context.Context,
	publicKey string,
	version int,
) (*model.Form, *model.SchemaVersion, error) {
	var formModel model.Form
	if err := s.db.GetDB().WithContext(ctx).Where(
		"public_key = ? AND status = ? AND active = true", publicKey, string(model.LifecyclePublished),
	).First(&formModel).Error; err != nil {
		return nil, nil, fmt.Errorf("get published form: %w", err)
	}
	if version == 0 {
		version = formModel.CurrentSchemaVersion
	}
	var record schemaRecord
	if err := s.db.GetDB().WithContext(ctx).Where(
		"form_id = ? AND version = ? AND state = ?", formModel.ID, version, string(model.SchemaVersionPublished),
	).First(&record).Error; err != nil {
		return nil, nil, fmt.Errorf("get published schema version: %w", err)
	}
	schemaVersion, err := restoreSchema(record)
	if err != nil {
		return nil, nil, err
	}
	formModel.Schema = schemaVersion.Schema()
	return &formModel, schemaVersion, nil
}

func (s *Store) PublishSchemaVersion(
	ctx context.Context,
	formID string,
	version int,
) (*model.SchemaVersion, error) {
	var published *model.SchemaVersion
	err := s.db.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var formModel model.Form
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("uuid = ?", formID).First(&formModel).Error; err != nil {
			return fmt.Errorf("lock form: %w", err)
		}
		if formModel.Status == string(model.LifecyclePublished) && version < formModel.CurrentSchemaVersion {
			return errors.New("published schema version cannot move backwards")
		}
		var record schemaRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"form_id = ? AND version = ?", formID, version,
		).First(&record).Error; err != nil {
			return fmt.Errorf("get schema version: %w", err)
		}
		existing, err := restoreSchema(record)
		if err != nil {
			return err
		}
		if existing.State() == model.SchemaVersionPublished {
			published = existing
		} else {
			published, err = existing.Publish(time.Now().UTC())
			if err != nil {
				return err
			}
			if err := tx.Model(&schemaRecord{}).Where("form_id = ? AND version = ?", formID, version).
				Updates(map[string]any{"state": string(model.SchemaVersionPublished), "published_at": published.PublishedAt()}).Error; err != nil {
				return fmt.Errorf("publish schema record: %w", err)
			}
		}
		if err := tx.Model(&model.Form{}).Where("uuid = ?", formID).Updates(map[string]any{
			"status": string(model.LifecyclePublished), "active": true, "current_schema_version": version,
		}).Error; err != nil {
			return fmt.Errorf("publish form: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("publish schema version: %w", err)
	}
	return published, nil
}

// CreateSubmission creates a new form submission
func (s *Store) CreateSubmission(ctx context.Context, submission *model.FormSubmission) error {
	if err := s.db.GetDB().WithContext(ctx).Create(submission).Error; err != nil {
		s.logger.Error("failed to create form submission",
			"submission_id", submission.ID,
			"form_id", submission.FormID,
			"error", err,
		)

		return fmt.Errorf("create submission: %w", common.NewDatabaseError("create", "form_submission", submission.ID, err))
	}

	return nil
}

// CreateSubmissionIdempotent atomically inserts or returns the original submission for a replayed key.
func (s *Store) CreateSubmissionIdempotent(
	ctx context.Context,
	submission *model.FormSubmission,
) (*model.FormSubmission, bool, error) {
	if submission == nil || submission.IdempotencyKey == "" {
		return nil, false, errors.New("submission and idempotency key are required")
	}
	result := s.db.GetDB().WithContext(ctx).Clauses(clause.OnConflict{
		Columns:     []clause.Column{{Name: "form_id"}, {Name: "idempotency_key"}},
		TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "idempotency_key IS NOT NULL"}}},
		DoNothing:   true,
	}).Create(submission)
	if result.Error != nil {
		return nil, false, fmt.Errorf("create idempotent submission: %w", result.Error)
	}
	if result.RowsAffected == 1 {
		return submission, false, nil
	}
	var existing model.FormSubmission
	if err := s.db.GetDB().WithContext(ctx).Where(
		"form_id = ? AND idempotency_key = ?", submission.FormID, submission.IdempotencyKey,
	).First(&existing).Error; err != nil {
		return nil, false, fmt.Errorf("load idempotent submission: %w", err)
	}
	return &existing, true, nil
}

// GetSubmissionByID retrieves a form submission by ID
func (s *Store) GetSubmissionByID(ctx context.Context, submissionID string) (*model.FormSubmission, error) {
	var submission model.FormSubmission
	if err := s.db.GetDB().WithContext(ctx).Where("uuid = ?", submissionID).First(&submission).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("get submission by ID: %w",
				common.NewNotFoundError("get", "form_submission", submissionID))
		}

		return nil, fmt.Errorf("get submission by ID: %w",
			common.NewDatabaseError("get", "form_submission", submissionID, err))
	}

	return &submission, nil
}

// ListSubmissions retrieves all submissions for a form
func (s *Store) ListSubmissions(ctx context.Context, formID string) ([]*model.FormSubmission, error) {
	var submissions []*model.FormSubmission
	if err := s.db.GetDB().WithContext(ctx).Where("form_id = ?", formID).Find(&submissions).Error; err != nil {
		s.logger.Error("failed to list form submissions",
			"form_id", formID,
			"error", err,
		)

		return nil, fmt.Errorf("list form submissions: %w", common.NewDatabaseError("list", "form_submission", formID, err))
	}

	return submissions, nil
}

// UpdateSubmission updates a form submission
func (s *Store) UpdateSubmission(ctx context.Context, submission *model.FormSubmission) error {
	result := s.db.GetDB().WithContext(ctx).
		Model(&model.FormSubmission{}).
		Where("uuid = ?", submission.ID).
		Updates(submission)
	if result.Error != nil {
		s.logger.Error("failed to update form submission",
			"submission_id", submission.ID,
			"error", result.Error,
		)

		return fmt.Errorf("update submission: %w",
			common.NewDatabaseError("update", "form_submission", submission.ID, result.Error))
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("update submission: %w", common.NewNotFoundError("update", "form_submission", submission.ID))
	}

	return nil
}

// DeleteSubmission deletes a form submission
func (s *Store) DeleteSubmission(ctx context.Context, submissionID string) error {
	result := s.db.GetDB().WithContext(ctx).Where("uuid = ?", submissionID).Delete(&model.FormSubmission{})
	if result.Error != nil {
		s.logger.Error("failed to delete form submission",
			"submission_id", submissionID,
			"error", result.Error,
		)

		return fmt.Errorf("delete submission: %w",
			common.NewDatabaseError("delete", "form_submission", submissionID, result.Error))
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("delete submission: %w", common.NewNotFoundError("delete", "form_submission", submissionID))
	}

	return nil
}

// GetByFormID retrieves all submissions for a form
func (s *Store) GetByFormID(ctx context.Context, formID string) ([]*model.FormSubmission, error) {
	return s.ListSubmissions(ctx, formID)
}

// GetByFormIDPaginated retrieves paginated submissions for a form
func (s *Store) GetByFormIDPaginated(
	ctx context.Context,
	formID string,
	params common.PaginationParams,
) (*common.PaginationResult, error) {
	var total int64

	query := s.db.GetDB().WithContext(ctx).Model(&model.FormSubmission{}).Where("form_id = ?", formID)
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count submissions: %w", err)
	}

	var submissions []*model.FormSubmission
	if err := query.
		Offset(params.GetOffset()).
		Limit(params.GetLimit()).
		Find(&submissions).Error; err != nil {
		return nil, fmt.Errorf("failed to get submissions: %w", err)
	}

	return &common.PaginationResult{
		Items:      submissions,
		TotalItems: int(total),
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: (int(total) + params.PageSize - 1) / params.PageSize,
	}, nil
}

// GetByFormAndUser retrieves a submission by form ID and user ID
func (s *Store) GetByFormAndUser(
	ctx context.Context,
	formID string,
	userID string,
) (*model.FormSubmission, error) {
	var submission model.FormSubmission

	query := s.db.GetDB().WithContext(ctx).
		Where("form_id = ? AND user_id = ?", formID, userID).
		First(&submission)
	if err := query.Error; err != nil {
		return nil, fmt.Errorf("failed to get submission: %w", err)
	}

	return &submission, nil
}

// GetSubmissionsByStatus retrieves submissions by status
func (s *Store) GetSubmissionsByStatus(
	ctx context.Context,
	status model.SubmissionStatus,
) ([]*model.FormSubmission, error) {
	var submissions []*model.FormSubmission
	if err := s.db.GetDB().WithContext(ctx).
		Where("status = ?", status).
		Find(&submissions).Error; err != nil {
		return nil, fmt.Errorf("failed to get submissions: %w", err)
	}

	return submissions, nil
}

// CountFormsByUser returns the number of forms owned by a user.
func (s *Store) CountFormsByUser(ctx context.Context, userID string) (int, error) {
	var count int64
	if err := s.db.GetDB().WithContext(ctx).
		Model(&model.Form{}).
		Where("user_id = ?", userID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count forms by user: %w", err)
	}

	return int(count), nil
}

// CountSubmissionsByUserMonth returns the number of submissions for a user in a given month.
func (s *Store) CountSubmissionsByUserMonth(
	ctx context.Context,
	userID string,
	year int,
	month int,
) (int, error) {
	startOfMonth := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	endOfMonth := startOfMonth.AddDate(0, 1, 0)

	var count int64
	if err := s.db.GetDB().WithContext(ctx).
		Model(&model.FormSubmission{}).
		Joins("JOIN forms ON forms.uuid = form_submissions.form_id AND forms.deleted_at IS NULL").
		Where(
			"forms.user_id = ? AND form_submissions.created_at >= ? AND form_submissions.created_at < ?",
			userID, startOfMonth, endOfMonth,
		).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count submissions by user month: %w", err)
	}

	return int(count), nil
}
