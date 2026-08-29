package repository

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/goformx/goforms/internal/domain/form/model"
	domainwebhook "github.com/goformx/goforms/internal/domain/webhook"
)

type webhookEndpointRecord struct {
	UUID              string
	FormID            string
	DestinationOrigin string
	EncryptedConfig   []byte
	Enabled           bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (webhookEndpointRecord) TableName() string { return "webhook_endpoints" }

type webhookDeliveryRecord struct {
	UUID              string
	SubmissionID      string
	FormID            string
	EndpointID        string
	DestinationOrigin string
	EncryptedConfig   []byte
	Status            domainwebhook.DeliveryStatus
	AttemptCount      int
	NextAttemptAt     time.Time
	LockedAt          *time.Time
	DeliveredAt       *time.Time
	LastHTTPStatus    *int
	LastErrorCategory string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (webhookDeliveryRecord) TableName() string { return "webhook_deliveries" }

func enqueueWebhookDelivery(tx *gorm.DB, submission *model.FormSubmission, now time.Time) error {
	var endpoint webhookEndpointRecord
	result := tx.Where("form_id = ? AND enabled = true", submission.FormID).Limit(1).Find(&endpoint)
	if result.Error != nil {
		return fmt.Errorf("load webhook endpoint: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil
	}
	delivery := webhookDeliveryRecord{
		UUID: uuid.NewString(), SubmissionID: submission.ID, FormID: submission.FormID, EndpointID: endpoint.UUID,
		DestinationOrigin: endpoint.DestinationOrigin,
		EncryptedConfig:   append([]byte(nil), endpoint.EncryptedConfig...),
		Status:            domainwebhook.DeliveryPending, NextAttemptAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.Create(&delivery).Error; err != nil {
		return fmt.Errorf("enqueue webhook delivery: %w", err)
	}
	return nil
}

func (s *Store) PutWebhookEndpoint(
	ctx context.Context,
	organizationID, formID, destinationURL string,
	config domainwebhook.SecretConfig,
	enabled bool,
) (*domainwebhook.Endpoint, error) {
	if s.webhookCipher == nil {
		return nil, domainwebhook.ErrDisabled
	}
	owned, err := s.organizationOwnsForm(ctx, organizationID, formID)
	if err != nil {
		return nil, err
	}
	if !owned {
		return nil, domainwebhook.ErrNotFound
	}
	target, err := url.Parse(destinationURL)
	if err != nil || target.Scheme != "https" || target.Host == "" {
		return nil, errors.New("webhook destination URL is invalid")
	}
	config.DestinationURL = destinationURL
	encrypted, err := s.webhookCipher.Encrypt(config, formID)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	origin := target.Scheme + "://" + target.Host
	record := webhookEndpointRecord{UUID: uuid.NewString(), FormID: formID, DestinationOrigin: origin,
		EncryptedConfig: encrypted, Enabled: enabled, CreatedAt: now, UpdatedAt: now}
	err = s.db.GetDB().WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "form_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"destination_origin": origin, "encrypted_config": encrypted, "enabled": enabled, "updated_at": now,
		}),
	}).Create(&record).Error
	if err != nil {
		return nil, fmt.Errorf("put webhook endpoint: %w", err)
	}
	return s.GetWebhookEndpoint(ctx, organizationID, formID)
}

func (s *Store) GetWebhookEndpoint(ctx context.Context, organizationID, formID string) (*domainwebhook.Endpoint, error) {
	var record webhookEndpointRecord
	if err := s.db.GetDB().WithContext(ctx).Table("webhook_endpoints").Select("webhook_endpoints.*").
		Joins("JOIN forms ON forms.uuid = webhook_endpoints.form_id").
		Where("forms.deleted_at IS NULL AND forms.organization_id = ? AND webhook_endpoints.form_id = ?", organizationID, formID).
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainwebhook.ErrNotFound
		}
		return nil, fmt.Errorf("get webhook endpoint: %w", err)
	}
	return restoreEndpoint(record), nil
}

func restoreEndpoint(record webhookEndpointRecord) *domainwebhook.Endpoint {
	return &domainwebhook.Endpoint{ID: record.UUID, FormID: record.FormID, Origin: record.DestinationOrigin,
		Enabled: record.Enabled, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}

func (s *Store) DeleteWebhookEndpoint(ctx context.Context, organizationID, formID string) error {
	ownedForms := s.db.GetDB().Table("forms").Select("uuid").
		Where("deleted_at IS NULL AND organization_id = ? AND uuid = ?", organizationID, formID)
	result := s.db.GetDB().WithContext(ctx).Where("form_id IN (?)", ownedForms).Delete(&webhookEndpointRecord{})
	if result.Error != nil {
		return fmt.Errorf("delete webhook endpoint: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domainwebhook.ErrNotFound
	}
	return nil
}

func (s *Store) ListWebhookDeliveries(
	ctx context.Context,
	organizationID string,
	formID string,
	limit int,
) ([]*domainwebhook.Delivery, error) {
	if limit < 1 || limit > 100 {
		return nil, errors.New("webhook delivery limit must be between 1 and 100")
	}
	var records []webhookDeliveryRecord
	if err := s.db.GetDB().WithContext(ctx).Table("webhook_deliveries").Select("webhook_deliveries.*").
		Joins("JOIN forms ON forms.uuid = webhook_deliveries.form_id").
		Where("forms.deleted_at IS NULL AND forms.organization_id = ? AND webhook_deliveries.form_id = ?", organizationID, formID).
		Order("webhook_deliveries.created_at DESC, webhook_deliveries.uuid DESC").Limit(limit).
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list webhook deliveries: %w", err)
	}
	deliveries := make([]*domainwebhook.Delivery, 0, len(records))
	for _, record := range records {
		deliveries = append(deliveries, restoreDelivery(record))
	}
	return deliveries, nil
}

func (s *Store) ReplayWebhookDelivery(ctx context.Context, organizationID, formID, deliveryID string) error {
	now := s.now().UTC()
	ownedForms := s.db.GetDB().Table("forms").Select("uuid").
		Where("deleted_at IS NULL AND organization_id = ? AND uuid = ?", organizationID, formID)
	result := s.db.GetDB().WithContext(ctx).Model(&webhookDeliveryRecord{}).
		Where("uuid = ? AND form_id IN (?) AND status = ?", deliveryID, ownedForms, domainwebhook.DeliveryDeadLetter).
		Updates(map[string]any{"status": domainwebhook.DeliveryPending, "next_attempt_at": now,
			"attempt_count": 0, "locked_at": nil, "delivered_at": nil,
			"last_error_category": "", "last_http_status": nil})
	if result.Error != nil {
		return fmt.Errorf("replay webhook delivery: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domainwebhook.ErrNotFound
	}
	return nil
}

func (s *Store) organizationOwnsForm(ctx context.Context, organizationID, formID string) (bool, error) {
	var count int64
	if err := s.db.GetDB().WithContext(ctx).Model(&model.Form{}).
		Where("organization_id = ? AND uuid = ?", organizationID, formID).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check form ownership: %w", err)
	}
	return count == 1, nil
}

func (s *Store) ClaimDelivery(
	ctx context.Context,
	lockTimeout time.Duration,
) (*domainwebhook.Delivery, *domainwebhook.Event, error) {
	var delivery webhookDeliveryRecord
	var submission model.FormSubmission
	now := s.now().UTC()
	staleBefore := now.Add(-lockTimeout)
	err := s.db.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Raw(`
			WITH candidate AS (
				SELECT uuid
				FROM webhook_deliveries
				WHERE (status = 'pending' AND next_attempt_at <= ?)
				   OR (status = 'processing' AND locked_at < ?)
				ORDER BY next_attempt_at ASC, created_at ASC
				FOR UPDATE SKIP LOCKED
				LIMIT 1
			)
			UPDATE webhook_deliveries AS delivery
			SET status = 'processing',
			    attempt_count = delivery.attempt_count + 1,
			    locked_at = ?,
			    updated_at = ?
			FROM candidate
			WHERE delivery.uuid = candidate.uuid
			RETURNING delivery.*
		`, now, staleBefore, now, now).Scan(&delivery)
		if query.Error != nil {
			return fmt.Errorf("claim webhook delivery: %w", query.Error)
		}
		if query.RowsAffected == 0 {
			return nil
		}
		if err := tx.Where("uuid = ?", delivery.SubmissionID).First(&submission).Error; err != nil {
			return fmt.Errorf("load webhook submission: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if delivery.UUID == "" {
		return nil, nil, nil
	}
	event := &domainwebhook.Event{ID: delivery.UUID, Type: "submission.accepted",
		CreatedAt: submission.SubmittedAt.UTC(), SubmissionID: submission.ID, FormID: submission.FormID,
		SchemaVersion: submission.SchemaVersion, Data: map[string]any(submission.Data)}
	return restoreDelivery(delivery), event, nil
}

func (s *Store) MarkDeliveryDelivered(ctx context.Context, deliveryID string, status int, now time.Time) error {
	result := s.db.GetDB().WithContext(ctx).Model(&webhookDeliveryRecord{}).
		Where("uuid = ? AND status = ?", deliveryID, domainwebhook.DeliveryProcessing).
		Updates(map[string]any{"status": domainwebhook.DeliveryDelivered, "delivered_at": now,
			"locked_at": nil, "last_http_status": status, "last_error_category": ""})
	if result.Error != nil {
		return fmt.Errorf("mark webhook delivered: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domainwebhook.ErrNotFound
	}
	return nil
}

func (s *Store) MarkDeliveryFailed(
	ctx context.Context,
	deliveryID, category string,
	httpStatus *int,
	retryable bool,
	maxAttempts int,
	backoffBase, backoffMax time.Duration,
	now time.Time,
) error {
	return s.db.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record webhookDeliveryRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("uuid = ? AND status = ?", deliveryID, domainwebhook.DeliveryProcessing).
			First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domainwebhook.ErrNotFound
			}
			return err
		}
		status := domainwebhook.DeliveryDeadLetter
		nextAttempt := record.NextAttemptAt
		if retryable && record.AttemptCount < maxAttempts {
			status = domainwebhook.DeliveryPending
			nextAttempt = now.Add(deliveryBackoff(record.AttemptCount, backoffBase, backoffMax))
		}
		return tx.Model(&webhookDeliveryRecord{}).Where("uuid = ?", record.UUID).Updates(map[string]any{
			"status": status, "next_attempt_at": nextAttempt, "locked_at": nil,
			"last_http_status": httpStatus, "last_error_category": category,
		}).Error
	})
}

func deliveryBackoff(attempt int, base, maximum time.Duration) time.Duration {
	delay := base
	for current := 1; current < attempt && delay < maximum; current++ {
		if delay > maximum/2 {
			return maximum
		}
		delay *= 2
	}
	if delay > maximum {
		return maximum
	}
	return delay
}

func restoreDelivery(record webhookDeliveryRecord) *domainwebhook.Delivery {
	return &domainwebhook.Delivery{ID: record.UUID, SubmissionID: record.SubmissionID, FormID: record.FormID,
		EndpointID: record.EndpointID, DestinationOrigin: record.DestinationOrigin,
		EncryptedConfig: append([]byte(nil), record.EncryptedConfig...), Status: record.Status,
		AttemptCount: record.AttemptCount, NextAttemptAt: record.NextAttemptAt, LockedAt: record.LockedAt,
		DeliveredAt: record.DeliveredAt, LastHTTPStatus: record.LastHTTPStatus,
		LastErrorCategory: record.LastErrorCategory, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
}
