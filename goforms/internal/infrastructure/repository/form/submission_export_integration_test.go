package repository_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/submission"
	formrepository "github.com/goformx/goforms/internal/infrastructure/repository/form"
	mocklogging "github.com/goformx/goforms/test/mocks/logging"
)

func TestSubmissionExportSnapshotBudgetsAndDurableAudit(t *testing.T) {
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PostgreSQL integration is run by task verify")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	owner := uuid.NewString()
	t.Cleanup(func() { require.NoError(t, db.Exec("DELETE FROM forms WHERE organization_id = ?", owner).Error) })
	store := formrepository.NewStore(&integrationDB{db: db}, mocklogging.NewMockLogger(gomock.NewController(t)))
	schema := model.JSON{"$schema": model.JSONSchemaDraft202012URI, "type": "object", "properties": map[string]any{"secret": map[string]any{"type": "string"}},
		submission.SensitiveAnnotation: []any{"/secret"}}
	form := model.NewForm(owner, "Export fixture", "", schema)
	form.Name = "export-fixture-" + uuid.NewString()[:8]
	require.NoError(t, store.CreateForm(t.Context(), form))
	_, err = store.PublishSchemaVersion(t.Context(), owner, form.ID, 1)
	require.NoError(t, err)
	start := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	rows := make([]*model.FormSubmission, 2)
	for index := range rows {
		rows[index] = &model.FormSubmission{FormID: form.ID, SchemaVersion: 1, RequestID: "req_fixture", IdempotencyKey: uuid.NewString(),
			Data: model.JSON{"secret": "private-canary", "visible": index}, SubmittedAt: start.Add(time.Duration(index) * time.Second), Status: model.SubmissionStatusAccepted}
	}
	require.NoError(t, db.Create(&rows).Error)
	selected, err := store.ReadSubmissionExport(t.Context(), owner, form.ID, submission.ExportFilters{})
	require.NoError(t, err)
	require.Len(t, selected, 2)
	require.Equal(t, rows[1].ID, selected[0].Submission.ID)
	require.Equal(t, 1, selected[0].AcceptedVersion)
	require.Equal(t, form.ID, selected[0].SchemaFormID)
	require.Equal(t, []any{"/secret"}, selected[0].Policy[submission.SensitiveAnnotation])
	foreign, err := store.ReadSubmissionExport(t.Context(), uuid.NewString(), form.ID, submission.ExportFilters{})
	require.NoError(t, err)
	require.Empty(t, foreign)
	end := start.Add(time.Second)
	filtered, err := store.ReadSubmissionExport(t.Context(), owner, form.ID, submission.ExportFilters{ReceivedFrom: &start, ReceivedBefore: &end, Status: model.SubmissionStatusAccepted, SchemaVersion: 1})
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	require.Equal(t, rows[0].ID, filtered[0].Submission.ID)
	audit := submission.ExportAudit{ID: uuid.NewString(), OrganizationID: owner, FormID: form.ID, SubjectID: "actor-1", CredentialClass: "service_token",
		CredentialID: "token-id", RequestID: "req_fixture", Format: submission.ExportJSON, RowCount: 2, ByteCount: 100, PreparedAt: time.Now()}
	require.NoError(t, store.SaveSubmissionExportAudit(t.Context(), audit))
	var event string
	require.NoError(t, db.Raw("SELECT event FROM submission_export_audit WHERE export_id = ?", audit.ID).Scan(&event).Error)
	require.Equal(t, "export.prepared", event)
	require.Error(t, db.Exec("UPDATE submission_export_audit SET byte_count = 99 WHERE export_id = ?", audit.ID).Error)
	require.Error(t, db.Exec("DELETE FROM submission_export_audit WHERE export_id = ?", audit.ID).Error)
	// Run global audit retention probes inside a rolled-back transaction so even
	// a missing guard cannot erase another test's rows or table.
	retentionProbe := db.Begin()
	require.NoError(t, retentionProbe.Error)
	truncateErr := retentionProbe.Exec("TRUNCATE submission_export_audit").Error
	require.NoError(t, retentionProbe.Rollback().Error)
	require.ErrorContains(t, truncateErr, "append-only")
	down, err := os.ReadFile("../../../../migrations/postgresql/2026083002_submission_export_audit.down.sql")
	require.NoError(t, err)
	retentionProbe = db.Begin()
	require.NoError(t, retentionProbe.Error)
	downErr := retentionProbe.Exec(string(down)).Error
	require.NoError(t, retentionProbe.Rollback().Error)
	require.ErrorContains(t, downErr, "cannot roll back a populated")
	foreignAudit := audit
	foreignAudit.ID, foreignAudit.OrganizationID = uuid.NewString(), uuid.NewString()
	require.Error(t, store.SaveSubmissionExportAudit(t.Context(), foreignAudit))
	// A large historical row must be rejected by the SQL source bound, not read
	// wholesale and checked only after JSON decoding in the application.
	require.NoError(t, db.Exec("UPDATE form_submissions SET data = json_build_object('large', repeat('x', ?)) WHERE uuid = ?", submission.MaxExportSourceBytes+1, rows[0].ID).Error)
	selected, err = store.ReadSubmissionExport(t.Context(), owner, form.ID, submission.ExportFilters{})
	require.ErrorIs(t, err, submission.ErrExportLimit)
	require.Nil(t, selected)
	require.NoError(t, db.Exec("UPDATE form_submissions SET data = '{}'::json WHERE uuid = ?", rows[0].ID).Error)
	// SQL inserts avoid public admission budgets; fixtures remain isolated by form.
	require.NoError(t, db.Exec(`INSERT INTO form_submissions (uuid, form_id, schema_version, request_id, status, data, submitted_at)
		SELECT gen_random_uuid()::text, ?, 1, 'req_fixture', 'accepted', '{}'::json, ? FROM generate_series(1, ?)`,
		form.ID, start, submission.MaxExportRows-2).Error)
	selected, err = store.ReadSubmissionExport(t.Context(), owner, form.ID, submission.ExportFilters{})
	require.NoError(t, err)
	require.Len(t, selected, submission.MaxExportRows)
	require.NoError(t, db.Exec(`INSERT INTO form_submissions (uuid, form_id, schema_version, request_id, status, data, submitted_at)
		VALUES (?, ?, 1, 'req_fixture', 'accepted', '{}'::json, ?)`, uuid.NewString(), form.ID, start).Error)
	selected, err = store.ReadSubmissionExport(t.Context(), owner, form.ID, submission.ExportFilters{})
	require.ErrorIs(t, err, submission.ErrExportLimit)
	require.Nil(t, selected)
	require.NoError(t, db.Exec("UPDATE forms SET deleted_at = now() WHERE uuid = ?", form.ID).Error)
	selected, err = store.ReadSubmissionExport(t.Context(), owner, form.ID, submission.ExportFilters{})
	require.NoError(t, err)
	require.Empty(t, selected)
	audit.ID = uuid.NewString()
	require.Error(t, store.SaveSubmissionExportAudit(t.Context(), audit))
	var count int64
	require.NoError(t, db.Table("submission_export_audit").Where("organization_id = ?", owner).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Exec("DELETE FROM forms WHERE uuid = ?", form.ID).Error)
	require.NoError(t, db.Table("submission_export_audit").Where("organization_id = ?", owner).Count(&count).Error)
	require.EqualValues(t, 1, count, "Form deletion must not cascade to audit history")
	var columns []string
	require.NoError(t, db.Raw("SELECT column_name FROM information_schema.columns WHERE table_name = 'submission_export_audit' AND table_schema = 'public'").Scan(&columns).Error)
	for _, forbidden := range []string{"payload", "data", "filters", "token", "schema"} {
		require.NotContains(t, columns, forbidden)
	}
	require.False(t, strings.Contains(strings.Join(columns, ","), "secret"))
}
