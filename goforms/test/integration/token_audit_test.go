package integration_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/goformx/goforms/internal/application/constants"
	"github.com/goformx/goforms/internal/application/handlers/web"
	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/infrastructure/authn"
	assertionreplay "github.com/goformx/goforms/internal/infrastructure/repository/assertionreplay"
	formrepository "github.com/goformx/goforms/internal/infrastructure/repository/form"
	tokenrepository "github.com/goformx/goforms/internal/infrastructure/repository/token"
	mocklogging "github.com/goformx/goforms/test/mocks/logging"
)

func tokenAuditDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PostgreSQL integration is run by task verify")
	}
	database, err := url.Parse(databaseURL)
	require.NoError(t, err)
	schema := "token_audit_test_" + uuid.NewString()[:8]
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
	// Isolate failure injection and retained audits from other suites while using
	// the real migrated columns, constraints, indexes and append-only function.
	require.NoError(t, db.Exec(`
		CREATE TABLE service_tokens (LIKE public.service_tokens INCLUDING ALL);
		CREATE TABLE management_audit (LIKE public.management_audit INCLUDING ALL);
		CREATE TABLE first_party_assertion_replays (LIKE public.first_party_assertion_replays INCLUDING ALL);
		CREATE TRIGGER audit_append_only BEFORE UPDATE OR DELETE ON management_audit
		FOR EACH ROW EXECUTE FUNCTION public.prevent_management_audit_mutation();
		CREATE TRIGGER audit_no_truncate BEFORE TRUNCATE ON management_audit
		FOR EACH STATEMENT EXECUTE FUNCTION public.prevent_management_audit_mutation();
	`).Error)
	return db
}

func TestTokenMutationsHaveAtomicActorAuditThroughRealHTTPAndPostgres(t *testing.T) {
	for _, credentialClass := range []auth.CredentialClass{auth.CredentialClassServiceToken, auth.CredentialClassFirstPartyAssertion} {
		t.Run(string(credentialClass), func(t *testing.T) {
			db := tokenAuditDatabase(t)
			database := &boundaryDB{db: db}
			organizationID := uuid.NewString()
			tokens := tokenrepository.NewStore(database)
			parent, parentSecret, err := auth.Issue(organizationID, []auth.Scope{auth.ScopeTokensWrite}, time.Hour, time.Now())
			require.NoError(t, err)
			require.NoError(t, tokens.Save(t.Context(), parent, auth.DatabaseAuditActor("fixture", organizationID)))
			publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
			require.NoError(t, err)
			keys, err := authn.NewJWKSProvider(authn.JWKSProviderConfig{Snapshot: fmt.Sprintf(
				`{"keys":[{"kty":"OKP","crv":"Ed25519","x":"%s","kid":"audit-key","use":"sig","alg":"EdDSA","state":"active"}]}`,
				base64.RawURLEncoding.EncodeToString(publicKey))})
			require.NoError(t, err)
			verifier, err := auth.NewFirstPartyVerifier("https://goformx.com", "https://api.goformx.com", keys, assertionreplay.NewStore(database))
			require.NoError(t, err)
			router := echo.New()
			logger := mocklogging.NewMockLogger(gomock.NewController(t))
			web.NewV1APIHandlerWithLimits(formrepository.NewStore(database, logger), tokens, nil,
				web.DefaultV1Limits(), verifier).RegisterRoutes(router)
			server := httptest.NewServer(router)
			t.Cleanup(server.Close)
			client := &http.Client{Timeout: 10 * time.Second}
			var lastActor auth.AuditActor
			credentialForScope := func(organization string, scope auth.Scope) string {
				if credentialClass == auth.CredentialClassServiceToken {
					caller, secret := parent, parentSecret
					if organization != organizationID || scope != auth.ScopeTokensWrite {
						caller, secret, err = auth.Issue(organization, []auth.Scope{scope}, time.Hour, time.Now())
						require.NoError(t, err)
						require.NoError(t, tokens.Save(t.Context(), caller, auth.DatabaseAuditActor("fixture", organization)))
					}
					lastActor = auth.AuditActor{OrganizationID: organization, SubjectID: caller.ID,
						CredentialClass: credentialClass, CredentialID: caller.ID}
					return secret
				}
				assertionID := uuid.NewString()
				bearer := signBoundaryAssertion(t, privateKey, "audit-key", organization, assertionID, scope, time.Now().UTC())
				payload, err := base64.RawURLEncoding.DecodeString(strings.Split(bearer, ".")[1])
				require.NoError(t, err)
				var claims struct {
					Subject string `json:"sub"`
					Request string `json:"rid"`
				}
				require.NoError(t, json.Unmarshal(payload, &claims))
				lastActor = auth.AuditActor{OrganizationID: organization, SubjectID: claims.Subject,
					CredentialClass: credentialClass, CredentialID: assertionID, RequestID: claims.Request}
				return bearer
			}
			credential := func(organization string) string {
				return credentialForScope(organization, auth.ScopeTokensWrite)
			}
			requestWithMedia := func(method, path, bearer string, body []byte, contentType *string, status int) []byte {
				req, err := http.NewRequestWithContext(t.Context(), method, server.URL+path, bytes.NewReader(body))
				require.NoError(t, err)
				req.Header.Set("Authorization", "Bearer "+bearer)
				if contentType != nil {
					req.Header.Set("Content-Type", *contentType)
				}
				if auth.IsFirstPartyAssertion(bearer) {
					req.Header.Set(constants.HeaderTraceID, uuid.NewString()) // signed rid must win
				} else {
					lastActor.CorrelationID = "caller-" + uuid.NewString()
					req.Header.Set(constants.HeaderTraceID, lastActor.CorrelationID)
				}
				callerTrace := req.Header.Get(constants.HeaderTraceID)
				req.Header.Set("X-Organization-ID", uuid.NewString()) // never audit authority
				response, err := client.Do(req)
				require.NoError(t, err)
				defer response.Body.Close()
				data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
				require.NoError(t, err)
				require.Equal(t, status, response.StatusCode, string(data))
				// Media-type/document rejection run before authentication, while
				// scope rejection happens before the verified principal is attached
				// to the request context. Once a principal is attached, its signed
				// identity must override the untrusted caller trace.
				prePrincipalRejection := status == http.StatusUnsupportedMediaType || status == http.StatusBadRequest || status == http.StatusForbidden
				if auth.IsFirstPartyAssertion(bearer) {
					if prePrincipalRejection {
						require.Contains(t, []string{callerTrace, lastActor.RequestID},
							response.Header.Get(constants.HeaderTraceID), "a rejecting boundary may run before or after principal attachment")
					} else {
						require.Equal(t, lastActor.RequestID, response.Header.Get(constants.HeaderTraceID))
					}
				} else {
					require.Equal(t, lastActor.CorrelationID, response.Header.Get(constants.HeaderTraceID))
				}
				require.NotContains(t, string(data), "audit-failure-canary")
				return data
			}
			request := func(method, path, bearer string, body []byte, status int) []byte {
				contentType := "application/json"
				return requestWithMedia(method, path, bearer, body, &contentType, status)
			}
			body := []byte(`{"name":"private-token-nickname","scopes":["tokens:write"],"expiresInSeconds":3600}`)
			denied := request(http.MethodPost, "/v1/service-tokens", credentialForScope(organizationID, auth.ScopeFormsRead),
				body, http.StatusForbidden)
			require.Contains(t, string(denied), `"code":"forbidden"`)
			var initialTokens, initialAudits int64
			require.NoError(t, db.Table("service_tokens").Count(&initialTokens).Error)
			require.NoError(t, db.Table("management_audit").Count(&initialAudits).Error)
			for _, rejection := range []struct {
				name, body string
				media      *string
				status     int
			}{
				{"missing content type", string(body), nil, http.StatusUnsupportedMediaType},
				{"wrong content type", string(body), pointerTo("text/plain"), http.StatusUnsupportedMediaType},
				{"duplicate member", `{"name":"first","name":"second","scopes":["tokens:write"]}`, pointerTo("application/json"), http.StatusBadRequest},
				{"escape-equivalent member", `{"name":"first","na\u006de":"second","scopes":["tokens:write"]}`, pointerTo("application/json"), http.StatusBadRequest},
				{"nested duplicate", `{"name":"first","scopes":["tokens:write"],"extra":{"x":1,"x":2}}`, pointerTo("application/json"), http.StatusBadRequest},
				{"trailing document", string(body) + ` {}`, pointerTo("application/json"), http.StatusBadRequest},
			} {
				t.Run(rejection.name, func(t *testing.T) {
					response := requestWithMedia(http.MethodPost, "/v1/service-tokens", credential(organizationID),
						[]byte(rejection.body), rejection.media, rejection.status)
					if rejection.status == http.StatusUnsupportedMediaType {
						require.Contains(t, string(response), `"code":"unsupported_media_type"`)
					} else {
						require.Contains(t, string(response), `"code":"invalid_request"`)
					}
				})
			}
			var rejectedTokens, rejectedAudits int64
			require.NoError(t, db.Table("service_tokens").Count(&rejectedTokens).Error)
			require.NoError(t, db.Table("management_audit").Count(&rejectedAudits).Error)
			require.Equal(t, initialTokens, rejectedTokens, "rejected request bodies cannot issue credentials")
			require.Equal(t, initialAudits, rejectedAudits, "rejected request bodies cannot append mutation audits")
			created := request(http.MethodPost, "/v1/service-tokens", credential(organizationID), body, http.StatusCreated)
			createdActor := lastActor
			var envelope struct {
				Data struct {
					Token    string `json:"token"`
					Metadata struct {
						ID string `json:"id"`
					} `json:"metadata"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(created, &envelope))
			tokenID, tokenSecret := envelope.Data.Metadata.ID, envelope.Data.Token
			require.Equal(t, tokenID, auth.LookupID(tokenSecret))
			var row struct{ AuditID, SubjectID, CredentialClass, CredentialID, RequestID, CorrelationID, OrganizationID, Event string }
			require.NoError(t, db.Table("management_audit").Where("target_id = ? AND event = ?", tokenID, "service_token.created").Take(&row).Error)
			require.NoError(t, uuid.Validate(row.AuditID))
			require.Equal(t, createdActor.SubjectID, row.SubjectID)
			require.Equal(t, string(createdActor.CredentialClass), row.CredentialClass)
			require.Equal(t, createdActor.CredentialID, row.CredentialID)
			if credentialClass == auth.CredentialClassFirstPartyAssertion {
				require.Equal(t, createdActor.RequestID, row.RequestID)
				require.Empty(t, row.CorrelationID)
			} else {
				require.NoError(t, uuid.Validate(row.RequestID))
				require.NotEqual(t, createdActor.CorrelationID, row.RequestID)
				require.Equal(t, createdActor.CorrelationID, row.CorrelationID)
			}
			require.Equal(t, organizationID, row.OrganizationID)
			require.Equal(t, "service_token.created", row.Event)
			var storedAudit string
			require.NoError(t, db.Raw("SELECT row_to_json(a)::text FROM management_audit a WHERE target_id = ?", tokenID).Scan(&storedAudit).Error)
			for _, forbidden := range []string{tokenSecret, parentSecret, "private-token-nickname", "token_hash"} {
				require.NotContains(t, storedAudit, forbidden)
			}
			var tokenCount int64
			require.NoError(t, db.Table("service_tokens").Count(&tokenCount).Error)
			require.NoError(t, db.Exec(`CREATE FUNCTION reject_audit() RETURNS trigger LANGUAGE plpgsql AS $$
				BEGIN RAISE EXCEPTION 'audit-failure-canary'; END $$;
				CREATE TRIGGER reject_audit BEFORE INSERT ON management_audit FOR EACH ROW EXECUTE FUNCTION reject_audit();`).Error)
			failed := request(http.MethodPost, "/v1/service-tokens", credential(organizationID), body, http.StatusServiceUnavailable)
			require.NotContains(t, string(failed), "gfst_")
			var afterFailure int64
			require.NoError(t, db.Table("service_tokens").Count(&afterFailure).Error)
			require.Equal(t, tokenCount, afterFailure)
			path := "/v1/service-tokens/" + tokenID
			request(http.MethodDelete, path, credential(organizationID), nil, http.StatusServiceUnavailable)
			active, err := tokens.FindByID(t.Context(), tokenID)
			require.NoError(t, err)
			require.NoError(t, active.Authenticate(tokenSecret, organizationID, time.Now()))
			require.NoError(t, db.Exec("DROP TRIGGER reject_audit ON management_audit").Error)
			request(http.MethodDelete, path, credential(organizationID), nil, http.StatusNoContent)
			request(http.MethodDelete, path, credential(organizationID), nil, http.StatusNoContent)
			var audits int64
			require.NoError(t, db.Table("management_audit").Where("target_id = ?", tokenID).Count(&audits).Error)
			require.EqualValues(t, 2, audits, "one creation and one actual revocation, not one audit per retry")
			request(http.MethodDelete, "/v1/service-tokens/nonexistent", tokenSecret, nil, http.StatusUnauthorized)
			request(http.MethodDelete, path, credential(uuid.NewString()), nil, http.StatusNotFound)
			for _, query := range []string{"UPDATE management_audit SET request_id = 'changed'", "DELETE FROM management_audit", "TRUNCATE management_audit"} {
				require.Error(t, db.Exec(query).Error)
			}
			require.NoError(t, db.Exec("DELETE FROM service_tokens").Error)
			require.NoError(t, db.Table("management_audit").Where("target_id = ?", tokenID).Count(&audits).Error)
			require.EqualValues(t, 2, audits, "audit history survives credential deletion")
			if credentialClass == auth.CredentialClassServiceToken {
				correlationDown, readErr := os.ReadFile("../../migrations/postgresql/2026090102_management_audit_correlation.down.sql")
				require.NoError(t, readErr)
				require.ErrorContains(t, db.Exec(string(correlationDown)).Error,
					"cannot remove retained management audit correlation data")
			}
			down, err := os.ReadFile("../../migrations/postgresql/2026083003_management_audit.down.sql")
			require.NoError(t, err)
			require.ErrorContains(t, db.Exec(string(down)).Error, "cannot roll back a populated management audit")
			require.NoError(t, db.Table("management_audit").Where("target_id = ?", tokenID).Count(&audits).Error)
			require.EqualValues(t, 2, audits)
		})
	}
}

func pointerTo[T any](value T) *T { return &value }
