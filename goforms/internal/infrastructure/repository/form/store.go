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
			"error", err,
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
			"error", err,
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

// ListForms retrieves all forms for an organization.
func (s *Store) ListForms(ctx context.Context, organizationID string) ([]*model.Form, error) {
	var forms []*model.Form
	if err := s.db.GetDB().WithContext(ctx).
		Where("organization_id = ?", organizationID).
		Order("created_at DESC").
		Find(&forms).Error; err != nil {
		s.logger.Error("failed to list forms",
			"organization_id", organizationID,
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
		if formModel.Status == model.LifecyclePublished {
			now := time.Now().UTC()
			if err := tx.Model(&schemaRecord{}).Where("form_id = ? AND version = ?", formModel.ID, formModel.CurrentSchemaVersion).
				Updates(map[string]any{"state": string(model.SchemaVersionPublished), "published_at": now}).Error; err != nil {
				return fmt.Errorf("publish schema version: %w", err)
			}
		}
		result := tx.Model(&model.Form{}).Where(
			"organization_id = ? AND uuid = ?", formModel.OrganizationID, formModel.ID,
		).Updates(formModel)
		if result.Error != nil {
			return fmt.Errorf("update form: %w", common.NewDatabaseError("update", "form", formModel.ID, result.Error))
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("update form: %w", common.NewNotFoundError("update", "form", formModel.ID))
		}
		return nil
	})
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
	formID string,
	before time.Time,
	beforeID string,
	limit int,
) ([]*model.FormSubmission, bool, error) {
	if limit < 1 || limit > 100 {
		return nil, false, errors.New("submission page limit must be between 1 and 100")
	}
	query := s.db.GetDB().WithContext(ctx).Where("form_id = ?", formID)
	if !before.IsZero() {
		query = query.Where("(submitted_at < ? OR (submitted_at = ? AND uuid < ?))", before, before, beforeID)
	}
	var submissions []*model.FormSubmission
	if err := query.Order("submitted_at DESC, uuid DESC").Limit(limit + 1).Find(&submissions).Error; err != nil {
		return nil, false, fmt.Errorf("list submission page: %w", err)
	}
	hasMore := len(submissions) > limit
	if hasMore {
		submissions = submissions[:limit]
	}
	return submissions, hasMore, nil
}
