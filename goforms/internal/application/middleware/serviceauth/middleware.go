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

type CredentialClass string

const (
	CredentialClassServiceToken        CredentialClass = "service_token"
	CredentialClassFirstPartyAssertion CredentialClass = "first_party_assertion"
)

type Repository interface {
	FindByID(ctx context.Context, tokenID string) (*auth.ServiceToken, error)
	MarkUsed(ctx context.Context, tokenID string, now time.Time) error
}

type Principal struct {
	CredentialClass CredentialClass
	CredentialID    string
	SubjectID       string
	OrganizationID  string
	RequestID       string
	TokenID         string // Deprecated compatibility alias for CredentialID.
	OwnerID         string // Deprecated compatibility alias for OrganizationID.
	Scopes          map[auth.Scope]struct{}
}

// HasScope reports whether the authenticated principal may perform or delegate a scope.
func (p Principal) HasScope(scope auth.Scope) bool {
	_, ok := p.Scopes[scope]
	return ok
}

type Middleware struct {
	repository Repository
	assertions AssertionVerifier
	now        func() time.Time
}

type AssertionVerifier interface {
	VerifyAndConsume(context.Context, string, time.Time) (auth.FirstPartyPrincipal, error)
}

func New(repository Repository) *Middleware {
	return &Middleware{repository: repository, now: time.Now}
}

func NewWithAssertions(repository Repository, assertions AssertionVerifier) *Middleware {
	return &Middleware{repository: repository, assertions: assertions, now: time.Now}
}

// Require authenticates a Bearer token and enforces one explicit control-plane scope.
func (m *Middleware) Require(scope auth.Scope) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			credential, err := bearerCredential(c.Request().Header.Get(echo.HeaderAuthorization))
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
			}
			if strings.HasPrefix(credential, auth.ServiceTokenPrefix) {
				return m.authenticateServiceToken(c, next, credential, scope)
			}
			if auth.IsFirstPartyAssertion(credential) {
				return m.authenticateFirstPartyAssertion(c, next, credential, scope)
			}
			return echo.NewHTTPError(http.StatusUnauthorized, "unsupported bearer credential")
		}
	}
}

func (m *Middleware) authenticateServiceToken(
	c echo.Context,
	next echo.HandlerFunc,
	plaintext string,
	scope auth.Scope,
) error {
	tokenID := auth.LookupID(plaintext)
	token, err := m.repository.FindByID(c.Request().Context(), tokenID)
	if err != nil || token == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid service token")
	}
	now := m.now()
	if err := token.Authenticate(plaintext, token.OwnerID, now); err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid service token")
	}
	if !token.HasScope(scope) {
		return echo.NewHTTPError(http.StatusForbidden, "service token is not authorized")
	}
	if err := m.repository.MarkUsed(c.Request().Context(), token.ID, now); err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "service token audit update failed")
	}
	scopes := make(map[auth.Scope]struct{}, len(token.Scopes))
	for granted := range token.Scopes {
		scopes[granted] = struct{}{}
	}
	c.Set(principalContextKey, Principal{CredentialClass: CredentialClassServiceToken,
		CredentialID: token.ID, SubjectID: token.ID, OrganizationID: token.OwnerID,
		TokenID: token.ID, OwnerID: token.OwnerID, Scopes: scopes})
	return next(c)
}

func (m *Middleware) authenticateFirstPartyAssertion(
	c echo.Context,
	next echo.HandlerFunc,
	compact string,
	scope auth.Scope,
) error {
	if m.assertions == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "first-party assertions are not configured")
	}
	verified, err := m.assertions.VerifyAndConsume(c.Request().Context(), compact, m.now())
	if err != nil {
		if errors.Is(err, auth.ErrFirstPartyAuthUnavailable) {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "first-party authentication is unavailable")
		}
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid first-party assertion")
	}
	if _, allowed := verified.Scopes[scope]; !allowed {
		return echo.NewHTTPError(http.StatusForbidden, "first-party assertion is not authorized")
	}
	c.Set(principalContextKey, Principal{CredentialClass: CredentialClassFirstPartyAssertion,
		CredentialID: verified.AssertionID, SubjectID: verified.SubjectID,
		OrganizationID: verified.OrganizationID, RequestID: verified.RequestID,
		TokenID: verified.AssertionID, OwnerID: verified.OrganizationID, Scopes: verified.Scopes})
	return next(c)
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

func bearerCredential(header string) (string, error) {
	scheme, value, found := strings.Cut(strings.TrimSpace(header), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || value == "" || strings.ContainsAny(value, " \t\r\n") {
		return "", errors.New("bearer credential is required")
	}
	return value, nil
}
