package web

import (
	"context"
	"net/http"
	"net/url"
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
	actor       auth.AuditActor
	listed      []*auth.ServiceToken
	hasMore     bool
	listOptions auth.TokenListOptions
}

func (r *managementTokenRepository) FindByID(_ context.Context, tokenID string) (*auth.ServiceToken, error) {
	if r.token != nil && r.token.ID == tokenID {
		return r.token, nil
	}
	return nil, nil
}

func (*managementTokenRepository) MarkUsed(context.Context, string, time.Time) error { return nil }

func (r *managementTokenRepository) Save(_ context.Context, token *auth.ServiceToken, actor auth.AuditActor) error {
	r.saved = token
	r.actor = actor
	return nil
}

func (r *managementTokenRepository) ListByOrganization(_ context.Context, organizationID string, options auth.TokenListOptions) ([]*auth.ServiceToken, bool, error) {
	r.listOptions = options
	if r.listed != nil {
		return r.listed, r.hasMore, nil
	}
	if r.saved == nil || r.saved.OwnerID != organizationID {
		return []*auth.ServiceToken{}, false, nil
	}
	return []*auth.ServiceToken{r.saved}, false, nil
}

func TestServiceTokenListUsesOpaqueKeysetPaginationAndStrictQuery(t *testing.T) {
	t.Parallel()
	credential, plaintext, err := auth.Issue("owner-a", []auth.Scope{auth.ScopeTokensRead}, time.Hour, time.Now())
	require.NoError(t, err)
	created := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	tokens := &managementTokenRepository{token: credential, hasMore: true, listed: []*auth.ServiceToken{{
		ID: "abcdefghijklmnop", Name: "older", OwnerID: "owner-a", CreatedAt: created, ExpiresAt: created.Add(time.Hour),
	}}}
	router := echo.New()
	newV1APIHandler(mockform.NewMockRepository(gomock.NewController(t)), tokens,
		validation.NewComprehensiveValidator(), nil).RegisterRoutes(router)

	first := requestJSON(t, router, http.MethodGet, "/v1/service-tokens?limit=1", nil, plaintext, "", nil)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	require.Contains(t, first.Body.String(), `"nextCursor":"`)
	require.Equal(t, 1, tokens.listOptions.Limit)
	cursor := encodeServiceTokenCursor(tokens.listed[0])
	second := requestJSON(t, router, http.MethodGet, "/v1/service-tokens?limit=1&cursor="+url.QueryEscape(cursor), nil, plaintext, "", nil)
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	require.Equal(t, created, tokens.listOptions.Before)
	require.Equal(t, "abcdefghijklmnop", tokens.listOptions.BeforeID)

	for _, query := range []string{"unknown=1", "limit=", "cursor=", "limit=1&limit=2", "cursor=invalid",
		"cursor=" + strings.Repeat("a", 1025), "cursor=" + strings.Repeat("a", 4097)} {
		response := requestJSON(t, router, http.MethodGet, "/v1/service-tokens?"+query, nil, plaintext, "", nil)
		require.Equal(t, http.StatusBadRequest, response.Code, query)
	}
}

func (r *managementTokenRepository) RevokeByOrganization(_ context.Context, organizationID, tokenID string, _ time.Time, actor auth.AuditActor) error {
	r.revokeOrg, r.revokeToken = organizationID, tokenID
	r.actor = actor
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
