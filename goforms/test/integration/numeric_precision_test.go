package integration_test

import (
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
	formrepository "github.com/goformx/goforms/internal/infrastructure/repository/form"
	tokenrepository "github.com/goformx/goforms/internal/infrastructure/repository/token"
	mocklogging "github.com/goformx/goforms/test/mocks/logging"
)

func TestNumericPrecisionThroughHTTPAndPostgres(t *testing.T) {
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PostgreSQL integration is run by task verify")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	require.NoError(t, err)
	database := &boundaryDB{db: db}
	organization := uuid.NewString()
	tokens := tokenrepository.NewStore(database)
	token, credential, err := auth.Issue(organization, []auth.Scope{auth.ScopeFormsRead, auth.ScopeFormsWrite, auth.ScopeFormsPublish, auth.ScopeSubmissionsRead}, time.Hour, time.Now())
	require.NoError(t, err)
	require.NoError(t, tokens.Save(t.Context(), token, auth.DatabaseAuditActor("integration-fixture", token.OwnerID)))
	t.Cleanup(func() {
		require.NoError(t, db.Exec("DELETE FROM forms WHERE organization_id = ?", organization).Error)
		require.NoError(t, db.Exec("DELETE FROM service_tokens WHERE organization_id = ?", organization).Error)
	})
	router := echo.New()
	web.NewV1APIHandler(formrepository.NewStore(database, mocklogging.NewMockLogger(gomock.NewController(t))), tokens, nil).RegisterRoutes(router)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: 10 * time.Second}
	request := func(method, path, body, key string, expected int) []byte {
		req, err := http.NewRequestWithContext(t.Context(), method, server.URL+path, strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if !strings.HasPrefix(path, "/v1/public/") {
			req.Header.Set("Authorization", "Bearer "+credential)
		}
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		response, err := client.Do(req)
		require.NoError(t, err)
		defer response.Body.Close()
		result, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		require.NoError(t, err)
		require.Equal(t, expected, response.StatusCode, "synthetic request %s %s: %s", method, path, result)
		return result
	}
	const schema = `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{"integer":{"type":"integer","minimum":9007199254740993},"decimal":{"type":"number","minimum":0.1234567890123456789},"large":{},"tiny":{},"nested":{}},"required":["integer","decimal"]}`
	firstSchema := strings.Replace(schema, "9007199254740993", "9007199254740991", 1)
	created := request("POST", "/v1/forms", `{"name":"exact-numbers","title":"Exact numbers","schema":`+firstSchema+`}`, "", 201)
	var form struct {
		Data struct {
			ID        string `json:"id"`
			PublicKey string `json:"publicKey"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created, &form))
	path := "/v1/forms/" + form.Data.ID
	version := request("POST", path+"/versions", `{"schema":`+schema+`}`, "", 201)
	require.Contains(t, string(version), "9007199254740993")
	require.Contains(t, string(version), "0.1234567890123456789")
	request("POST", path+"/versions/2/publish", "", "", 200)
	original := request("GET", path+"/versions/1", "", "", 200)
	require.Contains(t, string(original), "9007199254740991")
	publicPath := "/v1/public/forms/" + form.Data.PublicKey
	publicSchema := request("GET", publicPath+"/schema", "", "", 200)
	require.Contains(t, string(publicSchema), "9007199254740993")
	const payload = `{"data":{"integer":9007199254740993,"decimal":0.1234567890123456789,"large":1e1023,"tiny":1e-1024,"nested":[{"negative":-9007199254740993}]}}`
	request("POST", publicPath+"/submissions", strings.Replace(payload, "9007199254740993", "9007199254740992", 1), uuid.NewString(), 422)
	request("POST", publicPath+"/submissions", strings.Replace(payload, "0.1234567890123456789", "0.1234567890123456788", 1), uuid.NewString(), 422)
	request("POST", publicPath+"/submissions", strings.Replace(payload, "1e1023", "1e1000000000", 1), uuid.NewString(), 400)
	request("POST", publicPath+"/submissions", `{"data":{"integer":9007199254740993,"decimal":0.1234567890123456789},"data":{}}`, uuid.NewString(), 400)
	request("POST", publicPath+"/submissions", `{"data":{"integer":9007199254740993,"decimal":0.1234567890123456789,"nested":{"value":1,"value":2}}}`, uuid.NewString(), 400)
	key := uuid.NewString()
	accepted := request("POST", publicPath+"/submissions", payload, key, 202)
	var submission struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(accepted, &submission))
	retry := request("POST", publicPath+"/submissions", payload, key, 202)
	require.Contains(t, string(retry), submission.Data.ID)
	// Different precise values remain different even though both used to round identically.
	request("POST", publicPath+"/submissions", strings.Replace(payload, "0.1234567890123456789", "0.1234567890123456790", 1), key, 409)
	for _, route := range []string{path + "/submissions", path + "/submissions/" + submission.Data.ID} {
		readback := request("GET", route, "", "", 200)
		require.Contains(t, string(readback), "9007199254740993")
		require.Contains(t, string(readback), "0.1234567890123456789")
	}
	var persisted model.JSON
	require.NoError(t, db.Raw("SELECT data FROM form_submissions WHERE uuid = ?", submission.Data.ID).Row().Scan(&persisted))
	var persistedText string
	require.NoError(t, db.Raw("SELECT data::text FROM form_submissions WHERE uuid = ?", submission.Data.ID).Row().Scan(&persistedText))
	require.Contains(t, persistedText, `"large":1e1023`)
	require.Contains(t, persistedText, `"tiny":1e-1024`)
	for field, expected := range map[string]string{"large": "1e1023", "tiny": "1e-1024"} {
		actual, ok := new(big.Rat).SetString(string(persisted[field].(json.Number)))
		require.True(t, ok)
		want, ok := new(big.Rat).SetString(expected)
		require.True(t, ok)
		require.Zero(t, actual.Cmp(want), "submission JSON must preserve the value")
	}
	var count int64
	require.NoError(t, db.Table("form_submissions").Where("form_id = ?", form.Data.ID).Count(&count).Error)
	require.EqualValues(t, 1, count)
	// Schema JSONB normalization must remain value-compatible with accepted
	// submission JSON, including exponent expansion and signed zero.
	for _, number := range []string{"0.00e-1022", "-0.00e-1022", "1.00e-1022", "1e1023", "1e-1024", "0e1024"} {
		t.Run("jsonb_normalization_"+number, func(t *testing.T) {
			var input, normalized model.JSON
			encoded := []byte(`{"value":` + number + `}`)
			require.NoError(t, json.Unmarshal(encoded, &input))
			require.NoError(t, db.Raw("SELECT CAST(? AS jsonb)", string(encoded)).Row().Scan(&normalized))
			require.True(t, model.EqualJSON(input, normalized))
		})
	}
}
