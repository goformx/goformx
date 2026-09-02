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
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/goformx/goforms/internal/application/constants"
	"github.com/goformx/goforms/internal/application/handlers/web"
	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/managementaudit"
	domainwebhook "github.com/goformx/goforms/internal/domain/webhook"
	"github.com/goformx/goforms/internal/infrastructure/authn"
	assertionreplay "github.com/goformx/goforms/internal/infrastructure/repository/assertionreplay"
	formrepository "github.com/goformx/goforms/internal/infrastructure/repository/form"
	tokenrepository "github.com/goformx/goforms/internal/infrastructure/repository/token"
	mocklogging "github.com/goformx/goforms/test/mocks/logging"
)

func TestWebhookLifecycleAtomicityThroughRealHTTPAndPostgres(t *testing.T) {
	for _, credentialClass := range []auth.CredentialClass{auth.CredentialClassServiceToken, auth.CredentialClassFirstPartyAssertion} {
		t.Run(string(credentialClass), func(t *testing.T) {
			db := tokenAuditDatabase(t)
			// Isolate fault injection while retaining real migrated types, checks,
			// indexes and webhook FK/updated-at trigger definitions.
			require.NoError(t, db.Exec(`CREATE TABLE forms (LIKE public.forms INCLUDING ALL);
				CREATE TABLE form_schemas (LIKE public.form_schemas INCLUDING ALL);
				CREATE TABLE form_submissions (LIKE public.form_submissions INCLUDING ALL);
				CREATE FUNCTION update_updated_at_column() RETURNS TRIGGER AS $$
				BEGIN NEW.updated_at = NOW(); RETURN NEW; END; $$ LANGUAGE plpgsql;`).Error)
			outboxMigration, err := os.ReadFile("../../migrations/postgresql/2026082802_add_webhook_outbox.up.sql")
			require.NoError(t, err)
			require.NoError(t, db.Exec(string(outboxMigration)).Error)
			database := &boundaryDB{db: db}
			organizationID, foreignID := uuid.NewString(), uuid.NewString()
			actor := auth.DatabaseAuditActor("fixture", organizationID)
			cipher, err := domainwebhook.NewKeyring("active", map[string]string{"active": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))}, "")
			require.NoError(t, err)
			logger := mocklogging.NewMockLogger(gomock.NewController(t))
			logger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			forms := formrepository.NewStoreWithOptions(database, logger, formrepository.StoreOptions{WebhookCipher: cipher})
			form := model.NewForm(organizationID, "Webhook lifecycle", "", model.JSON{"$schema": model.JSONSchemaDraft202012URI, "type": "object"})
			form.Name = "lifecycle-" + uuid.NewString()[:8]
			require.NoError(t, forms.CreateForm(t.Context(), form))
			tokens := tokenrepository.NewStore(database)
			serviceSecrets := map[string]string{}
			for _, org := range []string{organizationID, foreignID} {
				token, secret, issueErr := auth.Issue(org, []auth.Scope{auth.ScopeWebhooksWrite}, time.Hour, time.Now())
				require.NoError(t, issueErr)
				require.NoError(t, tokens.Save(t.Context(), token, auth.DatabaseAuditActor("fixture", org)))
				serviceSecrets[org] = secret
			}
			publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
			require.NoError(t, err)
			keys, err := authn.NewJWKSProvider(authn.JWKSProviderConfig{Snapshot: fmt.Sprintf(
				`{"keys":[{"kty":"OKP","crv":"Ed25519","x":"%s","kid":"webhook-audit","use":"sig","alg":"EdDSA","state":"active"}]}`, base64.RawURLEncoding.EncodeToString(publicKey))})
			require.NoError(t, err)
			verifier, err := auth.NewFirstPartyVerifier("https://goformx.com", "https://api.goformx.com", keys, assertionreplay.NewStore(database))
			require.NoError(t, err)
			router := echo.New()
			web.NewV1APIHandlerWithLimits(forms, tokens, nil, web.DefaultV1Limits(), verifier).RegisterRoutes(router)
			server := httptest.NewServer(router)
			t.Cleanup(server.Close)
			client := &http.Client{Timeout: 10 * time.Second}
			var lastActor auth.AuditActor
			request := func(org, method, path, body string, status int) []byte {
				bearer := serviceSecrets[org]
				lastActor = auth.AuditActor{OrganizationID: org, CredentialClass: credentialClass,
					CredentialID: auth.LookupID(bearer), SubjectID: auth.LookupID(bearer)}
				if credentialClass == auth.CredentialClassFirstPartyAssertion {
					id := uuid.NewString()
					bearer = signBoundaryAssertion(t, privateKey, "webhook-audit", org, id, auth.ScopeWebhooksWrite, time.Now())
					payload, decodeErr := base64.RawURLEncoding.DecodeString(strings.Split(bearer, ".")[1])
					require.NoError(t, decodeErr)
					var claims struct {
						Subject string `json:"sub"`
						Request string `json:"rid"`
					}
					require.NoError(t, json.Unmarshal(payload, &claims))
					lastActor.CredentialID, lastActor.SubjectID, lastActor.RequestID = id, claims.Subject, claims.Request
				}
				req, requestErr := http.NewRequestWithContext(t.Context(), method, server.URL+path, strings.NewReader(body))
				require.NoError(t, requestErr)
				req.Header.Set("Authorization", "Bearer "+bearer)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Organization-ID", foreignID)
				if credentialClass == auth.CredentialClassServiceToken {
					lastActor.CorrelationID = "caller-" + uuid.NewString()
					req.Header.Set(constants.HeaderTraceID, lastActor.CorrelationID)
				}
				response, requestErr := client.Do(req)
				require.NoError(t, requestErr)
				defer response.Body.Close()
				data, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
				require.NoError(t, readErr)
				require.Equal(t, status, response.StatusCode, string(data))
				if credentialClass == auth.CredentialClassFirstPartyAssertion {
					require.Equal(t, lastActor.RequestID, response.Header.Get(constants.HeaderTraceID))
				}
				if credentialClass == auth.CredentialClassServiceToken {
					require.Equal(t, lastActor.CorrelationID, response.Header.Get(constants.HeaderTraceID))
				}
				for _, secret := range []string{"webhook-audit-failure-canary", "old-signing-secret", "new-signing-secret", "Bearer private-header", "/private-path"} {
					require.NotContains(t, string(data), secret)
				}
				if status == http.StatusServiceUnavailable {
					require.Contains(t, string(data), "management_audit_unavailable")
				}
				return data
			}
			countAudits := func() int64 {
				var count int64
				require.NoError(t, db.Table("management_audit").Where("form_id = ?", form.ID).Count(&count).Error)
				return count
			}
			assertLastAudit := func(kind string) {
				var records []struct{ AuditID, SubjectID, CredentialID, CredentialClass, OrganizationID, RequestID, CorrelationID, Event string }
				query := db.Table("management_audit").Where(
					"form_id = ? AND event = ? AND credential_id = ?", form.ID, kind, lastActor.CredentialID)
				if credentialClass == auth.CredentialClassFirstPartyAssertion {
					query = query.Where("request_id = ?", lastActor.RequestID)
				} else {
					query = query.Where("correlation_id = ?", lastActor.CorrelationID)
				}
				require.NoError(t, query.Find(&records).Error)
				require.Len(t, records, 1, "one mutation must produce one independently identified event")
				record := records[0]
				require.NoError(t, uuid.Validate(record.AuditID))
				require.Equal(t, lastActor.SubjectID, record.SubjectID)
				require.Equal(t, lastActor.CredentialID, record.CredentialID)
				require.Equal(t, string(credentialClass), record.CredentialClass)
				require.Equal(t, organizationID, record.OrganizationID)
				require.Equal(t, kind, record.Event)
				if credentialClass == auth.CredentialClassFirstPartyAssertion {
					require.Equal(t, lastActor.RequestID, record.RequestID)
					require.Empty(t, record.CorrelationID)
				} else {
					require.NoError(t, uuid.Validate(record.RequestID))
					require.NotEqual(t, lastActor.CorrelationID, record.RequestID)
					require.Equal(t, lastActor.CorrelationID, record.CorrelationID)
				}
			}
			fault := func(enabled bool) {
				if enabled {
					require.NoError(t, db.Exec(`CREATE OR REPLACE FUNCTION reject_webhook_audit() RETURNS trigger LANGUAGE plpgsql AS $$
					BEGIN RAISE EXCEPTION 'webhook-audit-failure-canary'; END $$;
					CREATE TRIGGER reject_webhook_audit BEFORE INSERT ON management_audit FOR EACH ROW EXECUTE FUNCTION reject_webhook_audit();`).Error)
				} else {
					require.NoError(t, db.Exec("DROP TRIGGER reject_webhook_audit ON management_audit").Error)
				}
			}
			config := domainwebhook.SecretConfig{SigningSecret: strings.Repeat("old-signing-secret", 3), Headers: map[string]string{"Authorization": "Bearer private-header"}}
			put := func() (*domainwebhook.Endpoint, error) {
				return forms.PutWebhookEndpoint(t.Context(), organizationID, form.ID, "https://example.com/private-path", config, true, actor)
			}
			fault(true)
			_, err = put()
			require.ErrorIs(t, err, managementaudit.ErrUnavailable)
			_, err = forms.GetWebhookEndpoint(t.Context(), organizationID, form.ID)
			require.ErrorIs(t, err, domainwebhook.ErrNotFound)
			fault(false)
			endpoint, err := put()
			require.NoError(t, err)
			require.EqualValues(t, 1, countAudits())
			var snapshot []byte
			require.NoError(t, db.Raw("SELECT encrypted_config FROM webhook_endpoints WHERE form_id = ?", form.ID).Row().Scan(&snapshot))
			fault(true)
			_, err = put()
			require.ErrorIs(t, err, managementaudit.ErrUnavailable)
			path := "/v1/forms/" + form.ID + "/webhook"
			request(organizationID, http.MethodPatch, path, `{"enabled":false}`, http.StatusServiceUnavailable)
			rotation := `{"signingSecret":"` + strings.Repeat("new-signing-secret", 3) + `"}`
			request(organizationID, http.MethodPatch, path, rotation, http.StatusServiceUnavailable)
			request(organizationID, http.MethodDelete, path, "", http.StatusServiceUnavailable)
			var afterFailure []byte
			require.NoError(t, db.Raw("SELECT encrypted_config FROM webhook_endpoints WHERE form_id = ?", form.ID).Row().Scan(&afterFailure))
			require.Equal(t, snapshot, afterFailure)
			unchanged, err := forms.GetWebhookEndpoint(t.Context(), organizationID, form.ID)
			require.NoError(t, err)
			require.True(t, unchanged.Enabled)
			require.EqualValues(t, 1, countAudits())
			fault(false)
			for _, body := range []string{`{}`, `{"enabled":null}`, `{"enabled":true,"signingSecret":null}`, `{"enabled":false,"signingSecret":"` + config.SigningSecret + `"}`, `{"signingSecret":"short"}`} {
				request(organizationID, http.MethodPatch, path, body, http.StatusUnprocessableEntity)
			}
			request(organizationID, http.MethodPatch, path, `{"headers":{"Authorization":"leak"}}`, http.StatusBadRequest)
			request(foreignID, http.MethodPatch, path, `{"enabled":false}`, http.StatusNotFound)
			request(foreignID, http.MethodDelete, path, "", http.StatusNotFound)
			_, err = forms.PatchWebhookEndpoint(t.Context(), organizationID, form.ID, domainwebhook.EndpointChange{Enabled: new(bool)}, auth.AuditActor{})
			require.ErrorIs(t, err, managementaudit.ErrInvalid)
			_, err = forms.PatchWebhookEndpoint(t.Context(), organizationID, form.ID, domainwebhook.EndpointChange{Enabled: new(bool)}, auth.DatabaseAuditActor("fixture", foreignID))
			require.ErrorIs(t, err, managementaudit.ErrInvalid)
			require.EqualValues(t, 1, countAudits())
			submit := func() {
				_, _, submitErr := forms.CreateSubmissionIdempotent(t.Context(), &model.FormSubmission{FormID: form.ID, SchemaVersion: 1, IdempotencyKey: uuid.NewString(), Data: model.JSON{"private": "submission-data"}, SubmittedAt: time.Now(), Status: model.SubmissionStatusAccepted})
				require.NoError(t, submitErr)
			}
			replaced, err := put()
			require.NoError(t, err)
			require.Equal(t, endpoint.ID, replaced.ID)
			require.EqualValues(t, 2, countAudits())
			require.NoError(t, db.Raw("SELECT encrypted_config FROM webhook_endpoints WHERE form_id = ?", form.ID).Row().Scan(&snapshot))
			submit()
			request(organizationID, http.MethodPatch, path, `{"enabled":false}`, http.StatusOK)
			assertLastAudit("webhook.paused")
			paused, err := forms.GetWebhookEndpoint(t.Context(), organizationID, form.ID)
			require.NoError(t, err)
			request(organizationID, http.MethodPatch, path, `{"enabled":false}`, http.StatusOK)
			repeated, err := forms.GetWebhookEndpoint(t.Context(), organizationID, form.ID)
			require.NoError(t, err)
			require.Equal(t, paused.UpdatedAt, repeated.UpdatedAt)
			submit()
			deliveries, err := forms.ListWebhookDeliveries(t.Context(), organizationID, form.ID, 100)
			require.NoError(t, err)
			require.Len(t, deliveries, 1)
			require.Equal(t, snapshot, deliveries[0].EncryptedConfig)
			request(organizationID, http.MethodPatch, path, rotation, http.StatusOK)
			assertLastAudit("webhook.signing_secret_rotated")
			var rotated []byte
			require.NoError(t, db.Raw("SELECT encrypted_config FROM webhook_endpoints WHERE form_id = ?", form.ID).Row().Scan(&rotated))
			decrypted, err := cipher.Decrypt(rotated, form.ID)
			require.NoError(t, err)
			require.Equal(t, strings.Repeat("new-signing-secret", 3), decrypted.SigningSecret)
			require.Equal(t, config.Headers, decrypted.Headers)
			require.Equal(t, "https://example.com/private-path", decrypted.DestinationURL)
			request(organizationID, http.MethodPatch, path, `{"enabled":true}`, http.StatusOK)
			assertLastAudit("webhook.resumed")
			submit()
			current, err := forms.ListWebhookDeliveries(t.Context(), organizationID, form.ID, 100)
			require.NoError(t, err)
			require.Len(t, current, 2)
			require.Equal(t, rotated, current[0].EncryptedConfig)
			require.Equal(t, snapshot, current[1].EncryptedConfig)
			// Simultaneous pause requests serialize and emit exactly one mutation.
			before := countAudits()
			var wg sync.WaitGroup
			failures := make(chan error, 8)
			for range 8 {
				wg.Go(func() {
					_, patchErr := forms.PatchWebhookEndpoint(t.Context(), organizationID, form.ID, domainwebhook.EndpointChange{Enabled: new(bool)}, actor)
					failures <- patchErr
				})
			}
			wg.Wait()
			close(failures)
			for patchErr := range failures {
				require.NoError(t, patchErr)
			}
			require.Equal(t, before+1, countAudits())
			claimed, _, err := forms.ClaimDelivery(t.Context(), time.Minute)
			require.NoError(t, err)
			require.Equal(t, deliveries[0].ID, claimed.ID)
			require.Equal(t, snapshot, claimed.EncryptedConfig)
			require.NoError(t, forms.MarkDeliveryFailed(t.Context(), claimed.ID, "network", nil, false, 1, time.Second, time.Minute, time.Now()))
			replayPath := "/v1/forms/" + form.ID + "/deliveries/" + claimed.ID + "/replay"
			fault(true)
			request(organizationID, http.MethodPost, replayPath, "", http.StatusServiceUnavailable)
			var state string
			require.NoError(t, db.Raw("SELECT status FROM webhook_deliveries WHERE uuid = ?", claimed.ID).Scan(&state).Error)
			require.Equal(t, "dead_letter", state)
			fault(false)
			request(organizationID, http.MethodDelete, path, "", http.StatusNoContent)
			assertLastAudit("webhook.deleted")
			request(foreignID, http.MethodPost, replayPath, "", http.StatusNotFound)
			request(organizationID, http.MethodPost, replayPath, "", http.StatusAccepted)
			assertLastAudit("webhook.delivery_replayed")
			before = countAudits()
			request(organizationID, http.MethodPost, replayPath, "", http.StatusNotFound)
			require.Equal(t, before, countAudits())
			var retained []byte
			require.NoError(t, db.Raw("SELECT encrypted_config FROM webhook_deliveries WHERE uuid = ?", claimed.ID).Row().Scan(&retained))
			require.Equal(t, snapshot, retained)
			var audits string
			require.NoError(t, db.Raw("SELECT json_agg(a)::text FROM management_audit a WHERE form_id = ?", form.ID).Scan(&audits).Error)
			for _, forbidden := range []string{config.SigningSecret, decrypted.SigningSecret, "private-header", "private-path", "submission-data", "encrypted_config", "destination_origin"} {
				require.NotContains(t, audits, forbidden)
			}
			require.Contains(t, audits, endpoint.ID)
			down, err := os.ReadFile("../../migrations/postgresql/2026083004_webhook_management_audit.down.sql")
			require.NoError(t, err)
			require.ErrorContains(t, db.Exec(string(down)).Error, "retained webhook history")
			require.Equal(t, before, countAudits())
			require.NoError(t, db.Exec("DELETE FROM forms WHERE uuid = ?", form.ID).Error)
			require.Equal(t, before, countAudits(), "resource deletion must preserve audit history")
		})
	}
}
