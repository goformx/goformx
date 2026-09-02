package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/goformx/goforms/internal/domain/managementaudit"
	domainwebhook "github.com/goformx/goforms/internal/domain/webhook"
)

func (h *V1APIHandler) patchWebhook(c echo.Context) error {
	formModel, ok := h.ownedForm(c)
	if !ok {
		return nil
	}
	if h.webhooks == nil {
		return h.writeWebhookMutationError(c, domainwebhook.ErrDisabled)
	}
	var fields map[string]json.RawMessage
	if err := decodeJSON(c, &fields, mediaTypeJSON); err != nil {
		return h.writeRequestDecodeError(c, err, "")
	}
	if len(fields) != 1 {
		return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed", domainwebhook.ErrInvalidChange.Error(), nil)
	}
	var change domainwebhook.EndpointChange
	for name, value := range fields {
		if name != "enabled" && name != "signingSecret" {
			return h.writeError(c, http.StatusBadRequest, "invalid_request", "Unknown webhook lifecycle field.", nil)
		}
		if strings.TrimSpace(string(value)) == "null" {
			return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed", domainwebhook.ErrInvalidChange.Error(), nil)
		}
		var err error
		if name == "enabled" {
			err = json.Unmarshal(value, &change.Enabled)
		} else {
			err = json.Unmarshal(value, &change.SigningSecret)
		}
		if err != nil {
			return h.writeError(c, http.StatusBadRequest, "invalid_request", "Invalid webhook lifecycle field type.", nil)
		}
	}
	if err := change.Validate(); err != nil {
		return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed", err.Error(), nil)
	}
	endpoint, err := h.webhooks.PatchWebhookEndpoint(c.Request().Context(), formModel.OrganizationID, formModel.ID, change, managementAuditActor(c))
	if err != nil {
		return h.writeWebhookMutationError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": webhookEndpointResource(endpoint)})
}

func (h *V1APIHandler) writeWebhookMutationError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, managementaudit.ErrUnavailable):
		return h.writeError(c, http.StatusServiceUnavailable, "management_audit_unavailable",
			"The change could not be durably audited; no webhook change was committed.", nil)
	case errors.Is(err, domainwebhook.ErrNotFound):
		return h.writeError(c, http.StatusNotFound, "not_found", "Webhook resource was not found.", nil)
	case errors.Is(err, domainwebhook.ErrDisabled):
		return h.writeError(c, http.StatusServiceUnavailable, "webhooks_disabled", "Webhook delivery is not configured on this service.", nil)
	default:
		return h.writeRepositoryError(c, err)
	}
}
