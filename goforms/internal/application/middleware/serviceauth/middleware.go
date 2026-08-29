package serviceauth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/goformx/goforms/internal/domain/auth"
)

const principalContextKey = "service_token_principal"

type Repository interface {
	FindByID(ctx context.Context, tokenID string) (*auth.ServiceToken, error)
	MarkUsed(ctx context.Context, tokenID string, now time.Time) error
}

type Principal struct {
	TokenID string
	OwnerID string
	Scopes  map[auth.Scope]struct{}
}

// HasScope reports whether the authenticated principal may perform or delegate a scope.
func (p Principal) HasScope(scope auth.Scope) bool {
	_, ok := p.Scopes[scope]
	return ok
}

type Middleware struct {
	repository Repository
	now        func() time.Time
}

func New(repository Repository) *Middleware {
	return &Middleware{repository: repository, now: time.Now}
}

// Require authenticates a Bearer token and enforces one explicit control-plane scope.
func (m *Middleware) Require(scope auth.Scope) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			plaintext, err := bearerToken(c.Request().Header.Get(echo.HeaderAuthorization))
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
			}
			tokenID := auth.LookupID(plaintext)
			token, err := m.repository.FindByID(c.Request().Context(), tokenID)
			if err != nil || token == nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid service token")
			}
			now := m.now()
			if err := token.Authorize(plaintext, token.OwnerID, scope, now); err != nil {
				return echo.NewHTTPError(http.StatusForbidden, "service token is not authorized")
			}
			if err := m.repository.MarkUsed(c.Request().Context(), token.ID, now); err != nil {
				return echo.NewHTTPError(http.StatusServiceUnavailable, "service token audit update failed")
			}
			scopes := make(map[auth.Scope]struct{}, len(token.Scopes))
			for granted := range token.Scopes {
				scopes[granted] = struct{}{}
			}
			c.Set(principalContextKey, Principal{TokenID: token.ID, OwnerID: token.OwnerID, Scopes: scopes})
			return next(c)
		}
	}
}

func PrincipalFrom(c echo.Context) (Principal, bool) {
	principal, ok := c.Get(principalContextKey).(Principal)
	return principal, ok
}

// RequireOwner prevents a valid token from crossing aggregate ownership boundaries.
func RequireOwner(c echo.Context, ownerID string) error {
	principal, ok := PrincipalFrom(c)
	if !ok || ownerID == "" || principal.OwnerID != ownerID {
		return echo.NewHTTPError(http.StatusForbidden, "resource owner mismatch")
	}
	return nil
}

func bearerToken(header string) (string, error) {
	scheme, value, found := strings.Cut(strings.TrimSpace(header), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || !strings.HasPrefix(value, auth.ServiceTokenPrefix) {
		return "", errors.New("bearer service token is required")
	}
	return value, nil
}
