package repository_test

import (
	"context"
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
	ctrl := gomock.NewController(t)
	store := formrepository.NewStore(&integrationDB{db: db}, mocklogging.NewMockLogger(ctrl))

	form := model.NewForm("11111111-1111-4111-8111-111111111111", "Agent Contact", "", model.JSON{
		"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
		"properties": map[string]any{"name": map[string]any{"type": "string"}}, "required": []any{"name"},
	})
	form.Name = "agent-contact-" + uuid.NewString()[:8]
	require.NoError(t, store.CreateForm(t.Context(), form))
	require.Regexp(t, `^gfpk_`, form.PublicKey)
	require.Equal(t, 1, form.CurrentSchemaVersion)

	form.Status = model.LifecyclePublished
	require.NoError(t, store.UpdateForm(t.Context(), form))
	publicForm, err := store.GetFormByPublicKey(t.Context(), form.PublicKey)
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

	concurrentForm := model.NewForm("11111111-1111-4111-8111-111111111111", "Concurrent Admission", "", model.JSON{
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

	var versions int64
	require.NoError(t, db.Table("form_schemas").Where("form_id = ?", form.ID).Count(&versions).Error)
	require.EqualValues(t, 2, versions)
}
