package web

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/goformx/goforms/internal/application/constants"
	"github.com/goformx/goforms/internal/application/middleware/serviceauth"
	"github.com/goformx/goforms/internal/domain/site"
)

// SiteRepository is a separate capability so existing form mocks retain their contract.
type SiteRepository interface {
	CreateSite(context.Context, *site.Site) (*site.Site, bool, error)
	GetSite(context.Context, string, string) (*site.Site, error)
	ListSites(context.Context, string, int, int) ([]*site.Site, int64, error)
}

func siteResource(value *site.Site) map[string]any {
	return map[string]any{"id": value.ID, "organizationId": value.OrganizationID,
		"name": value.Name, "origin": value.Origin,
		"createdAt": value.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		"updatedAt": value.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z07:00")}
}

func (h *V1APIHandler) createSite(c echo.Context) error {
	var request struct {
		Name   string `json:"name"`
		Origin string `json:"origin"`
	}
	if err := decodeJSON(c, &request, mediaTypeJSON); err != nil {
		return h.writeRequestDecodeError(c, err, "")
	}
	principal, _ := serviceauth.PrincipalFrom(c)
	candidate, err := site.New(principal.OwnerID, request.Name, request.Origin)
	if errors.Is(err, site.ErrInvalid) {
		return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed", "Site metadata is invalid.", nil)
	}
	if err != nil {
		return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed", "Site metadata is invalid.", nil)
	}
	if h.sites == nil {
		return h.writeError(c, http.StatusServiceUnavailable, "service_unavailable", "Site management is unavailable.", nil)
	}
	value, created, err := h.sites.CreateSite(c.Request().Context(), candidate)
	if err != nil {
		return h.writeRepositoryError(c, err)
	}
	c.Response().Header().Set(echo.HeaderLocation, constants.PathV1Sites+"/"+value.ID)
	if created {
		return c.JSON(http.StatusCreated, map[string]any{"data": siteResource(value)})
	}
	return c.JSON(http.StatusOK, map[string]any{"data": siteResource(value)})
}

func (h *V1APIHandler) getSite(c echo.Context) error {
	if h.sites == nil {
		return h.writeError(c, http.StatusServiceUnavailable, "service_unavailable", "Site management is unavailable.", nil)
	}
	principal, _ := serviceauth.PrincipalFrom(c)
	value, err := h.sites.GetSite(c.Request().Context(), principal.OwnerID, c.Param("siteId"))
	if err != nil {
		return h.writeRepositoryError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": siteResource(value)})
}

func (h *V1APIHandler) listSites(c echo.Context) error {
	if h.sites == nil {
		return h.writeError(c, http.StatusServiceUnavailable, "service_unavailable", "Site management is unavailable.", nil)
	}
	limit, err := boundedInt(c.QueryParam("limit"), 25, 1, 100, "site page limit")
	if err != nil {
		return h.writeError(c, http.StatusBadRequest, "invalid_request", err.Error(), nil)
	}
	offset, err := boundedInt(c.QueryParam("offset"), 0, 0, 10000, "site page offset")
	if err != nil {
		return h.writeError(c, http.StatusBadRequest, "invalid_request", err.Error(), nil)
	}
	principal, _ := serviceauth.PrincipalFrom(c)
	values, total, err := h.sites.ListSites(c.Request().Context(), principal.OwnerID, limit, offset)
	if err != nil {
		return h.writeRepositoryError(c, err)
	}
	data := make([]map[string]any, 0, len(values))
	for _, value := range values {
		data = append(data, siteResource(value))
	}
	return c.JSON(http.StatusOK, map[string]any{"data": data, "meta": map[string]any{
		"limit": limit, "offset": offset, "total": total, "organizationId": principal.OwnerID,
	}})
}
