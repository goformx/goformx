// Package repository provides the form repository implementation
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/goformx/goforms/internal/domain/form"
	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/submission"
	domainwebhook "github.com/goformx/goforms/internal/domain/webhook"
	"github.com/goformx/goforms/internal/infrastructure/database"
	"github.com/goformx/goforms/internal/infrastructure/logging"
	"github.com/goformx/goforms/internal/infrastructure/repository/common"
)

// Store persists the supported schema-first form lifecycle.
type Store struct {
	db                   database.DB
	logger               logging.Logger
	dailySubmissionLimit int
	webhookCipher        *domainwebhook.Cipher
	now                  func() time.Time
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

// NewStore creates a schema-first form store.
func NewStore(db database.DB, logger logging.Logger) *Store {
	return NewStoreWithDailySubmissionLimit(db, logger, form.DefaultSubmissionsPerDay)
}

// NewStoreWithDailySubmissionLimit creates a store with a transactionally enforced rolling quota.
func NewStoreWithDailySubmissionLimit(db database.DB, logger logging.Logger, limit int) *Store {
	return NewStoreWithOptions(db, logger, StoreOptions{DailySubmissionLimit: limit})
}

type StoreOptions struct {
	DailySubmissionLimit int
	WebhookCipher        *domainwebhook.Cipher
}

func NewStoreWithOptions(db database.DB, logger logging.Logger, options StoreOptions) *Store {
	limit := options.DailySubmissionLimit
	if limit <= 0 {
		limit = form.DefaultSubmissionsPerDay
	}
	return &Store{db: db, logger: logger, dailySubmissionLimit: limit,
		webhookCipher: options.WebhookCipher, now: time.Now}
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
		)

		return fmt.Errorf("create form: %w", common.NewDatabaseError("create", "form", formModel.ID, err))
	}

	return nil
}

// GetFormByID retrieves a form only inside the authenticated organization boundary.
func (s *Store) GetFormByID(ctx context.Context, organizationID, id string) (*model.Form, error) {
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
	if err := s.db.GetDB().WithContext(ctx).Where(
		"organization_id = ? AND uuid = ?", organizationID, normalizedID,
	).First(&formModel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Debug("form not found in database",
				"id_length", len(normalizedID),
				"error_type", "not_found")

			return nil, fmt.Errorf("get form by ID: %w", common.NewNotFoundError("get", "form", normalizedID))
		}

		s.logger.Error("database error while getting form",
			"id_length", len(normalizedID),
			"error_type", "database_error")

		dbErr := common.NewDatabaseError("get", "form", normalizedID, err)

		return nil, fmt.Errorf("get form by ID: %w", dbErr)
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

// ListForms returns one bounded, filtered organization page and the matching total.
func (s *Store) ListForms(
	ctx context.Context,
	organizationID string,
	options model.FormListOptions,
) ([]*model.Form, int64, error) {
	if err := options.Validate(); err != nil {
		return nil, 0, err
	}
	query := s.db.GetDB().WithContext(ctx).Model(&model.Form{}).Where("organization_id = ?", organizationID)
	if options.Status != "" {
		query = query.Where("status = ?", options.Status)
	}
	if options.Query != "" {
		pattern := "%" + escapeLike(options.Query) + "%"
		query = query.Where("(name ILIKE ? ESCAPE '\\' OR title ILIKE ? ESCAPE '\\')", pattern, pattern)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count forms: %w", common.NewDatabaseError("count", "form", "", err))
	}
	order, ok := formSortOrder[options.Sort]
	if !ok {
		return nil, 0, errors.New("form sort is invalid")
	}
	var forms []*model.Form
	if err := query.Order(order).Offset(options.Offset).Limit(options.Limit).
		Find(&forms).Error; err != nil {
		s.logger.Error("failed to list forms",
			"organization_id", organizationID,
		)

		return nil, 0, fmt.Errorf("list forms: %w", common.NewDatabaseError("list", "form", "", err))
	}

	return forms, total, nil
}

var formSortOrder = map[model.FormSort]string{
	model.FormSortCreatedDesc: "created_at DESC, uuid DESC",
	model.FormSortCreatedAsc:  "created_at ASC, uuid ASC",
	model.FormSortUpdatedDesc: "updated_at DESC, uuid DESC",
	model.FormSortUpdatedAsc:  "updated_at ASC, uuid ASC",
	model.FormSortNameDesc:    "name DESC, uuid DESC",
	model.FormSortNameAsc:     "name ASC, uuid ASC",
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

// UpdateForm updates a form
func (s *Store) UpdateForm(ctx context.Context, formModel *model.Form, expectedUpdatedAt time.Time) error {
	return s.db.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current schemaRecord
		if err := tx.Where("form_id = ? AND version = ?", formModel.ID, formModel.CurrentSchemaVersion).First(&current).Error; err != nil {
			return fmt.Errorf("load schema for update: %w", err)
		}
		if formModel.Schema != nil && !model.EqualJSON(formModel.Schema, current.Schema) {
			formModel.CurrentSchemaVersion++
			next := schemaRecord{ID: uuid.NewString(), FormID: formModel.ID, Schema: formModel.Schema,
				Version: formModel.CurrentSchemaVersion, State: string(model.SchemaVersionDraft), CreatedAt: time.Now().UTC()}
			if err := tx.Create(&next).Error; err != nil {
				return fmt.Errorf("create schema version: %w", err)
			}
		}
		if formModel.Status == model.LifecyclePublished {
			now := time.Now().UTC()
			if err := tx.Model(&schemaRecord{}).Where("form_id = ? AND version = ?", formModel.ID, formModel.CurrentSchemaVersion).
				Updates(map[string]any{"state": string(model.SchemaVersionPublished), "published_at": now}).Error; err != nil {
				return fmt.Errorf("publish schema version: %w", err)
			}
		}
		result := tx.Model(&model.Form{}).Where(
			"organization_id = ? AND uuid = ? AND updated_at = ?", formModel.OrganizationID, formModel.ID, expectedUpdatedAt,
		).Updates(formModel)
		if result.Error != nil {
			return fmt.Errorf("update form: %w", common.NewDatabaseError("update", "form", formModel.ID, result.Error))
		}
		if result.RowsAffected == 0 {
			return model.ErrPreconditionFailed
		}
		return nil
	})
}

// CreateSchemaVersion appends a draft without mutating any existing snapshot.
func (s *Store) CreateSchemaVersion(
	ctx context.Context,
	organizationID string,
	formID string,
	schema model.JSON,
) (*model.SchemaVersion, error) {
	var created *model.SchemaVersion
	err := s.db.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var formModel model.Form
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"organization_id = ? AND uuid = ?", organizationID, formID,
		).First(&formModel).Error; err != nil {
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

func (s *Store) GetSchemaVersion(
	ctx context.Context,
	organizationID string,
	formID string,
	version int,
) (*model.SchemaVersion, error) {
	var record schemaRecord
	if err := s.db.GetDB().WithContext(ctx).Table("form_schemas").Select("form_schemas.*").
		Joins("JOIN forms ON forms.uuid = form_schemas.form_id").
		Where("forms.deleted_at IS NULL AND forms.organization_id = ? AND form_schemas.form_id = ? AND form_schemas.version = ?", organizationID, formID, version).
		First(&record).Error; err != nil {
		return nil, fmt.Errorf("get schema version: %w", err)
	}
	return restoreSchema(record)
}

// ListSchemaVersions returns one organization-owned page, newest version first.
func (s *Store) ListSchemaVersions(
	ctx context.Context,
	organizationID string,
	formID string,
	limit int,
	offset int,
) ([]*model.SchemaVersion, int64, error) {
	if limit < 1 || limit > 100 || offset < 0 || offset > 10000 {
		return nil, 0, errors.New("schema version page is out of bounds")
	}
	query := s.db.GetDB().WithContext(ctx).Table("form_schemas").
		Joins("JOIN forms ON forms.uuid = form_schemas.form_id").
		Where("forms.deleted_at IS NULL AND forms.organization_id = ? AND form_schemas.form_id = ?", organizationID, formID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count schema versions: %w", err)
	}
	var records []schemaRecord
	if err := query.Select("form_schemas.*").Order("form_schemas.version DESC").
		Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("list schema versions: %w", err)
	}
	versions := make([]*model.SchemaVersion, 0, len(records))
	for _, record := range records {
		version, err := restoreSchema(record)
		if err != nil {
			return nil, 0, err
		}
		versions = append(versions, version)
	}
	return versions, total, nil
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
	organizationID string,
	formID string,
	version int,
) (*model.SchemaVersion, error) {
	var published *model.SchemaVersion
	err := s.db.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var formModel model.Form
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"organization_id = ? AND uuid = ?", organizationID, formID,
		).First(&formModel).Error; err != nil {
			return fmt.Errorf("lock form: %w", err)
		}
		if formModel.Status == model.LifecyclePublished && version < formModel.CurrentSchemaVersion {
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
		if err := tx.Model(&model.Form{}).Where("organization_id = ? AND uuid = ?", organizationID, formID).Updates(map[string]any{
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

// CreateSubmissionIdempotent atomically inserts or returns the original submission for a replayed key.
func (s *Store) CreateSubmissionIdempotent(
	ctx context.Context,
	submission *model.FormSubmission,
) (*model.FormSubmission, bool, error) {
	if submission == nil || submission.IdempotencyKey == "" {
		return nil, false, errors.New("submission and idempotency key are required")
	}
	var stored *model.FormSubmission
	replayed := false
	err := s.db.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if existing, found, err := findIdempotentSubmission(tx, submission.FormID, submission.IdempotencyKey); err != nil {
			return err
		} else if found {
			stored, replayed = existing, true
			return nil
		}

		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", submission.FormID).Error; err != nil {
			return fmt.Errorf("lock form submission admission: %w", err)
		}
		if existing, found, err := findIdempotentSubmission(tx, submission.FormID, submission.IdempotencyKey); err != nil {
			return err
		} else if found {
			stored, replayed = existing, true
			return nil
		}

		var recent int64
		windowStart := s.now().UTC().Add(-24 * time.Hour)
		if err := tx.Model(&model.FormSubmission{}).Where(
			"form_id = ? AND submitted_at >= ?", submission.FormID, windowStart,
		).Count(&recent).Error; err != nil {
			return fmt.Errorf("count recent form submissions: %w", err)
		}
		if recent >= int64(s.dailySubmissionLimit) {
			return form.ErrSubmissionLimitExceeded
		}

		result := tx.Clauses(clause.OnConflict{
			Columns:     []clause.Column{{Name: "form_id"}, {Name: "idempotency_key"}},
			TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "idempotency_key IS NOT NULL"}}},
			DoNothing:   true,
		}).Create(submission)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 1 {
			stored = submission
			return enqueueWebhookDelivery(tx, submission, s.now().UTC())
		}
		existing, found, err := findIdempotentSubmission(tx, submission.FormID, submission.IdempotencyKey)
		if err != nil {
			return err
		}
		if !found {
			return errors.New("idempotent submission conflict did not return an existing row")
		}
		stored, replayed = existing, true
		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("create idempotent submission: %w", err)
	}
	return stored, replayed, nil
}

func findIdempotentSubmission(tx *gorm.DB, formID, idempotencyKey string) (*model.FormSubmission, bool, error) {
	var existing model.FormSubmission
	result := tx.Where("form_id = ? AND idempotency_key = ?", formID, idempotencyKey).Limit(1).Find(&existing)
	if result.Error != nil {
		return nil, false, fmt.Errorf("load idempotent submission: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	return &existing, true, nil
}

// ListSubmissionsPage returns one deterministic, bounded cursor page.
func (s *Store) ListSubmissionsPage(
	ctx context.Context,
	organizationID string,
	formID string,
	options submission.ListOptions,
) ([]*model.FormSubmission, bool, error) {
	if err := options.Validate(); err != nil {
		return nil, false, err
	}
	query := s.db.GetDB().WithContext(ctx).Model(&model.FormSubmission{}).
		Select("form_submissions.*").
		Joins("JOIN forms ON forms.uuid = form_submissions.form_id").
		Where("forms.deleted_at IS NULL AND forms.organization_id = ? AND form_submissions.form_id = ?", organizationID, formID)
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
		query = query.Where(
			"(form_submissions.submitted_at < ? OR (form_submissions.submitted_at = ? AND form_submissions.uuid < ?))",
			options.Before, options.Before, options.BeforeID,
		)
	}
	var submissions []*model.FormSubmission
	if err := query.Order("form_submissions.submitted_at DESC, form_submissions.uuid DESC").Limit(options.Limit + 1).
		Find(&submissions).Error; err != nil {
		return nil, false, fmt.Errorf("list submission page: %w", err)
	}
	hasMore := len(submissions) > options.Limit
	if hasMore {
		submissions = submissions[:options.Limit]
	}
	return submissions, hasMore, nil
}

// GetSubmissionByOrganization resolves a submission through its owning form so
// foreign and absent identifiers have the same repository result.
func (s *Store) GetSubmissionByOrganization(
	ctx context.Context,
	organizationID string,
	formID string,
	submissionID string,
) (*model.FormSubmission, error) {
	if _, err := uuid.Parse(formID); err != nil {
		return nil, fmt.Errorf("get submission: %w", common.NewNotFoundError("get", "submission", submissionID))
	}
	if _, err := uuid.Parse(submissionID); err != nil {
		return nil, fmt.Errorf("get submission: %w", common.NewNotFoundError("get", "submission", submissionID))
	}
	var submission model.FormSubmission
	result := s.db.GetDB().WithContext(ctx).
		Table("form_submissions").
		Select("form_submissions.*").
		Joins("JOIN forms ON forms.uuid = form_submissions.form_id").
		Where("forms.deleted_at IS NULL AND forms.organization_id = ? AND forms.uuid = ? AND form_submissions.uuid = ?", organizationID, formID, submissionID).
		First(&submission)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("get submission: %w", common.NewNotFoundError("get", "submission", submissionID))
	}
	if result.Error != nil {
		return nil, fmt.Errorf("get submission: %w", common.NewDatabaseError("get", "submission", submissionID, result.Error))
	}
	return &submission, nil
}
