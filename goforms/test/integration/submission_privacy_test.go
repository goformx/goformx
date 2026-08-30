package integration_test

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"gorm.io/gorm"

	"github.com/goformx/goforms/internal/application/handlers/web"
	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/submission"
	"github.com/goformx/goforms/internal/infrastructure/config"
	databasepackage "github.com/goformx/goforms/internal/infrastructure/database"
	"github.com/goformx/goforms/internal/infrastructure/logging"
	formrepository "github.com/goformx/goforms/internal/infrastructure/repository/form"
	tokenrepository "github.com/goformx/goforms/internal/infrastructure/repository/token"
)

func TestSubmissionPrivacyUsesAcceptedVersionThroughHTTPAndPostgres(t *testing.T) {
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PostgreSQL integration is run by task verify")
	}
	parsed, err := url.Parse(databaseURL)
	require.NoError(t, err)
	require.NotNil(t, parsed.User)
	port, err := strconv.Atoi(parsed.Port())
	require.NoError(t, err)
	password, _ := parsed.User.Password()
	cfg := &config.Config{Database: config.DatabaseConfig{Driver: "postgres", Host: parsed.Hostname(), Port: port,
		Name: strings.TrimPrefix(parsed.Path, "/"), Username: parsed.User.Username(), Password: password,
		SSLMode: parsed.Query().Get("sslmode"), MaxOpenConns: 4, MaxIdleConns: 2,
		Logging: config.DatabaseLoggingConfig{LogLevel: "info"}}}
	core, observed := observer.New(zapcore.DebugLevel)
	factory, err := logging.NewFactory(&logging.FactoryConfig{AppName: "privacy-test", Environment: "production", LogLevel: "debug"}, nil)
	require.NoError(t, err)
	logger, err := factory.WithTestCore(core).CreateLogger()
	require.NoError(t, err)
	// Use the real runtime connection/logger composition, not a quiet test DB.
	database, err := databasepackage.New(cfg, logger)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	db := database.GetDB()
	organization, foreign := uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		require.NoError(t, db.Exec("DELETE FROM forms WHERE organization_id = ?", organization).Error)
		require.NoError(t, db.Exec("DELETE FROM service_tokens WHERE organization_id IN (?, ?)", organization, foreign).Error)
	})
	tokens := tokenrepository.NewStore(database)
	issue := func(owner string, scopes ...auth.Scope) string {
		token, credential, err := auth.Issue(owner, scopes, time.Hour, time.Now())
		require.NoError(t, err)
		require.NoError(t, tokens.Save(t.Context(), token))
		return credential
	}
	credential := issue(organization, auth.ScopeFormsRead, auth.ScopeFormsWrite, auth.ScopeFormsPublish, auth.ScopeSubmissionsRead)
	foreignCredential := issue(foreign, auth.ScopeSubmissionsRead)
	formOnlyCredential := issue(organization, auth.ScopeFormsRead)
	router := echo.New()
	web.NewV1APIHandler(formrepository.NewStore(database, logger), tokens, logger).RegisterRoutes(router)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: 10 * time.Second}
	request := func(method, path, body, bearer, key string, expected int) []byte {
		req, err := http.NewRequestWithContext(t.Context(), method, server.URL+path, strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		response, err := client.Do(req)
		require.NoError(t, err)
		defer response.Body.Close()
		encoded, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		require.NoError(t, err)
		require.Equal(t, expected, response.StatusCode, "synthetic %s %s", method, path)
		require.Equal(t, "no-store", response.Header.Get("Cache-Control"))
		if strings.HasSuffix(path, "/submissions/export") {
			if expected == http.StatusOK {
				require.NotEmpty(t, response.Header.Get("X-GoFormX-Export-ID"))
				require.Regexp(t, `^attachment; filename="goformx-submissions-[0-9a-f-]+\.(json|csv)"$`, response.Header.Get("Content-Disposition"))
			} else {
				require.Empty(t, response.Header.Get("Content-Disposition"))
				require.Empty(t, response.Header.Get("X-GoFormX-Export-ID"))
			}
		}
		require.NotContains(t, string(encoded), "private-canary")
		return encoded
	}
	const schema = `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{"name":{"type":"string"},"secret":{"type":"string","format":"email"},"nested":{"type":"object"},"items":{"type":"array","items":{"type":"string"}}},"x-goformx-sensitive":["/secret","/nested/token","/items/0","/absent"]}`
	created := request("POST", "/v1/forms", `{"name":"private-submissions","title":"Private submissions","schema":`+schema+`}`, credential, "", 201)
	var form struct {
		Data struct {
			ID        string `json:"id"`
			PublicKey string `json:"publicKey"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(created, &form))
	path := "/v1/forms/" + form.Data.ID
	publicPath := "/v1/public/forms/" + form.Data.PublicKey + "/submissions"
	request("POST", path+"/versions/1/publish", "", credential, "", 200)
	const body = `{"data":{"name":"visible-name","secret":"private-canary@example.test","nested":{"token":"private-canary"},"items":["private-canary","visible-item"]}}`
	key := uuid.NewString()
	first := request("POST", publicPath, body, "", key, 202)
	type detail struct {
		Data struct {
			ID            string     `json:"id"`
			SchemaVersion int        `json:"schemaVersion"`
			Data          model.JSON `json:"data"`
			Schema        model.JSON `json:"schema"`
			RedactedPaths []string   `json:"redactedPaths"`
		} `json:"data"`
	}
	var one detail
	require.NoError(t, json.Unmarshal(first, &one))
	require.Equal(t, "visible-name", one.Data.Data["name"])
	require.Equal(t, []any{nil, "visible-item"}, one.Data.Data["items"])
	require.ElementsMatch(t, []string{"/secret", "/nested/token", "/items/0", "/absent"}, one.Data.RedactedPaths)
	retry := request("POST", publicPath, body, "", key, 202)
	require.Contains(t, string(retry), one.Data.ID)
	// Detailed validator diagnostics must not echo a sensitive invalid instance.
	request("POST", publicPath, strings.Replace(body, "private-canary@example.test", "private-canary-invalid-email", 1), "", uuid.NewString(), 422)
	// Decoder diagnostics can echo attacker-supplied property names too.
	request("POST", publicPath, `{"private-canary-envelope":"secret","data":{}}`, "", uuid.NewString(), 400)
	request("POST", publicPath, `{"data":null}`, "", uuid.NewString(), 400)
	request("POST", publicPath, `{}`, "", uuid.NewString(), 400)
	newSchema := strings.Replace(schema, `"/absent"]`, `"/absent","/name"]`, 1)
	request("POST", path+"/versions", `{"schema":`+newSchema+`}`, credential, "", 201)
	request("POST", path+"/versions/2/publish", "", credential, "", 200)
	var two detail
	require.NoError(t, json.Unmarshal(request("POST", publicPath, body, "", uuid.NewString(), 202), &two))
	require.NotContains(t, two.Data.Data, "name")
	require.Equal(t, 2, two.Data.SchemaVersion)
	var old detail
	require.NoError(t, json.Unmarshal(request("GET", path+"/submissions/"+one.Data.ID+"?reveal=true", "", credential, "", 200), &old))
	require.Equal(t, 1, old.Data.SchemaVersion)
	require.Equal(t, "visible-name", old.Data.Data["name"], "New policy cannot reinterpret old acceptance")
	var expectedSchema model.JSON
	require.NoError(t, json.Unmarshal([]byte(schema), &expectedSchema))
	require.True(t, model.EqualJSON(expectedSchema, old.Data.Schema))
	request("GET", path+"/submissions", "", credential, "", 200)
	request("GET", path+"/submissions/"+one.Data.ID, "", foreignCredential, "", 404)
	request("GET", path+"/submissions/"+one.Data.ID, "", formOnlyCredential, "", 403)
	request("GET", path+"/submissions/"+one.Data.ID, "", "", "", 401)
	// Both export encodings use the accepted version, with a durable audit before
	// any attachment headers or bytes are released.
	exportPath := path + "/submissions/export"
	jsonExport := request("POST", exportPath, `{"format":"json"}`, credential, "", 200)
	var exported struct {
		Data []submission.Projection `json:"data"`
		Meta submission.ExportMeta   `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(jsonExport, &exported))
	require.Len(t, exported.Data, 2)
	require.Equal(t, 2, exported.Meta.RowCount)
	byID := map[string]submission.Projection{}
	for _, row := range exported.Data {
		byID[row.ID] = row
	}
	require.Equal(t, "visible-name", byID[one.Data.ID].Data["name"])
	require.NotContains(t, byID[two.Data.ID].Data, "name")
	var audit struct {
		OrganizationID  string
		SubjectID       string
		CredentialClass string
		CredentialID    string
		Format          string
		RowCount        int
		ByteCount       int
	}
	require.NoError(t, db.Table("submission_export_audit").Where("export_id = ?", exported.Meta.ExportID).Take(&audit).Error)
	require.Equal(t, organization, audit.OrganizationID)
	require.Equal(t, "service_token", audit.CredentialClass)
	require.Equal(t, auth.LookupID(credential), audit.CredentialID)
	require.Equal(t, audit.CredentialID, audit.SubjectID)
	require.Equal(t, "json", audit.Format)
	require.Equal(t, 2, audit.RowCount)
	require.Equal(t, len(jsonExport), audit.ByteCount)
	csvExport := request("POST", exportPath, `{"format":"csv","schemaVersion":1}`, credential, "", 200)
	csvRows, err := csv.NewReader(strings.NewReader(string(csvExport))).ReadAll()
	require.NoError(t, err)
	require.Len(t, csvRows, 2, "One version-one row plus header")
	require.Contains(t, csvRows[1], "'"+one.Data.ID)
	require.Contains(t, csvRows[1], "'visible-name")
	request("POST", exportPath, `{"format":"json"}`, foreignCredential, "", 404)
	request("POST", exportPath, `{"format":"json"}`, formOnlyCredential, "", 403)
	request("POST", exportPath, `{"format":"json"}`, "", "", 401)
	request("POST", exportPath, `{"format":"json","payload":"private-canary"}`, credential, "", 400)
	request("POST", exportPath, `{"format":"json","format":"csv"}`, credential, "", 400)
	var auditCount int64
	require.NoError(t, db.Table("submission_export_audit").Where("organization_id = ?", organization).Count(&auditCount).Error)
	require.EqualValues(t, 2, auditCount)
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register("privacy_test:audit_failure", func(tx *gorm.DB) {
		if strings.Contains(tx.Statement.SQL.String(), "INSERT INTO submission_export_audit") {
			failure := tx.Session(&gorm.Session{NewDB: true}).Exec("DO $$ BEGIN RAISE EXCEPTION 'private-canary-audit-failure'; END $$").Error
			_ = tx.AddError(failure)
		}
	}))
	request("POST", exportPath, `{"format":"csv"}`, credential, "", 503)
	require.NoError(t, db.Table("submission_export_audit").Where("organization_id = ?", organization).Count(&auditCount).Error)
	require.EqualValues(t, 2, auditCount, "Failed and denied exports do not create prepared audit records")
	request("POST", path+"/versions", `{"schema":`+strings.Replace(schema, `"/secret"`, `"#/secret"`, 1)+`}`, credential, "", 422)
	// Syntactically valid policy that cannot traverse this instance fails before persistence.
	badShape := strings.Replace(schema, `"/absent"]`, `"/name/child"]`, 1)
	request("POST", path+"/versions", `{"schema":`+badShape+`}`, credential, "", 201)
	request("POST", path+"/versions/3/publish", "", credential, "", 200)
	request("POST", publicPath, body, "", uuid.NewString(), 422)
	// Instance locations inside a sensitive object can themselves contain secrets.
	const keyedSchema = `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{"nested":{"type":"object","additionalProperties":{"type":"integer"}}},"x-goformx-sensitive":["/nested"]}`
	request("POST", path+"/versions", `{"schema":`+keyedSchema+`}`, credential, "", 201)
	request("POST", path+"/versions/4/publish", "", credential, "", 200)
	request("POST", publicPath, `{"data":{"nested":{"private-canary-property":"not an integer"}}}`, "", uuid.NewString(), 422)
	var persisted model.JSON
	require.NoError(t, db.Raw("SELECT data FROM form_submissions WHERE uuid = ?", one.Data.ID).Row().Scan(&persisted))
	require.Equal(t, "private-canary@example.test", persisted["secret"], "Redaction must not rewrite immutable storage")
	var count int64
	require.NoError(t, db.Table("form_submissions").Where("form_id = ?", form.Data.ID).Count(&count).Error)
	require.EqualValues(t, 2, count)
	// Inject a real PostgreSQL failure in this connection's submission insert
	// callback. No schema triggers or shared database state are changed.
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("privacy_test:driver_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "form_submissions" {
			failure := tx.Session(&gorm.Session{NewDB: true}).Exec("DO $$ BEGIN RAISE EXCEPTION 'private-canary-database-failure'; END $$").Error
			_ = tx.AddError(failure)
		}
	}))
	request("POST", publicPath, `{"data":{"nested":{},"note":"private-canary-payload"}}`, "", uuid.NewString(), 500)
	require.NoError(t, db.Table("form_submissions").Where("form_id = ?", form.Data.ID).Count(&count).Error)
	require.EqualValues(t, 2, count)
	require.NotEmpty(t, observed.FilterMessage("database operation failed").All())
	require.NotEmpty(t, observed.FilterMessage("v1 API repository failure").All())
	encodedLogs, err := json.Marshal(observed.All())
	require.NoError(t, err)
	logs := string(encodedLogs)
	require.NotContains(t, logs, "private-canary")
	require.NotContains(t, logs, credential)
	require.NotContains(t, logs, "visible-name")
}
