package repository_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	domainform "github.com/goformx/goforms/internal/domain/form"
	"github.com/goformx/goforms/internal/domain/form/model"
	domainwebhook "github.com/goformx/goforms/internal/domain/webhook"
	formrepository "github.com/goformx/goforms/internal/infrastructure/repository/form"
	mocklogging "github.com/goformx/goforms/test/mocks/logging"
)

type integrationDB struct{ db *gorm.DB }

func (d *integrationDB) Close() error                          { return nil }
func (d *integrationDB) MonitorConnectionPool(context.Context) {}
func (d *integrationDB) Ping(ctx context.Context) error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}
func (d *integrationDB) GetDB() *gorm.DB { return d.db }

func TestStorePersistsImmutableVersionsAndPublicKeys(t *testing.T) {
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GOFORMX_TEST_DATABASE_URL is not set")
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	require.NoError(t, err)
	ownerID := "11111111-1111-4111-8111-111111111111"
	require.NoError(t, db.Exec(`
		INSERT INTO users (uuid, email, hashed_password, first_name, last_name)
		VALUES (?, 'form-fixture@example.test', 'not-used', 'Form', 'Fixture')
		ON CONFLICT (uuid) DO NOTHING
	`, ownerID).Error)
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM forms WHERE organization_id = ?", ownerID).Error
		_ = db.Exec("DELETE FROM users WHERE uuid = ?", ownerID).Error
	})
	ctrl := gomock.NewController(t)
	store := formrepository.NewStore(&integrationDB{db: db}, mocklogging.NewMockLogger(ctrl))

	form := model.NewForm(ownerID, "Agent Contact", "", model.JSON{
		"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
		"properties": map[string]any{"name": map[string]any{"type": "string"}}, "required": []any{"name"},
	})
	form.Name = "agent-contact-" + uuid.NewString()[:8]
	require.NoError(t, store.CreateForm(t.Context(), form))
	require.Regexp(t, `^gfpk_`, form.PublicKey)
	require.Equal(t, 1, form.CurrentSchemaVersion)

	form.Status = model.LifecyclePublished
	require.NoError(t, store.UpdateForm(t.Context(), form))
	publicForm, err := store.GetFormByID(t.Context(), ownerID, form.ID)
	require.NoError(t, err)
	require.Equal(t, form.ID, publicForm.ID)
	require.Equal(t, "object", publicForm.Schema["type"])

	form.Schema = model.JSON{
		"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
		"properties": map[string]any{"email": map[string]any{"type": "string", "format": "email"}},
		"required":   []any{"email"},
	}
	version, err := store.CreateSchemaVersion(t.Context(), form.ID, form.Schema)
	require.NoError(t, err)
	require.Equal(t, 2, version.Version())
	published, err := store.PublishSchemaVersion(t.Context(), form.ID, 2)
	require.NoError(t, err)
	require.Equal(t, model.SchemaVersionPublished, published.State())
	publicForm, exactVersion, err := store.GetPublishedSchemaVersion(t.Context(), form.PublicKey, 2)
	require.NoError(t, err)
	require.Equal(t, form.ID, publicForm.ID)
	require.Equal(t, 2, exactVersion.Version())
	_, err = store.PublishSchemaVersion(t.Context(), form.ID, 1)
	require.ErrorContains(t, err, "cannot move backwards")

	submission := &model.FormSubmission{FormID: form.ID, SchemaVersion: 1,
		IdempotencyKey: "repository-contact-0001", Data: model.JSON{"name": "Ada"},
		SubmittedAt: time.Now().UTC(), Status: model.SubmissionStatusAccepted}
	created, replayed, err := store.CreateSubmissionIdempotent(t.Context(), submission)
	require.NoError(t, err)
	require.False(t, replayed)
	require.NotEmpty(t, created.ID)
	retry := &model.FormSubmission{FormID: form.ID, SchemaVersion: 1,
		IdempotencyKey: submission.IdempotencyKey, Data: model.JSON{"name": "Ada"},
		SubmittedAt: time.Now().UTC(), Status: model.SubmissionStatusAccepted}
	replayedSubmission, replayed, err := store.CreateSubmissionIdempotent(t.Context(), retry)
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, created.ID, replayedSubmission.ID)

	limitedStore := formrepository.NewStoreWithDailySubmissionLimit(
		&integrationDB{db: db}, mocklogging.NewMockLogger(ctrl), 1,
	)
	overLimit := &model.FormSubmission{FormID: form.ID, SchemaVersion: 1,
		IdempotencyKey: "repository-contact-0002", Data: model.JSON{"name": "Grace"},
		SubmittedAt: time.Now().UTC(), Status: model.SubmissionStatusAccepted}
	_, _, err = limitedStore.CreateSubmissionIdempotent(t.Context(), overLimit)
	require.ErrorIs(t, err, domainform.ErrSubmissionLimitExceeded)
	replayedSubmission, replayed, err = limitedStore.CreateSubmissionIdempotent(t.Context(), retry)
	require.NoError(t, err)
	require.True(t, replayed)
	require.Equal(t, created.ID, replayedSubmission.ID)
	second, replayed, err := store.CreateSubmissionIdempotent(t.Context(), overLimit)
	require.NoError(t, err)
	require.False(t, replayed)
	require.NotEmpty(t, second.ID)

	page, hasMore, err := store.ListSubmissionsPage(t.Context(), form.ID, time.Time{}, "", 1)
	require.NoError(t, err)
	require.True(t, hasMore)
	require.Len(t, page, 1)
	nextPage, hasMore, err := store.ListSubmissionsPage(
		t.Context(), form.ID, page[0].SubmittedAt, page[0].ID, 1,
	)
	require.NoError(t, err)
	require.False(t, hasMore)
	require.Len(t, nextPage, 1)
	require.NotEqual(t, page[0].ID, nextPage[0].ID)

	concurrentForm := model.NewForm(ownerID, "Concurrent Admission", "", model.JSON{
		"$schema": model.JSONSchemaDraft202012URI, "type": "object",
		"properties": map[string]any{"name": map[string]any{"type": "string"}},
	})
	concurrentForm.Name = "concurrent-admission-" + uuid.NewString()[:8]
	require.NoError(t, store.CreateForm(t.Context(), concurrentForm))
	concurrentStore := formrepository.NewStoreWithDailySubmissionLimit(
		&integrationDB{db: db}, mocklogging.NewMockLogger(ctrl), 1,
	)
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for index := 0; index < 2; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			_, _, createErr := concurrentStore.CreateSubmissionIdempotent(t.Context(), &model.FormSubmission{
				FormID: concurrentForm.ID, SchemaVersion: 1,
				IdempotencyKey: fmt.Sprintf("concurrent-submit-%04d", index),
				Data:           model.JSON{"name": "Ada"}, SubmittedAt: time.Now().UTC(),
				Status: model.SubmissionStatusAccepted,
			})
			results <- createErr
		}(index)
	}
	wait.Wait()
	close(results)
	accepted, rejected := 0, 0
	for createErr := range results {
		if createErr == nil {
			accepted++
		} else if errors.Is(createErr, domainform.ErrSubmissionLimitExceeded) {
			rejected++
		} else {
			require.NoError(t, createErr)
		}
	}
	require.Equal(t, 1, accepted)
	require.Equal(t, 1, rejected)

	webhookKey := sha256.Sum256([]byte("repository webhook encryption key"))
	webhookCipher, err := domainwebhook.NewCipher(base64.RawStdEncoding.EncodeToString(webhookKey[:]))
	require.NoError(t, err)
	webhookStore := formrepository.NewStoreWithOptions(&integrationDB{db: db}, mocklogging.NewMockLogger(ctrl),
		formrepository.StoreOptions{DailySubmissionLimit: 100, WebhookCipher: webhookCipher})
	webhookForm := model.NewForm(ownerID, "Webhook Atomicity", "", model.JSON{
		"$schema": model.JSONSchemaDraft202012URI, "type": "object",
		"properties": map[string]any{"name": map[string]any{"type": "string"}},
	})
	webhookForm.Name = "webhook-atomicity-" + uuid.NewString()[:8]
	require.NoError(t, webhookStore.CreateForm(t.Context(), webhookForm))
	t.Cleanup(func() {
		_ = db.Unscoped().Where("uuid = ?", webhookForm.ID).Delete(&model.Form{}).Error
	})
	_, err = webhookStore.PutWebhookEndpoint(t.Context(), webhookForm.ID, "https://hooks.example/receive",
		domainwebhook.SecretConfig{Headers: map[string]string{"Authorization": "Bearer encrypted"},
			SigningSecret: "repository-signing-secret-long-enough"}, true)
	require.NoError(t, err)
	webhookSubmission := &model.FormSubmission{FormID: webhookForm.ID, SchemaVersion: 1,
		IdempotencyKey: "webhook-atomic-submit-0001", Data: model.JSON{"name": "Ada"},
		SubmittedAt: time.Now().UTC(), Status: model.SubmissionStatusAccepted}
	storedWebhookSubmission, replayed, err := webhookStore.CreateSubmissionIdempotent(t.Context(), webhookSubmission)
	require.NoError(t, err)
	require.False(t, replayed)
	deliveries, err := webhookStore.ListWebhookDeliveries(t.Context(), webhookForm.ID, 25)
	require.NoError(t, err)
	require.Len(t, deliveries, 1)
	require.Equal(t, storedWebhookSubmission.ID, deliveries[0].SubmissionID)
	require.Equal(t, "https://hooks.example", deliveries[0].DestinationOrigin)
	require.NotContains(t, string(deliveries[0].EncryptedConfig), "/receive")
	decrypted, err := webhookCipher.Decrypt(deliveries[0].EncryptedConfig, webhookForm.ID)
	require.NoError(t, err)
	require.Equal(t, "https://hooks.example/receive", decrypted.DestinationURL)
	require.Equal(t, "Bearer encrypted", decrypted.Headers["Authorization"])

	_, replayed, err = webhookStore.CreateSubmissionIdempotent(t.Context(), &model.FormSubmission{
		FormID: webhookForm.ID, SchemaVersion: 1, IdempotencyKey: webhookSubmission.IdempotencyKey,
		Data: model.JSON{"name": "Ada"}, SubmittedAt: time.Now().UTC(), Status: model.SubmissionStatusAccepted,
	})
	require.NoError(t, err)
	require.True(t, replayed)
	deliveries, err = webhookStore.ListWebhookDeliveries(t.Context(), webhookForm.ID, 25)
	require.NoError(t, err)
	require.Len(t, deliveries, 1)

	claimed, event, err := webhookStore.ClaimDelivery(t.Context(), time.Minute)
	require.NoError(t, err)
	require.Equal(t, deliveries[0].ID, claimed.ID)
	require.Equal(t, storedWebhookSubmission.ID, event.SubmissionID)
	require.NoError(t, webhookStore.MarkDeliveryFailed(t.Context(), claimed.ID, "network", nil,
		true, 1, time.Second, time.Minute, time.Now().UTC()))
	deliveries, err = webhookStore.ListWebhookDeliveries(t.Context(), webhookForm.ID, 25)
	require.NoError(t, err)
	require.Equal(t, domainwebhook.DeliveryDeadLetter, deliveries[0].Status)
	require.NoError(t, webhookStore.ReplayWebhookDelivery(t.Context(), webhookForm.ID, claimed.ID))
	deliveries, err = webhookStore.ListWebhookDeliveries(t.Context(), webhookForm.ID, 25)
	require.NoError(t, err)
	require.Equal(t, domainwebhook.DeliveryPending, deliveries[0].Status)
	require.Zero(t, deliveries[0].AttemptCount)

	require.NoError(t, db.Exec(`
		CREATE OR REPLACE FUNCTION goformx_test_reject_outbox() RETURNS trigger AS $$
		BEGIN
			IF NEW.destination_origin = 'https://reject.example' THEN
				RAISE EXCEPTION 'intentional outbox failure';
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER goformx_test_reject_outbox
		BEFORE INSERT ON webhook_deliveries
		FOR EACH ROW EXECUTE FUNCTION goformx_test_reject_outbox();
	`).Error)
	t.Cleanup(func() {
		_ = db.Exec("DROP TRIGGER IF EXISTS goformx_test_reject_outbox ON webhook_deliveries").Error
		_ = db.Exec("DROP FUNCTION IF EXISTS goformx_test_reject_outbox()").Error
	})
	_, err = webhookStore.PutWebhookEndpoint(t.Context(), webhookForm.ID, "https://reject.example/hooks",
		domainwebhook.SecretConfig{SigningSecret: "repository-signing-secret-long-enough"}, true)
	require.NoError(t, err)
	rollbackKey := "webhook-rollback-submit-0002"
	_, _, err = webhookStore.CreateSubmissionIdempotent(t.Context(), &model.FormSubmission{
		FormID: webhookForm.ID, SchemaVersion: 1, IdempotencyKey: rollbackKey,
		Data: model.JSON{"name": "Grace"}, SubmittedAt: time.Now().UTC(), Status: model.SubmissionStatusAccepted,
	})
	require.ErrorContains(t, err, "intentional outbox failure")
	var rolledBack int64
	require.NoError(t, db.Model(&model.FormSubmission{}).
		Where("form_id = ? AND idempotency_key = ?", webhookForm.ID, rollbackKey).Count(&rolledBack).Error)
	require.Zero(t, rolledBack)

	var versions int64
	require.NoError(t, db.Table("form_schemas").Where("form_id = ?", form.ID).Count(&versions).Error)
	require.EqualValues(t, 2, versions)
}
