package main

import (
	"context"
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

func (unavailableTokens) FindByID(context.Context, string) (*auth.ServiceToken, error) {
	return nil, errors.New("not found")
}

func (unavailableTokens) MarkUsed(context.Context, string, time.Time) error {
	return nil
}

func TestProductionRouterMountsOnlyHealthAndSchemaFirstAPI(t *testing.T) {
	t.Parallel()
	repository := mockform.NewMockRepository(gomock.NewController(t))
	router := newRouter(&config.Config{}, repository, unavailableTokens{}, nil)
	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
		require.False(t, strings.HasPrefix(route.Path, "/api/"), route.Path)
	}
	for _, expected := range []string{
		"GET /health",
		"HEAD /health",
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
	router := newRouter(&config.Config{}, repository, unavailableTokens{}, nil)
	request := httptest.NewRequest(http.MethodGet, "/v1/forms", nil)
	request.Header.Set("X-User-Id", "11111111-1111-4111-8111-111111111111")
	request.Header.Set("X-Timestamp", time.Now().UTC().Format(time.RFC3339))
	request.Header.Set("X-Signature", "legacy-hmac-assertion")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Contains(t, response.Body.String(), "unauthorized")
}
