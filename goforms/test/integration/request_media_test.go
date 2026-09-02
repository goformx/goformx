package integration_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/goformx/goforms/internal/application/handlers/web"
	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/domain/form/model"
	formrepository "github.com/goformx/goforms/internal/infrastructure/repository/form"
	tokenrepository "github.com/goformx/goforms/internal/infrastructure/repository/token"
)

func requestMediaDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PostgreSQL integration is run by task verify")
	}
	database, err := url.Parse(databaseURL)
	require.NoError(t, err)
	schema := "request_media_test_" + uuid.NewString()[:8]
	query := database.Query()
	query.Set("search_path", schema)
	database.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(database.String()), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Exec("DROP SCHEMA " + pgx.Identifier{schema}.Sanitize() + " CASCADE").Error
		_ = sqlDB.Close()
	})
	require.NoError(t, db.Exec("CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE forms (LIKE public.forms INCLUDING ALL);
		CREATE TABLE form_schemas (LIKE public.form_schemas INCLUDING ALL);
		CREATE TABLE form_submissions (LIKE public.form_submissions INCLUDING ALL);
		CREATE TABLE service_tokens (LIKE public.service_tokens INCLUDING ALL);
		CREATE TABLE webhook_endpoints (LIKE public.webhook_endpoints INCLUDING ALL);
		CREATE TABLE webhook_deliveries (LIKE public.webhook_deliveries INCLUDING ALL);
		CREATE TABLE submission_export_audit (LIKE public.submission_export_audit INCLUDING ALL);
		CREATE TABLE management_audit (LIKE public.management_audit INCLUDING ALL);
	`).Error)
	return db
}

func TestEveryBodyOperationRejectsUnsupportedMediaBeforePostgresMutation(t *testing.T) {
	db := requestMediaDatabase(t)
	database := &boundaryDB{db: db}
	organizationID := uuid.NewString()
	tokens := tokenrepository.NewStore(database)
	caller, secret, err := auth.Issue(organizationID, auth.AllScopes(), time.Hour, time.Now())
	require.NoError(t, err)
	require.NoError(t, tokens.Save(t.Context(), caller, auth.DatabaseAuditActor("fixture", organizationID)))
	forms := formrepository.NewStore(database, nil)
	form := model.NewForm(organizationID, "Request media fixture", "", model.JSON{
		"$schema": model.JSONSchemaDraft202012URI, "type": "object",
	})
	form.Name = "request-media-" + uuid.NewString()[:8]
	require.NoError(t, forms.CreateForm(t.Context(), form))
	_, err = forms.PublishSchemaVersion(t.Context(), organizationID, form.ID, 1)
	require.NoError(t, err)
	router := echo.New()
	web.NewV1APIHandler(forms, tokens, nil).RegisterRoutes(router)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: 10 * time.Second}
	formID := form.ID
	read, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/v1/forms/"+formID, nil)
	require.NoError(t, err)
	read.Header.Set("Authorization", "Bearer "+secret)
	readResponse, err := client.Do(read)
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, readResponse.Body)
	require.NoError(t, readResponse.Body.Close())
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, readResponse.StatusCode)
	etag := readResponse.Header.Get("ETag")
	require.NotEmpty(t, etag)

	operations := []struct {
		name, method, path, expectedMedia string
	}{
		{"create form", http.MethodPost, "/v1/forms", "application/json"},
		{"update form", http.MethodPatch, "/v1/forms/" + formID, "application/merge-patch+json"},
		{"create schema version", http.MethodPost, "/v1/forms/" + formID + "/versions", "application/json"},
		{"export submissions", http.MethodPost, "/v1/forms/" + formID + "/submissions/export", "application/json"},
		{"put webhook", http.MethodPut, "/v1/forms/" + formID + "/webhook", "application/json"},
		{"patch webhook", http.MethodPatch, "/v1/forms/" + formID + "/webhook", "application/json"},
		{"create public submission", http.MethodPost, "/v1/public/forms/" + url.PathEscape(form.PublicKey) + "/submissions", "application/json"},
	}
	tables := []string{"forms", "form_schemas", "form_submissions", "service_tokens", "webhook_endpoints", "webhook_deliveries", "submission_export_audit", "management_audit"}
	counts := make(map[string]int64, len(tables))
	for _, table := range tables {
		var count int64
		require.NoError(t, db.Table(table).Count(&count).Error)
		counts[table] = count
	}

	for _, operation := range operations {
		for _, media := range []struct {
			name, contentType string
		}{
			{"missing", ""},
			{"wrong", map[bool]string{true: "application/json", false: "text/plain"}[operation.expectedMedia == "application/merge-patch+json"]},
		} {
			t.Run(operation.name+"/"+media.name, func(t *testing.T) {
				req, err := http.NewRequestWithContext(t.Context(), operation.method, server.URL+operation.path, bytes.NewBufferString(`{}`))
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+secret)
				req.Header.Set("If-Match", etag)
				req.Header.Set("Idempotency-Key", uuid.NewString())
				if media.contentType != "" {
					req.Header.Set("Content-Type", media.contentType)
				}
				response, err := client.Do(req)
				require.NoError(t, err)
				body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
				require.NoError(t, response.Body.Close())
				require.NoError(t, readErr)
				require.Equal(t, http.StatusUnsupportedMediaType, response.StatusCode, string(body))
				require.Contains(t, string(body), `"code":"unsupported_media_type"`)
			})
		}
	}
	for _, table := range tables {
		var after int64
		require.NoError(t, db.Table(table).Count(&after).Error)
		require.Equal(t, counts[table], after, "%s changed after rejected requests", table)
	}
}
