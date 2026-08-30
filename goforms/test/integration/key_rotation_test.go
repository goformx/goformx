package integration_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
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
	"github.com/goformx/goforms/internal/infrastructure/authn"
	assertionreplay "github.com/goformx/goforms/internal/infrastructure/repository/assertionreplay"
	formrepository "github.com/goformx/goforms/internal/infrastructure/repository/form"
	tokenrepository "github.com/goformx/goforms/internal/infrastructure/repository/token"
	mocklogging "github.com/goformx/goforms/test/mocks/logging"
)

type rotationKey struct {
	id      string
	public  ed25519.PublicKey
	private ed25519.PrivateKey
	state   auth.VerificationKeyState
}

func newRotationKey(t *testing.T, id string, state auth.VerificationKeyState) rotationKey {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return rotationKey{id: id, public: public, private: private, state: state}
}

func rotationJWKS(t *testing.T, keys ...rotationKey) string {
	t.Helper()
	entries := make([]map[string]string, 0, len(keys))
	for _, key := range keys {
		entries = append(entries, map[string]string{
			"kty": "OKP", "crv": "Ed25519", "alg": auth.FirstPartyAssertionAlgorithm,
			"use": "sig", "kid": key.id, "state": string(key.state),
			"x": base64.RawURLEncoding.EncodeToString(key.public),
		})
	}
	document, err := json.Marshal(map[string]any{"keys": entries})
	require.NoError(t, err)
	return string(document)
}

func TestFirstPartyKeyRotationDrill(t *testing.T) {
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PostgreSQL integration is run by the canonical task verify command")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	database := &boundaryDB{db: db}
	organizationID, foreignOrganizationID := uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		require.NoError(t, db.Exec("DELETE FROM first_party_assertion_replays WHERE organization_id IN (?, ?)", organizationID, foreignOrganizationID).Error)
		require.NoError(t, db.Exec("DELETE FROM forms WHERE organization_id = ?", organizationID).Error)
		require.NoError(t, db.Exec("DELETE FROM service_tokens WHERE organization_id = ?", organizationID).Error)
	})
	logger := mocklogging.NewMockLogger(gomock.NewController(t))
	logger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	forms := formrepository.NewStore(database, logger)
	form := model.NewForm(organizationID, "Rotation drill", "", model.JSON{
		"$schema": model.JSONSchemaDraft202012URI, "type": "object",
		"properties": map[string]any{"email": map[string]any{"type": "string"}},
	})
	form.Name = "rotation-" + uuid.NewString()
	require.NoError(t, forms.CreateForm(t.Context(), form))
	tokens := tokenrepository.NewStore(database)
	token, serviceCredential, err := auth.Issue(organizationID, []auth.Scope{auth.ScopeFormsRead}, time.Hour, time.Now())
	require.NoError(t, err)
	require.NoError(t, tokens.Save(t.Context(), token, auth.DatabaseAuditActor("integration-fixture", token.OwnerID)))
	old := newRotationKey(t, "old", auth.VerificationKeyActive)
	next := newRotationKey(t, "next", auth.VerificationKeyNext)
	replacement := newRotationKey(t, "replacement", auth.VerificationKeyNext)
	var published atomic.Value
	published.Store(rotationJWKS(t, old))
	var unavailable atomic.Bool
	discovery := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		if unavailable.Load() {
			http.Error(response, "discovery unavailable", http.StatusServiceUnavailable)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(response, published.Load().(string))
	}))
	t.Cleanup(discovery.Close)
	newRouter := func(t *testing.T, snapshot string, enabled bool) *echo.Echo {
		var verifier *auth.FirstPartyVerifier
		if enabled {
			keys, keyErr := authn.NewJWKSProvider(authn.JWKSProviderConfig{
				Snapshot: snapshot, URL: discovery.URL, HTTPClient: discovery.Client(), RefreshInterval: time.Nanosecond,
			})
			require.NoError(t, keyErr)
			verifier, keyErr = auth.NewFirstPartyVerifier("https://goformx.com", "https://api.goformx.com", keys, assertionreplay.NewStore(database))
			require.NoError(t, keyErr)
		}
		router := echo.New()
		if enabled {
			web.NewV1APIHandlerWithLimits(forms, tokens, nil, web.DefaultV1Limits(), verifier).RegisterRoutes(router)
		} else {
			web.NewV1APIHandler(forms, tokens, nil).RegisterRoutes(router)
		}
		return router
	}
	router := newRouter(t, published.Load().(string), true)
	assertion := func(t *testing.T, key rotationKey, organization string, scope auth.Scope) string {
		return signBoundaryAssertion(t, key.private, key.id, organization, uuid.NewString(), scope, time.Now().UTC().Truncate(time.Second))
	}
	check := func(t *testing.T, key rotationKey, expected int) {
		t.Helper()
		response := boundaryRequest(router, http.MethodGet, "/v1/forms/"+form.ID, assertion(t, key, organizationID, auth.ScopeFormsRead))
		require.Equal(t, expected, response.Code, "unexpected status for key %s", key.id)
		require.Equal(t, http.StatusOK, boundaryRequest(router, http.MethodGet, "/v1/forms/"+form.ID, serviceCredential).Code,
			"assertion-key transitions must not affect external service tokens")
	}

	t.Run("active_and_single_use", func(t *testing.T) {
		check(t, old, http.StatusOK)
		credential := assertion(t, old, organizationID, auth.ScopeFormsRead)
		require.Equal(t, http.StatusOK, boundaryRequest(router, http.MethodGet, "/v1/forms", credential).Code)
		require.Equal(t, http.StatusUnauthorized, boundaryRequest(router, http.MethodGet, "/v1/forms", credential).Code)
	})
	t.Run("announce_next", func(t *testing.T) {
		published.Store(rotationJWKS(t, old, next))
		check(t, old, http.StatusOK)
		check(t, next, http.StatusOK)
	})
	t.Run("switch_signer_and_retire", func(t *testing.T) {
		old.state, next.state = auth.VerificationKeyRetiring, auth.VerificationKeyActive
		published.Store(rotationJWKS(t, old, next))
		check(t, old, http.StatusOK)
		check(t, next, http.StatusOK)
	})
	t.Run("remove_drained_key", func(t *testing.T) {
		// The deployment runbook owns the real 65-second drain; this verifies the
		// resulting removal state without replacing the production clock or sleeping.
		old.state = auth.VerificationKeyRevoked
		published.Store(rotationJWKS(t, old, next))
		check(t, old, http.StatusUnauthorized)
		published.Store(rotationJWKS(t, next))
		check(t, old, http.StatusUnauthorized)
		check(t, next, http.StatusOK)
		stale := old
		stale.state = auth.VerificationKeyActive
		published.Store(rotationJWKS(t, stale, next))
		check(t, old, http.StatusUnauthorized)
		published.Store(rotationJWKS(t, next))
		require.Equal(t, http.StatusForbidden, boundaryRequest(router, http.MethodGet, "/v1/forms", assertion(t, next, organizationID, auth.ScopeFormsWrite)).Code)
		require.Equal(t, http.StatusNotFound, boundaryRequest(router, http.MethodGet, "/v1/forms/"+form.ID, assertion(t, next, foreignOrganizationID, auth.ScopeFormsRead)).Code)
	})
	t.Run("emergency_revoke_and_stale_discovery", func(t *testing.T) {
		next.state = auth.VerificationKeyRevoked
		published.Store(rotationJWKS(t, next, replacement))
		check(t, next, http.StatusUnauthorized)
		check(t, replacement, http.StatusOK)
		stale := next
		stale.state = auth.VerificationKeyActive
		published.Store(rotationJWKS(t, stale, replacement))
		check(t, next, http.StatusUnauthorized)
		check(t, replacement, http.StatusOK)
	})
	t.Run("disable_first_party_only", func(t *testing.T) {
		router = newRouter(t, "", false)
		check(t, next, http.StatusUnauthorized)
		check(t, replacement, http.StatusUnauthorized)
	})
	t.Run("cold_start_from_revoked_snapshot_with_discovery_down", func(t *testing.T) {
		replacement.state = auth.VerificationKeyActive
		snapshot := rotationJWKS(t, old, next, replacement)
		unavailable.Store(true)
		router = newRouter(t, snapshot, true)
		check(t, old, http.StatusUnauthorized)
		check(t, next, http.StatusUnauthorized)
		check(t, replacement, http.StatusOK)
		credential := assertion(t, replacement, organizationID, auth.ScopeFormsRead)
		require.Equal(t, http.StatusOK, boundaryRequest(router, http.MethodGet, "/v1/forms", credential).Code)
		router = newRouter(t, snapshot, true)
		require.Equal(t, http.StatusUnauthorized, boundaryRequest(router, http.MethodGet, "/v1/forms", credential).Code,
			"replay consumption survives verifier restart in PostgreSQL")
		unavailable.Store(false)
		check(t, next, http.StatusUnauthorized) // The publisher is still serving the stale active key.
		check(t, replacement, http.StatusOK)
	})
}
