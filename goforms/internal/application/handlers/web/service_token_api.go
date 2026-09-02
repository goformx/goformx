package web

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/goformx/goforms/internal/application/constants"
	"github.com/goformx/goforms/internal/application/middleware/serviceauth"
	"github.com/goformx/goforms/internal/application/validation"
	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/domain/managementaudit"
)

const (
	defaultServiceTokenTTL = 30 * 24 * time.Hour
	minimumServiceTokenTTL = 5 * time.Minute
	maximumServiceTokenTTL = 365 * 24 * time.Hour
)

type createServiceTokenRequest struct {
	Name             string       `json:"name"`
	Scopes           []auth.Scope `json:"scopes"`
	ExpiresInSeconds int64        `json:"expiresInSeconds"`
}

func (h *V1APIHandler) listServiceTokens(c echo.Context) error {
	if h.tokens == nil {
		return h.writeError(c, http.StatusServiceUnavailable, "service_unavailable", "Token management is unavailable.", nil)
	}
	limit, err := submissionPageLimit(c.QueryParam("limit"))
	if err != nil {
		return h.writeError(c, http.StatusBadRequest, "invalid_request", err.Error(), nil)
	}
	principal, _ := serviceauth.PrincipalFrom(c)
	tokens, err := h.tokens.ListByOrganization(c.Request().Context(), principal.OwnerID, limit)
	if err != nil {
		return h.writeRepositoryError(c, err)
	}
	data := make([]map[string]any, 0, len(tokens))
	for _, token := range tokens {
		data = append(data, serviceTokenResource(token, time.Now().UTC()))
	}
	return c.JSON(http.StatusOK, map[string]any{
		"data": data,
		"meta": map[string]any{"limit": limit},
	})
}

func (h *V1APIHandler) createServiceToken(c echo.Context) error {
	if h.tokens == nil {
		return h.writeError(c, http.StatusServiceUnavailable, "service_unavailable", "Token management is unavailable.", nil)
	}
	var request createServiceTokenRequest
	if err := decodeJSON(c, &request, mediaTypeJSON); err != nil {
		return h.writeRequestDecodeError(c, err, "")
	}
	request.Name = strings.TrimSpace(request.Name)
	fieldErrors := validateServiceTokenRequest(request)
	principal, _ := serviceauth.PrincipalFrom(c)
	seen := make(map[auth.Scope]struct{}, len(request.Scopes))
	for index, scope := range request.Scopes {
		if _, duplicate := seen[scope]; duplicate {
			fieldErrors = append(fieldErrors, validation.Error{Pointer: "/scopes", Code: "uniqueItems", Message: "Scopes must not be repeated."})
			break
		}
		seen[scope] = struct{}{}
		if !scope.Valid() {
			fieldErrors = append(fieldErrors, validation.Error{Pointer: "/scopes/" + strconv.Itoa(index), Code: "enum", Message: "Scope is not part of the v1 contract."})
		} else if !principal.HasScope(scope) {
			fieldErrors = append(fieldErrors, validation.Error{Pointer: "/scopes/" + strconv.Itoa(index), Code: "scope_denied", Message: "The caller cannot delegate this scope."})
		}
	}
	if len(fieldErrors) > 0 {
		return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed", "Service token settings are invalid.", fieldErrors)
	}
	ttl := time.Duration(request.ExpiresInSeconds) * time.Second
	if ttl == 0 {
		ttl = defaultServiceTokenTTL
	}
	token, plaintext, err := auth.Issue(principal.OwnerID, request.Scopes, ttl, time.Now().UTC())
	if err != nil {
		return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed", err.Error(), nil)
	}
	token.Name = request.Name
	if err := h.tokens.Save(c.Request().Context(), token, managementAuditActor(c)); err != nil {
		return h.writeManagementMutationError(c, err)
	}
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	c.Response().Header().Set("Pragma", "no-cache")
	c.Response().Header().Set(echo.HeaderLocation, constants.PathV1ServiceTokens+"/"+token.ID)
	return c.JSON(http.StatusCreated, map[string]any{
		"data": map[string]any{"token": plaintext, "metadata": serviceTokenResource(token, time.Now().UTC())},
	})
}

func validateServiceTokenRequest(request createServiceTokenRequest) []validation.Error {
	var fieldErrors []validation.Error
	if len(request.Name) < 1 || len(request.Name) > 100 {
		fieldErrors = append(fieldErrors, validation.Error{Pointer: "/name", Code: "length", Message: "Name must be between 1 and 100 characters."})
	}
	if len(request.Scopes) == 0 {
		fieldErrors = append(fieldErrors, validation.Error{Pointer: "/scopes", Code: "minItems", Message: "At least one scope is required."})
	}
	if request.ExpiresInSeconds != 0 {
		if request.ExpiresInSeconds < int64(minimumServiceTokenTTL/time.Second) ||
			request.ExpiresInSeconds > int64(maximumServiceTokenTTL/time.Second) {
			fieldErrors = append(fieldErrors, validation.Error{Pointer: "/expiresInSeconds", Code: "range", Message: "Lifetime must be between 300 and 31536000 seconds."})
		}
	}
	return fieldErrors
}

func (h *V1APIHandler) revokeServiceToken(c echo.Context) error {
	if h.tokens == nil {
		return h.writeError(c, http.StatusServiceUnavailable, "service_unavailable", "Token management is unavailable.", nil)
	}
	principal, _ := serviceauth.PrincipalFrom(c)
	if err := h.tokens.RevokeByOrganization(c.Request().Context(), principal.OwnerID, c.Param("tokenId"), time.Now().UTC(), managementAuditActor(c)); err != nil {
		return h.writeManagementMutationError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func managementAuditActor(c echo.Context) auth.AuditActor {
	principal, _ := serviceauth.PrincipalFrom(c)
	return auth.AuditActor{OrganizationID: principal.OrganizationID, SubjectID: principal.SubjectID,
		CredentialClass: principal.CredentialClass, CredentialID: principal.CredentialID, RequestID: requestID(c)}
}

func (h *V1APIHandler) writeManagementMutationError(c echo.Context, err error) error {
	if errors.Is(err, managementaudit.ErrUnavailable) {
		return h.writeError(c, http.StatusServiceUnavailable, "management_audit_unavailable",
			"The change could not be durably audited; no credential change was committed.", nil)
	}
	return h.writeRepositoryError(c, err)
}

func serviceTokenResource(token *auth.ServiceToken, now time.Time) map[string]any {
	scopes := make([]string, 0, len(token.Scopes))
	for scope := range token.Scopes {
		scopes = append(scopes, string(scope))
	}
	sort.Strings(scopes)
	status := "active"
	if token.RevokedAt != nil {
		status = "revoked"
	} else if !now.Before(token.ExpiresAt) {
		status = "expired"
	}
	resource := map[string]any{
		"id": token.ID, "name": token.Name, "organizationId": token.OwnerID, "scopes": scopes, "status": status,
		"createdAt": token.CreatedAt.UTC().Format(time.RFC3339), "expiresAt": token.ExpiresAt.UTC().Format(time.RFC3339),
	}
	if token.LastUsedAt != nil {
		resource["lastUsedAt"] = token.LastUsedAt.UTC().Format(time.RFC3339)
	}
	if token.RevokedAt != nil {
		resource["revokedAt"] = token.RevokedAt.UTC().Format(time.RFC3339)
	}
	return resource
}
