package integration_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/goformx/goforms/internal/application/handlers/web"
	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/submission"
	formrepository "github.com/goformx/goforms/internal/infrastructure/repository/form"
	tokenrepository "github.com/goformx/goforms/internal/infrastructure/repository/token"
	mocklogging "github.com/goformx/goforms/test/mocks/logging"
)

func TestSubmissionReadFiltersThroughHTTPAndPostgres(t *testing.T) {
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PostgreSQL integration is run by task verify")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	database := &boundaryDB{db: db}
	owner, foreign := uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		require.NoError(t, db.Exec("DELETE FROM forms WHERE organization_id IN (?, ?)", owner, foreign).Error)
		require.NoError(t, db.Exec("DELETE FROM service_tokens WHERE organization_id IN (?, ?)", owner, foreign).Error)
	})
	logger := mocklogging.NewMockLogger(gomock.NewController(t))
	logger.EXPECT().Debug("form not found in database", "id_length", 36, "error_type", "not_found").AnyTimes()
	store := formrepository.NewStore(database, logger)
	tokens := tokenrepository.NewStore(database)
	issue := func(organization string, scope auth.Scope) string {
		token, credential, err := auth.Issue(organization, []auth.Scope{scope}, time.Hour, time.Now())
		require.NoError(t, err)
		require.NoError(t, tokens.Save(t.Context(), token))
		return credential
	}
	credential := issue(owner, auth.ScopeSubmissionsRead)
	foreignCredential := issue(foreign, auth.ScopeSubmissionsRead)
	formOnlyCredential := issue(owner, auth.ScopeFormsRead)
	makeForm := func(organization string) *model.Form {
		definition := model.JSON{"$schema": model.JSONSchemaDraft202012URI, "type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}}
		form := model.NewForm(organization, "Submission read fixture", "", definition)
		form.Name = "reads-" + uuid.NewString()
		require.NoError(t, store.CreateForm(t.Context(), form))
		_, err := store.PublishSchemaVersion(t.Context(), organization, form.ID, 1)
		require.NoError(t, err)
		_, err = store.CreateSchemaVersion(t.Context(), organization, form.ID, definition)
		require.NoError(t, err)
		_, err = store.PublishSchemaVersion(t.Context(), organization, form.ID, 2)
		require.NoError(t, err)
		return form
	}
	form, otherForm, foreignForm := makeForm(owner), makeForm(owner), makeForm(foreign)
	start := time.Date(2026, 8, 1, 12, 0, 0, 123456000, time.UTC)
	end := start.Add(time.Hour)
	insert := func(target *model.Form, version int, received time.Time) *model.FormSubmission {
		row, replay, err := store.CreateSubmissionIdempotent(t.Context(), &model.FormSubmission{
			FormID: target.ID, SchemaVersion: version, Data: model.JSON{"name": "synthetic-private-value"},
			IdempotencyKey: uuid.NewString(), SubmittedAt: received, Status: model.SubmissionStatusAccepted,
		})
		require.NoError(t, err)
		require.False(t, replay)
		return row
	}
	wanted := []string{insert(form, 2, start).ID, insert(form, 2, start).ID}
	sort.Sort(sort.Reverse(sort.StringSlice(wanted)))
	insert(form, 1, start)
	insert(form, 2, start.Add(-time.Microsecond))
	insert(form, 2, end)
	otherRow, foreignRow := insert(otherForm, 2, start), insert(foreignForm, 2, start)
	router := echo.New()
	web.NewV1APIHandler(store, tokens, nil).RegisterRoutes(router)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: 10 * time.Second}
	get := func(path, bearer string, status int) []byte {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+path, nil)
		require.NoError(t, err)
		if bearer != "" {
			request.Header.Set("Authorization", "Bearer "+bearer)
		}
		response, err := client.Do(request)
		require.NoError(t, err)
		defer response.Body.Close()
		body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		require.NoError(t, err)
		require.Equal(t, status, response.StatusCode)
		if status >= 400 {
			require.NotContains(t, string(body), "synthetic-private-value")
		}
		return body
	}
	path := "/v1/forms/" + form.ID + "/submissions"
	filters := url.Values{"limit": {"1"}, "status": {"accepted"}, "schemaVersion": {"2"},
		"receivedFrom": {start.Format(time.RFC3339Nano)}, "receivedBefore": {end.Format(time.RFC3339Nano)}}
	type page struct {
		Data []struct {
			ID            string `json:"id"`
			SchemaVersion int    `json:"schemaVersion"`
			SubmittedAt   string `json:"submittedAt"`
		} `json:"data"`
		Meta struct {
			NextCursor *string `json:"nextCursor"`
		} `json:"meta"`
	}
	var first, second page
	require.NoError(t, json.Unmarshal(get(path+"?"+filters.Encode(), credential, 200), &first))
	require.Len(t, first.Data, 1)
	require.Equal(t, wanted[0], first.Data[0].ID)
	require.Equal(t, 2, first.Data[0].SchemaVersion)
	require.Equal(t, start.Format(time.RFC3339Nano), first.Data[0].SubmittedAt)
	require.NotNil(t, first.Meta.NextCursor)
	filters.Set("cursor", *first.Meta.NextCursor)
	require.NoError(t, json.Unmarshal(get(path+"?"+filters.Encode(), credential, 200), &second))
	require.Len(t, second.Data, 1)
	require.Equal(t, wanted[1], second.Data[0].ID)
	require.Nil(t, second.Meta.NextCursor)
	get(path+"?"+filters.Encode(), foreignCredential, 404)
	get(path, formOnlyCredential, 403)
	get(path, "", 401)
	get(path+"/"+wanted[0], credential, 200)
	get(path+"/"+wanted[0], foreignCredential, 404)
	get(path+"/"+otherRow.ID, credential, 404)
	get(path+"/"+foreignRow.ID, credential, 404)
	get(path+"/"+uuid.NewString(), credential, 404)
	get(path+"?status=delivered", credential, 400)
	get(path+"?schemaVersion=2&schemaVersion=1", credential, 400)
	get(path+"?receivedBefore=private-synthetic-text", credential, 400)
	filters.Del("cursor")
	filters.Set("schemaVersion", "3")
	var empty page
	require.NoError(t, json.Unmarshal(get(path+"?"+filters.Encode(), credential, 200), &empty))
	require.Empty(t, empty.Data)
	require.Nil(t, empty.Meta.NextCursor)
	// A known form identifier or retained cursor cannot bypass soft deletion.
	require.NoError(t, db.Model(&model.Form{}).Where("uuid = ?", form.ID).Update("deleted_at", time.Now()).Error)
	get(path, credential, 404)
	get(path+"/"+wanted[0], credential, 404)
	rows, more, err := store.ListSubmissionsPage(t.Context(), owner, form.ID, submission.ListOptions{Limit: 1})
	require.NoError(t, err)
	require.Empty(t, rows)
	require.False(t, more)
	// Both documented indexes must exist, be valid, and use the required key order.
	for name, keys := range map[string]string{
		"form_submissions_form_time_id_idx":         "(form_id, submitted_at DESC, uuid DESC)",
		"form_submissions_form_version_time_id_idx": "(form_id, schema_version, submitted_at DESC, uuid DESC)",
	} {
		var definition string
		require.NoError(t, db.Raw("SELECT pg_get_indexdef(indexrelid) FROM pg_index WHERE indexrelid = to_regclass(?) AND indisvalid", name).Row().Scan(&definition))
		require.Contains(t, definition, keys)
	}
}
