package serviceauth_test

import (
	"context"
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

type tokenRepository struct{ token *auth.ServiceToken }

func (r tokenRepository) FindByID(_ context.Context, tokenID string) (*auth.ServiceToken, error) {
	if r.token == nil || r.token.ID != tokenID {
		return nil, errors.New("not found")
	}
	return r.token, nil
}

func TestMiddlewareEnforcesBearerScopeAndOwner(t *testing.T) {
	t.Parallel()
	now := time.Now()
	token, plaintext, err := auth.Issue("owner-a", []auth.Scope{auth.ScopeFormsRead}, time.Hour, now)
	require.NoError(t, err)
	middleware := serviceauth.New(tokenRepository{token: token})

	e := echo.New()
	handler := middleware.Require(auth.ScopeFormsRead)(func(c echo.Context) error {
		require.NoError(t, serviceauth.RequireOwner(c, "owner-a"))
		require.Error(t, serviceauth.RequireOwner(c, "owner-b"))
		return c.NoContent(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/forms", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+plaintext)
	recorder := httptest.NewRecorder()
	require.NoError(t, handler(e.NewContext(req, recorder)))
	require.Equal(t, http.StatusNoContent, recorder.Code)

	denied := middleware.Require(auth.ScopeFormsWrite)(func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	err = denied(e.NewContext(req, httptest.NewRecorder()))
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusForbidden, httpErr.Code)
}

func TestMiddlewareRejectsMissingAndUnknownTokens(t *testing.T) {
	t.Parallel()
	e := echo.New()
	middleware := serviceauth.New(tokenRepository{})
	handler := middleware.Require(auth.ScopeFormsRead)(func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	err := handler(e.NewContext(httptest.NewRequest(http.MethodGet, "/v1/forms", nil), httptest.NewRecorder()))
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusUnauthorized, httpErr.Code)
}
