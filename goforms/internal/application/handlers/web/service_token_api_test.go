package web

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/goformx/goforms/internal/application/validation"
	"github.com/goformx/goforms/internal/domain/auth"
	mockform "github.com/goformx/goforms/test/mocks/form"
)

type managementTokenRepository struct {
	token       *auth.ServiceToken
	saved       *auth.ServiceToken
	revokeOrg   string
	revokeToken string
}

func (r *managementTokenRepository) FindByID(_ context.Context, tokenID string) (*auth.ServiceToken, error) {
	if r.token != nil && r.token.ID == tokenID {
		return r.token, nil
	}
	return nil, nil
}

func (*managementTokenRepository) MarkUsed(context.Context, string, time.Time) error { return nil }

func (r *managementTokenRepository) Save(_ context.Context, token *auth.ServiceToken) error {
	r.saved = token
	return nil
}

func (r *managementTokenRepository) ListByOrganization(_ context.Context, organizationID string, _ int) ([]*auth.ServiceToken, error) {
	if r.saved == nil || r.saved.OwnerID != organizationID {
		return []*auth.ServiceToken{}, nil
	}
	return []*auth.ServiceToken{r.saved}, nil
}

func (r *managementTokenRepository) RevokeByOrganization(_ context.Context, organizationID, tokenID string, _ time.Time) error {
	r.revokeOrg, r.revokeToken = organizationID, tokenID
	return nil
}

func TestServiceTokenManagementScopesSecretsAndOrganizationBoundary(t *testing.T) {
	t.Parallel()
	managementCredential, plaintext, err := auth.Issue("owner-a", []auth.Scope{
		auth.ScopeTokensRead, auth.ScopeTokensWrite, auth.ScopeFormsRead,
	}, time.Hour, time.Now())
	require.NoError(t, err)
	tokens := &managementTokenRepository{token: managementCredential}
	repository := mockform.NewMockRepository(gomock.NewController(t))
	router := echo.New()
	newV1APIHandler(repository, tokens, validation.NewComprehensiveValidator(), nil).RegisterRoutes(router)

	created := requestJSON(t, router, http.MethodPost, "/v1/service-tokens", map[string]any{
		"name": "Read-only dashboard", "scopes": []string{"forms:read"}, "expiresInSeconds": 3600,
	}, plaintext, "", nil)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	require.Equal(t, "no-store", created.Header().Get(echo.HeaderCacheControl))
	require.Contains(t, created.Body.String(), `"token":"gfst_`)
	require.Equal(t, "owner-a", tokens.saved.OwnerID)
	require.Equal(t, "Read-only dashboard", tokens.saved.Name)
	require.True(t, tokens.saved.HasScope(auth.ScopeFormsRead))
	require.False(t, tokens.saved.HasScope(auth.ScopeFormsWrite))

	listed := requestJSON(t, router, http.MethodGet, "/v1/service-tokens", nil, plaintext, "", nil)
	require.Equal(t, http.StatusOK, listed.Code, listed.Body.String())
	require.Contains(t, listed.Body.String(), "Read-only dashboard")
	require.Contains(t, listed.Body.String(), `"organizationId":"owner-a"`)
	require.NotContains(t, listed.Body.String(), `"token":"gfst_`)
	require.NotContains(t, strings.ToLower(listed.Body.String()), "hash")

	deniedDelegation := requestJSON(t, router, http.MethodPost, "/v1/service-tokens", map[string]any{
		"name": "Too powerful", "scopes": []string{"forms:write"}, "expiresInSeconds": 3600,
	}, plaintext, "", nil)
	require.Equal(t, http.StatusUnprocessableEntity, deniedDelegation.Code, deniedDelegation.Body.String())
	require.Contains(t, deniedDelegation.Body.String(), "scope_denied")

	revoked := requestJSON(t, router, http.MethodDelete, "/v1/service-tokens/target-token", nil, plaintext, "", nil)
	require.Equal(t, http.StatusNoContent, revoked.Code, revoked.Body.String())
	require.Equal(t, "owner-a", tokens.revokeOrg)
	require.Equal(t, "target-token", tokens.revokeToken)
}

func TestServiceTokenManagementRequiresDedicatedScopes(t *testing.T) {
	t.Parallel()
	credential, plaintext, err := auth.Issue("owner-a", []auth.Scope{auth.ScopeFormsWrite}, time.Hour, time.Now())
	require.NoError(t, err)
	tokens := &managementTokenRepository{token: credential}
	router := echo.New()
	newV1APIHandler(mockform.NewMockRepository(gomock.NewController(t)), tokens,
		validation.NewComprehensiveValidator(), nil).RegisterRoutes(router)

	response := requestJSON(t, router, http.MethodPost, "/v1/service-tokens", map[string]any{
		"name": "Denied", "scopes": []string{"forms:write"}, "expiresInSeconds": 3600,
	}, plaintext, "", nil)
	require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
	require.Nil(t, tokens.saved)
}
