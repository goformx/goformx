package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

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
	require.NoError(t, store.CreateForm(t.Context(), form))
	require.Regexp(t, `^gfpk_`, form.PublicKey)
	require.Equal(t, 1, form.CurrentSchemaVersion)

	form.Status = string(model.LifecyclePublished)
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
	form.Status = string(model.LifecycleDraft)
	require.NoError(t, store.UpdateForm(t.Context(), form))
	require.Equal(t, 2, form.CurrentSchemaVersion)

	submission := &model.FormSubmission{FormID: form.ID, SchemaVersion: 1,
		Data: model.JSON{"name": "Ada"}, SubmittedAt: time.Now().UTC(), Status: model.SubmissionStatusPending}
	require.NoError(t, store.CreateSubmission(t.Context(), submission))

	var versions int64
	require.NoError(t, db.Table("form_schemas").Where("form_id = ?", form.ID).Count(&versions).Error)
	require.EqualValues(t, 2, versions)
}
