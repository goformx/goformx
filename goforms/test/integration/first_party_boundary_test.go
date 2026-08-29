package integration_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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
	"github.com/goformx/goforms/internal/infrastructure/authn"
	assertionreplay "github.com/goformx/goforms/internal/infrastructure/repository/assertionreplay"
	formrepository "github.com/goformx/goforms/internal/infrastructure/repository/form"
	tokenrepository "github.com/goformx/goforms/internal/infrastructure/repository/token"
	mocklogging "github.com/goformx/goforms/test/mocks/logging"
)

type boundaryDB struct{ db *gorm.DB }

func (d *boundaryDB) Close() error                          { return nil }
func (d *boundaryDB) MonitorConnectionPool(context.Context) {}
func (d *boundaryDB) Ping(context.Context) error            { return nil }
func (d *boundaryDB) GetDB() *gorm.DB                       { return d.db }

func TestFirstPartyAssertionBoundaryEnforcesReplayScopeAndTenant(t *testing.T) {
	databaseURL := os.Getenv("GOFORMX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PostgreSQL integration is run by the canonical task verify command")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	require.NoError(t, err)
	database := &boundaryDB{db: db}
	organizationID, foreignOrganizationID := uuid.NewString(), uuid.NewString()
	formID := uuid.NewString()
	require.NoError(t, db.Exec(`
		INSERT INTO users (uuid, email, hashed_password, first_name, last_name)
		VALUES (?, ?, 'not-used', 'Boundary', 'Fixture')
	`, organizationID, organizationID+"@example.test").Error)
	require.NoError(t, db.Exec(`
		INSERT INTO forms (uuid, organization_id, name, title, description, active, status, public_key,
			current_schema_version, cors_origins, cors_methods, cors_headers)
		VALUES (?, ?, ?, 'Boundary Fixture', '', true, 'draft', ?, 1, '{}'::jsonb, '{}'::jsonb, '{}'::jsonb)
	`, formID, organizationID, "boundary-"+uuid.NewString()[:8], "gfpk_"+uuid.NewString()).Error)
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM first_party_assertion_replays WHERE organization_id IN (?, ?)",
			organizationID, foreignOrganizationID).Error
		_ = db.Exec("DELETE FROM forms WHERE organization_id = ?", organizationID).Error
		_ = db.Exec("DELETE FROM users WHERE uuid = ?", organizationID).Error
	})

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	keyID := "gofx-fpa-integration"
	snapshot := fmt.Sprintf(`{"keys":[{"kty":"OKP","crv":"Ed25519","x":"%s","kid":"%s","use":"sig","alg":"EdDSA","state":"active"}]}`,
		base64.RawURLEncoding.EncodeToString(publicKey), keyID)
	keys, err := authn.NewJWKSProvider(authn.JWKSProviderConfig{Snapshot: snapshot})
	require.NoError(t, err)
	verifier, err := auth.NewFirstPartyVerifier("https://goformx.com", "https://api.goformx.com",
		keys, assertionreplay.NewStore(database))
	require.NoError(t, err)
	logger := mocklogging.NewMockLogger(gomock.NewController(t))
	logger.EXPECT().Debug(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	router := echo.New()
	web.NewV1APIHandlerWithLimits(
		formrepository.NewStore(database, logger), tokenrepository.NewStore(database), nil, web.DefaultV1Limits(), verifier,
	).RegisterRoutes(router)
	now := time.Now().UTC().Truncate(time.Second)
	owned := signBoundaryAssertion(t, privateKey, keyID, organizationID, uuid.NewString(), auth.ScopeFormsRead, now)
	response := boundaryRequest(router, http.MethodGet, "/v1/forms", owned)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), formID)
	require.Equal(t, http.StatusUnauthorized,
		boundaryRequest(router, http.MethodGet, "/v1/forms", owned).Code, "assertions are single-use")

	underScoped := signBoundaryAssertion(t, privateKey, keyID, organizationID, uuid.NewString(), auth.ScopeFormsWrite, now)
	require.Equal(t, http.StatusForbidden,
		boundaryRequest(router, http.MethodGet, "/v1/forms", underScoped).Code)
	foreign := signBoundaryAssertion(t, privateKey, keyID, foreignOrganizationID, uuid.NewString(), auth.ScopeFormsRead, now)
	foreignResponse := boundaryRequest(router, http.MethodGet, "/v1/forms/"+formID, foreign)
	require.Equal(t, http.StatusNotFound, foreignResponse.Code, foreignResponse.Body.String())
}

func signBoundaryAssertion(
	t *testing.T,
	privateKey ed25519.PrivateKey,
	keyID string,
	organizationID string,
	assertionID string,
	scope auth.Scope,
	now time.Time,
) string {
	t.Helper()
	header, err := json.Marshal(map[string]any{
		"alg": auth.FirstPartyAssertionAlgorithm, "typ": auth.FirstPartyAssertionType, "kid": keyID,
	})
	require.NoError(t, err)
	claims, err := json.Marshal(map[string]any{
		"iss": "https://goformx.com", "aud": "https://api.goformx.com", "sub": uuid.NewString(),
		"org": organizationID, "scp": []auth.Scope{scope}, "iat": now.Unix(), "nbf": now.Unix(),
		"exp": now.Add(time.Minute).Unix(), "jti": assertionID, "rid": uuid.NewString(), "ver": 1,
	})
	require.NoError(t, err)
	message := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	return message + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(message)))
}

func boundaryRequest(router http.Handler, method, path, credential string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	request.Header.Set(echo.HeaderAuthorization, "Bearer "+credential)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}
