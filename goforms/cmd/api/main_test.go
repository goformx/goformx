package main

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/infrastructure/config"
	mockform "github.com/goformx/goforms/test/mocks/form"
)

type unavailableTokens struct{}

type readinessStub struct{ err error }

type assertionVerifierStub struct{ principal auth.FirstPartyPrincipal }

func (stub assertionVerifierStub) VerifyAndConsume(
	context.Context,
	string,
	time.Time,
) (auth.FirstPartyPrincipal, error) {
	return stub.principal, nil
}

func (stub readinessStub) Ping(context.Context) error { return stub.err }

func (unavailableTokens) FindByID(context.Context, string) (*auth.ServiceToken, error) {
	return nil, errors.New("not found")
}

func (unavailableTokens) MarkUsed(context.Context, string, time.Time) error {
	return nil
}

func TestProductionRouterMountsOnlyHealthAndSchemaFirstAPI(t *testing.T) {
	t.Parallel()
	repository := mockform.NewMockRepository(gomock.NewController(t))
	router := newRouter(&config.Config{}, repository, unavailableTokens{}, readinessStub{}, nil)
	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
		require.False(t, strings.HasPrefix(route.Path, "/api/"), route.Path)
	}
	for _, expected := range []string{
		"GET /health",
		"HEAD /health",
		"GET /ready",
		"HEAD /ready",
		"GET /v1/forms",
		"POST /v1/forms",
		"GET /v1/public/forms/:publicKey/schema",
		"POST /v1/public/forms/:publicKey/submissions",
	} {
		_, ok := routes[expected]
		require.True(t, ok, expected)
	}
}

func TestProductionRouterRejectsLegacyAssertionHeaders(t *testing.T) {
	t.Parallel()
	repository := mockform.NewMockRepository(gomock.NewController(t))
	router := newRouter(&config.Config{}, repository, unavailableTokens{}, readinessStub{}, nil)
	request := httptest.NewRequest(http.MethodGet, "/v1/forms", nil)
	request.Header.Set("X-User-Id", "11111111-1111-4111-8111-111111111111")
	request.Header.Set("X-Timestamp", time.Now().UTC().Format(time.RFC3339))
	request.Header.Set("X-Signature", "legacy-hmac-assertion")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Contains(t, response.Body.String(), "unauthorized")
}

func TestProductionRouterConvergesFirstPartyIdentityOnOwnedQueries(t *testing.T) {
	t.Parallel()
	repository := mockform.NewMockRepository(gomock.NewController(t))
	organizationID := "22222222-2222-4222-8222-222222222222"
	repository.EXPECT().ListForms(gomock.Any(), organizationID, gomock.Any()).Return(nil, int64(0), nil)
	assertions := assertionVerifierStub{principal: auth.FirstPartyPrincipal{
		AssertionID: "33333333-3333-4333-8333-333333333333",
		SubjectID:   "11111111-1111-4111-8111-111111111111", OrganizationID: organizationID,
		RequestID: "44444444-4444-4444-8444-444444444444",
		Scopes:    map[auth.Scope]struct{}{auth.ScopeFormsRead: {}},
	}}
	router := newRouterWithAssertions(&config.Config{}, repository, unavailableTokens{}, readinessStub{}, nil, assertions)
	request := httptest.NewRequest(http.MethodGet, "/v1/forms", nil)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"gofx-fpa+jwt","kid":"test"}`))
	request.Header.Set("Authorization", "Bearer "+header+".claims.signature")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, assertions.principal.RequestID, response.Header().Get("X-Trace-Id"))
}

func TestReadinessReflectsDatabaseAvailability(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		pingErr    error
		statusCode int
		body       string
	}{
		{name: "ready", statusCode: http.StatusOK, body: `"status":"ready"`},
		{name: "database unavailable", pingErr: errors.New("database unavailable"), statusCode: http.StatusServiceUnavailable, body: `"status":"unavailable"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repository := mockform.NewMockRepository(gomock.NewController(t))
			router := newRouter(&config.Config{}, repository, unavailableTokens{}, readinessStub{err: test.pingErr}, nil)
			request := httptest.NewRequest(http.MethodGet, "/ready", nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, test.statusCode, response.Code)
			require.Contains(t, response.Body.String(), test.body)
			require.NotContains(t, response.Body.String(), "database unavailable")
		})
	}
}
