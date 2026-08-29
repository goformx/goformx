package serviceauth_test

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	serviceauth "github.com/goformx/goforms/internal/application/middleware/serviceauth"
	"github.com/goformx/goforms/internal/domain/auth"
)

type tokenRepository struct {
	token       *auth.ServiceToken
	markUsedErr error
	usedAt      *time.Time
	findCalls   int
}

type assertionVerifier struct {
	principal auth.FirstPartyPrincipal
	err       error
	calls     int
}

func (v *assertionVerifier) VerifyAndConsume(
	_ context.Context,
	_ string,
	_ time.Time,
) (auth.FirstPartyPrincipal, error) {
	v.calls++
	return v.principal, v.err
}

func (r *tokenRepository) FindByID(_ context.Context, tokenID string) (*auth.ServiceToken, error) {
	r.findCalls++
	if r.token == nil || r.token.ID != tokenID {
		return nil, errors.New("not found")
	}
	return r.token, nil
}

func (r *tokenRepository) MarkUsed(_ context.Context, _ string, now time.Time) error {
	r.usedAt = &now
	return r.markUsedErr
}

func TestMiddlewareEnforcesBearerScopeAndOwner(t *testing.T) {
	t.Parallel()
	now := time.Now()
	token, plaintext, err := auth.Issue("owner-a", []auth.Scope{auth.ScopeFormsRead}, time.Hour, now)
	require.NoError(t, err)
	repository := &tokenRepository{token: token}
	middleware := serviceauth.New(repository)

	e := echo.New()
	handler := middleware.Require(auth.ScopeFormsRead)(func(c echo.Context) error {
		require.NoError(t, serviceauth.RequireOwner(c, "owner-a"))
		require.Error(t, serviceauth.RequireOwner(c, "owner-b"))
		principal, ok := serviceauth.PrincipalFrom(c)
		require.True(t, ok)
		require.Equal(t, serviceauth.CredentialClassServiceToken, principal.CredentialClass)
		require.Equal(t, "owner-a", principal.OrganizationID)
		return c.NoContent(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/forms", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+plaintext)
	recorder := httptest.NewRecorder()
	require.NoError(t, handler(e.NewContext(req, recorder)))
	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.NotNil(t, repository.usedAt)

	denied := middleware.Require(auth.ScopeFormsWrite)(func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	err = denied(e.NewContext(req, httptest.NewRecorder()))
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusForbidden, httpErr.Code)
}

func TestMiddlewareConvergesFirstPartyAssertionWithoutTokenFallback(t *testing.T) {
	t.Parallel()
	verifier := &assertionVerifier{principal: auth.FirstPartyPrincipal{
		AssertionID: "33333333-3333-4333-8333-333333333333",
		SubjectID:   "11111111-1111-4111-8111-111111111111", OrganizationID: "22222222-2222-4222-8222-222222222222",
		RequestID: "44444444-4444-4444-8444-444444444444",
		Scopes:    map[auth.Scope]struct{}{auth.ScopeFormsRead: {}},
	}}
	repository := &tokenRepository{}
	middleware := serviceauth.NewWithAssertions(repository, verifier)
	handler := middleware.Require(auth.ScopeFormsRead)(func(c echo.Context) error {
		principal, ok := serviceauth.PrincipalFrom(c)
		require.True(t, ok)
		require.Equal(t, serviceauth.CredentialClassFirstPartyAssertion, principal.CredentialClass)
		require.Equal(t, verifier.principal.OrganizationID, principal.OwnerID)
		require.Equal(t, verifier.principal.RequestID, principal.RequestID)
		return c.NoContent(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/forms", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+assertionShapedCredential())
	recorder := httptest.NewRecorder()
	require.NoError(t, handler(echo.New().NewContext(req, recorder)))
	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Equal(t, 1, verifier.calls)
	require.Zero(t, repository.findCalls)

	denied := middleware.Require(auth.ScopeFormsWrite)(func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
	err := denied(echo.New().NewContext(req, httptest.NewRecorder()))
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusForbidden, httpErr.Code)

	verifier.err = auth.ErrInvalidFirstPartyAssertion
	err = handler(echo.New().NewContext(req, httptest.NewRecorder()))
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusUnauthorized, httpErr.Code)
}

func TestMiddlewareReturnsUnavailableForReplayStoreFailure(t *testing.T) {
	t.Parallel()
	verifier := &assertionVerifier{err: auth.ErrFirstPartyAuthUnavailable}
	middleware := serviceauth.NewWithAssertions(&tokenRepository{}, verifier)
	handler := middleware.Require(auth.ScopeFormsRead)(func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/forms", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+assertionShapedCredential())
	err := handler(echo.New().NewContext(req, httptest.NewRecorder()))
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusServiceUnavailable, httpErr.Code)
}

func TestMiddlewareRejectsMissingAndUnknownTokens(t *testing.T) {
	t.Parallel()
	e := echo.New()
	middleware := serviceauth.New(&tokenRepository{})
	handler := middleware.Require(auth.ScopeFormsRead)(func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	err := handler(e.NewContext(httptest.NewRequest(http.MethodGet, "/v1/forms", nil), httptest.NewRecorder()))
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusUnauthorized, httpErr.Code)
}

func TestMiddlewareFailsClosedWhenUsageAuditCannotBeWritten(t *testing.T) {
	t.Parallel()
	now := time.Now()
	token, plaintext, err := auth.Issue("owner-a", []auth.Scope{auth.ScopeFormsRead}, time.Hour, now)
	require.NoError(t, err)
	middleware := serviceauth.New(&tokenRepository{token: token, markUsedErr: errors.New("database unavailable")})
	handler := middleware.Require(auth.ScopeFormsRead)(func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/forms", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+plaintext)
	err = handler(echo.New().NewContext(req, httptest.NewRecorder()))
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusServiceUnavailable, httpErr.Code)
}

func TestMiddlewareTreatsExpiredServiceTokenAsAuthenticationFailure(t *testing.T) {
	t.Parallel()
	now := time.Now()
	token, plaintext, err := auth.Issue("owner-a", []auth.Scope{auth.ScopeFormsRead}, time.Minute, now.Add(-2*time.Minute))
	require.NoError(t, err)
	middleware := serviceauth.New(&tokenRepository{token: token})
	handler := middleware.Require(auth.ScopeFormsRead)(func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/v1/forms", nil)
	request.Header.Set(echo.HeaderAuthorization, "Bearer "+plaintext)
	err = handler(echo.New().NewContext(request, httptest.NewRecorder()))
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusUnauthorized, httpErr.Code)
}

func assertionShapedCredential() string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"gofx-fpa+jwt","kid":"test"}`))
	return header + ".claims.signature"
}
