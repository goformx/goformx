//go:generate go tool mockgen -typed -source=v1_api.go -destination=../../../../test/mocks/form/mock_repository.go -package=form -mock_names=V1Repository=MockRepository,WebhookRepository=MockWebhookRepository,RequestLogger=MockRequestLogger

package web

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/net/http/httpguts"

	"github.com/goformx/goforms/internal/application/constants"
	"github.com/goformx/goforms/internal/application/middleware/serviceauth"
	"github.com/goformx/goforms/internal/application/validation"
	deliveryapp "github.com/goformx/goforms/internal/application/webhook"
	"github.com/goformx/goforms/internal/domain/auth"
	domainform "github.com/goformx/goforms/internal/domain/form"
	"github.com/goformx/goforms/internal/domain/form/model"
	domainwebhook "github.com/goformx/goforms/internal/domain/webhook"
)

var (
	formNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,62}$`)
	traceIDPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)

// V1APIHandler implements the schema-first control and public data planes.
type V1APIHandler struct {
	repository   V1Repository
	auth         *serviceauth.Middleware
	validator    *validation.ComprehensiveValidator
	admission    *submissionLimiter
	webhooks     WebhookRepository
	tokens       ServiceTokenManagementRepository
	destinations *deliveryapp.DestinationPolicy
	logger       RequestLogger
	requests     atomic.Uint64
	errors       atomic.Uint64
}

// V1Repository is the persistence boundary used by the schema-first API.
// It intentionally excludes legacy users, plans, pagination, and browser sessions.
type V1Repository interface {
	CreateForm(context.Context, *model.Form) error
	ListForms(context.Context, string, model.FormListOptions) ([]*model.Form, int64, error)
	GetFormByID(context.Context, string, string) (*model.Form, error)
	UpdateForm(context.Context, *model.Form, time.Time) error
	CreateSchemaVersion(context.Context, string, model.JSON) (*model.SchemaVersion, error)
	GetSchemaVersion(context.Context, string, int) (*model.SchemaVersion, error)
	PublishSchemaVersion(context.Context, string, int) (*model.SchemaVersion, error)
	ListSubmissionsPage(context.Context, string, time.Time, string, int) ([]*model.FormSubmission, bool, error)
	GetSubmissionByOrganization(context.Context, string, string, string) (*model.FormSubmission, error)
	GetPublishedSchemaVersion(context.Context, string, int) (*model.Form, *model.SchemaVersion, error)
	CreateSubmissionIdempotent(context.Context, *model.FormSubmission) (*model.FormSubmission, bool, error)
}

type WebhookRepository interface {
	PutWebhookEndpoint(context.Context, string, string, domainwebhook.SecretConfig, bool) (*domainwebhook.Endpoint, error)
	GetWebhookEndpoint(context.Context, string) (*domainwebhook.Endpoint, error)
	DeleteWebhookEndpoint(context.Context, string) error
	ListWebhookDeliveries(context.Context, string, int) ([]*domainwebhook.Delivery, error)
	ReplayWebhookDelivery(context.Context, string, string) error
}

// ServiceTokenManagementRepository exposes organization-scoped metadata and lifecycle operations.
type ServiceTokenManagementRepository interface {
	Save(context.Context, *auth.ServiceToken) error
	ListByOrganization(context.Context, string, int) ([]*auth.ServiceToken, error)
	RevokeByOrganization(context.Context, string, string, time.Time) error
}

// RequestLogger keeps the HTTP application boundary independent of logging implementations.
type RequestLogger interface {
	Info(string, ...any)
	Error(string, ...any)
}

// NewV1APIHandler wires the v1 API to the canonical repository and scoped service-token store.
func NewV1APIHandler(
	repository V1Repository,
	tokens serviceauth.Repository,
	logger RequestLogger,
) *V1APIHandler {
	return NewV1APIHandlerWithLimits(repository, tokens, logger, DefaultV1Limits())
}

// NewV1APIHandlerWithLimits wires the v1 API with explicit public-write admission limits.
func NewV1APIHandlerWithLimits(
	repository V1Repository,
	tokens serviceauth.Repository,
	logger RequestLogger,
	limits V1Limits,
) *V1APIHandler {
	return newV1APIHandlerWithLimits(repository, tokens, validation.NewComprehensiveValidator(), logger, limits)
}

func newV1APIHandler(
	repository V1Repository,
	tokens serviceauth.Repository,
	validator *validation.ComprehensiveValidator,
	logger RequestLogger,
) *V1APIHandler {
	return newV1APIHandlerWithLimits(repository, tokens, validator, logger, DefaultV1Limits())
}

func newV1APIHandlerWithLimits(
	repository V1Repository,
	tokens serviceauth.Repository,
	validator *validation.ComprehensiveValidator,
	logger RequestLogger,
	limits V1Limits,
) *V1APIHandler {
	var webhooks WebhookRepository
	if candidate, ok := repository.(WebhookRepository); ok {
		webhooks = candidate
	}
	var tokenManagement ServiceTokenManagementRepository
	if candidate, ok := tokens.(ServiceTokenManagementRepository); ok {
		tokenManagement = candidate
	}
	return &V1APIHandler{repository: repository, auth: serviceauth.New(tokens), validator: validator,
		admission: newSubmissionLimiter(limits), webhooks: webhooks, tokens: tokenManagement,
		destinations: deliveryapp.NewDestinationPolicy(nil), logger: logger}
}

func (h *V1APIHandler) RegisterRoutes(e *echo.Echo) {
	control := e.Group(constants.PathV1Forms)
	control.GET("", h.instrument("list_forms", h.listForms), h.require(auth.ScopeFormsRead))
	control.POST("", h.instrument("create_form", h.createForm), h.require(auth.ScopeFormsWrite))
	control.GET("/:formId", h.instrument("get_form", h.getForm), h.require(auth.ScopeFormsRead))
	control.PATCH("/:formId", h.instrument("update_form", h.updateForm), h.require(auth.ScopeFormsWrite))
	control.POST("/:formId/versions", h.instrument("create_schema_version", h.createSchemaVersion), h.require(auth.ScopeFormsWrite))
	control.POST("/:formId/versions/:version/publish", h.instrument("publish_schema_version", h.publishSchemaVersion), h.require(auth.ScopeFormsPublish))
	control.GET("/:formId/submissions", h.instrument("list_submissions", h.listSubmissions), h.require(auth.ScopeSubmissionsRead))
	control.GET("/:formId/submissions/:submissionId", h.instrument("get_submission", h.getSubmission), h.require(auth.ScopeSubmissionsRead))
	control.PUT("/:formId/webhook", h.instrument("put_webhook", h.putWebhook), h.require(auth.ScopeWebhooksWrite))
	control.GET("/:formId/webhook", h.instrument("get_webhook", h.getWebhook), h.require(auth.ScopeWebhooksRead))
	control.DELETE("/:formId/webhook", h.instrument("delete_webhook", h.deleteWebhook), h.require(auth.ScopeWebhooksWrite))
	control.GET("/:formId/deliveries", h.instrument("list_deliveries", h.listWebhookDeliveries), h.require(auth.ScopeSubmissionsRead))
	control.POST("/:formId/deliveries/:deliveryId/replay", h.instrument("replay_delivery", h.replayWebhookDelivery),
		h.require(auth.ScopeWebhooksWrite))

	tokens := e.Group(constants.PathV1ServiceTokens)
	tokens.GET("", h.instrument("list_service_tokens", h.listServiceTokens), h.require(auth.ScopeTokensRead))
	tokens.POST("", h.instrument("create_service_token", h.createServiceToken), h.require(auth.ScopeTokensWrite))
	tokens.DELETE("/:tokenId", h.instrument("revoke_service_token", h.revokeServiceToken), h.require(auth.ScopeTokensWrite))

	public := e.Group(constants.PathV1PublicForms)
	public.Use(h.publicCORS())
	public.OPTIONS("/:publicKey/schema", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	public.OPTIONS("/:publicKey/submissions", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	public.GET("/:publicKey/schema", h.instrument("get_published_schema", h.getPublishedSchema))
	public.POST("/:publicKey/submissions", h.instrument("create_submission", h.createSubmission))
}

func (h *V1APIHandler) instrument(operation string, next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		started := time.Now()
		requestID(c)
		h.requests.Add(1)
		err := next(c)
		status := c.Response().Status
		if status >= http.StatusBadRequest || err != nil {
			h.errors.Add(1)
		}
		if h.logger != nil {
			h.logger.Info("v1 API request", "operation", operation, "request_id", requestID(c),
				"status", status, "duration_ms", time.Since(started).Milliseconds(),
				"requests_total", h.requests.Load(), "errors_total", h.errors.Load())
		}
		return err
	}
}

func (h *V1APIHandler) publicCORS() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			origin := strings.TrimSpace(c.Request().Header.Get(echo.HeaderOrigin))
			if origin == "" {
				return next(c)
			}
			formModel, _, err := h.repository.GetPublishedSchemaVersion(
				c.Request().Context(), c.Param("publicKey"), 0,
			)
			if err != nil {
				return h.writeError(c, http.StatusNotFound, "not_found", "Published form schema was not found.", nil)
			}
			origins, methods, headers := formModel.GetCorsConfig()
			allowed := false
			for _, candidate := range origins {
				if candidate == origin {
					allowed = true
					break
				}
			}
			if !allowed {
				return h.writeError(c, http.StatusForbidden, "origin_denied", "This origin is not allowed to use the form.", nil)
			}
			responseHeaders := c.Response().Header()
			responseHeaders.Set(echo.HeaderAccessControlAllowOrigin, origin)
			responseHeaders.Set(echo.HeaderVary, echo.HeaderOrigin)
			responseHeaders.Set(echo.HeaderAccessControlAllowMethods, strings.Join(methods, ", "))
			responseHeaders.Set(echo.HeaderAccessControlAllowHeaders, strings.Join(headers, ", "))
			responseHeaders.Set(echo.HeaderAccessControlExposeHeaders,
				"ETag, "+constants.HeaderSchemaVersion+", "+constants.HeaderTraceID+", "+constants.HeaderReplay)
			if c.Request().Method == http.MethodOptions {
				return c.NoContent(http.StatusNoContent)
			}
			return next(c)
		}
	}
}

func (h *V1APIHandler) require(scope auth.Scope) echo.MiddlewareFunc {
	inner := h.auth.Require(scope)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		secured := inner(next)
		return func(c echo.Context) error {
			if err := secured(c); err != nil {
				status := http.StatusUnauthorized
				code := "unauthorized"
				message := "Authentication is required."
				var httpErr *echo.HTTPError
				if errors.As(err, &httpErr) {
					status = httpErr.Code
					message = fmt.Sprint(httpErr.Message)
				}
				switch status {
				case http.StatusForbidden:
					code = "forbidden"
				case http.StatusServiceUnavailable:
					code = "service_unavailable"
				}
				return h.writeError(c, status, code, message, nil)
			}
			return nil
		}
	}
}

type createFormRequest struct {
	Name           string     `json:"name"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Schema         model.JSON `json:"schema"`
	AllowedOrigins []string   `json:"allowedOrigins"`
}

func (h *V1APIHandler) createForm(c echo.Context) error {
	var request createFormRequest
	if err := decodeJSON(c, &request); err != nil {
		return h.writeError(c, http.StatusBadRequest, "invalid_request", err.Error(), nil)
	}
	if !formNamePattern.MatchString(request.Name) {
		return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed", "Form metadata is invalid.",
			[]validation.Error{{Pointer: "/name", Code: "pattern", Message: "Must be a lowercase slug of 3 to 63 characters."}})
	}
	if len(request.Title) < model.MinTitleLength || len(request.Title) > model.MaxTitleLength {
		return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed", "Form metadata is invalid.",
			[]validation.Error{{Pointer: "/title", Code: "length", Message: "Title must be between 3 and 100 characters."}})
	}
	if originErrors := validateOrigins(request.AllowedOrigins); len(originErrors) > 0 {
		return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed", "Allowed origins are invalid.", originErrors)
	}
	if result := h.validator.ValidateSchema(request.Schema); !result.IsValid {
		return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed", "Form schema is invalid.", prefixErrors(result.Errors, "/schema"))
	}
	principal, _ := serviceauth.PrincipalFrom(c)
	formModel := model.NewForm(principal.OwnerID, request.Title, request.Description, request.Schema)
	formModel.Name = request.Name
	formModel.SetCorsConfig(request.AllowedOrigins, []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		[]string{echo.HeaderContentType, constants.HeaderIdempotencyKey, constants.HeaderSchemaVersion})
	if err := formModel.Validate(); err != nil {
		return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed", err.Error(), nil)
	}
	if err := h.repository.CreateForm(c.Request().Context(), formModel); err != nil {
		return h.writeRepositoryError(c, err)
	}
	c.Response().Header().Set(constants.HeaderETag, formETag(formModel))
	c.Response().Header().Set(echo.HeaderLocation, constants.PathV1Forms+"/"+formModel.ID)
	return c.JSON(http.StatusCreated, map[string]any{"data": formResource(formModel)})
}

func (h *V1APIHandler) listForms(c echo.Context) error {
	options, err := formListOptions(c)
	if err != nil {
		return h.writeError(c, http.StatusBadRequest, "invalid_request", err.Error(), nil)
	}
	principal, _ := serviceauth.PrincipalFrom(c)
	forms, total, err := h.repository.ListForms(c.Request().Context(), principal.OwnerID, options)
	if err != nil {
		return h.writeRepositoryError(c, err)
	}
	data := make([]map[string]any, 0, len(forms))
	for _, formModel := range forms {
		data = append(data, formResource(formModel))
	}
	return c.JSON(http.StatusOK, map[string]any{"data": data, "meta": map[string]any{
		"limit": options.Limit, "offset": options.Offset, "total": total,
	}})
}

func (h *V1APIHandler) ownedForm(c echo.Context) (*model.Form, bool) {
	principal, ok := serviceauth.PrincipalFrom(c)
	if !ok {
		_ = h.writeError(c, http.StatusUnauthorized, "unauthorized", "Authentication is required.", nil)
		return nil, false
	}
	formModel, err := h.repository.GetFormByID(c.Request().Context(), principal.OwnerID, c.Param("formId"))
	if err != nil {
		_ = h.writeRepositoryError(c, err)
		return nil, false
	}
	return formModel, true
}

func (h *V1APIHandler) getForm(c echo.Context) error {
	formModel, ok := h.ownedForm(c)
	if !ok {
		return nil
	}
	c.Response().Header().Set(constants.HeaderETag, formETag(formModel))
	return c.JSON(http.StatusOK, map[string]any{"data": formResource(formModel)})
}

type updateFormRequest struct {
	Title          *string   `json:"title"`
	Description    *string   `json:"description"`
	AllowedOrigins *[]string `json:"allowedOrigins"`
}

func (h *V1APIHandler) updateForm(c echo.Context) error {
	formModel, ok := h.ownedForm(c)
	if !ok {
		return nil
	}
	expected := formModel.UpdatedAt
	ifMatch := strings.TrimSpace(c.Request().Header.Get(constants.HeaderIfMatch))
	if ifMatch == "" {
		return h.writeError(c, http.StatusPreconditionRequired, "precondition_required", "If-Match is required for form updates.", nil)
	}
	if ifMatch != formETag(formModel) {
		return h.writeError(c, http.StatusPreconditionFailed, "precondition_failed", "The form was modified by another request.", nil)
	}
	var request updateFormRequest
	if err := decodeJSON(c, &request); err != nil {
		return h.writeError(c, http.StatusBadRequest, "invalid_request", err.Error(), nil)
	}
	if request.Title == nil && request.Description == nil && request.AllowedOrigins == nil {
		return h.writeError(c, http.StatusBadRequest, "invalid_request", "At least one metadata field is required.", nil)
	}
	if request.Title != nil {
		if len(*request.Title) < model.MinTitleLength || len(*request.Title) > model.MaxTitleLength {
			return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed", "Title is invalid.", nil)
		}
		formModel.Title = *request.Title
	}
	if request.Description != nil {
		formModel.Description = *request.Description
	}
	if request.AllowedOrigins != nil {
		if originErrors := validateOrigins(*request.AllowedOrigins); len(originErrors) > 0 {
			return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed", "Allowed origins are invalid.", originErrors)
		}
		formModel.SetCorsConfig(*request.AllowedOrigins, []string{http.MethodGet, http.MethodPost, http.MethodOptions},
			[]string{echo.HeaderContentType, constants.HeaderIdempotencyKey, constants.HeaderSchemaVersion})
	}
	formModel.Schema = nil // Metadata updates must never create a schema version.
	if err := h.repository.UpdateForm(c.Request().Context(), formModel, expected); err != nil {
		return h.writeRepositoryError(c, err)
	}
	updated, err := h.repository.GetFormByID(c.Request().Context(), formModel.OrganizationID, formModel.ID)
	if err != nil {
		return h.writeRepositoryError(c, err)
	}
	c.Response().Header().Set(constants.HeaderETag, formETag(updated))
	return c.JSON(http.StatusOK, map[string]any{"data": formResource(updated)})
}

type createVersionRequest struct {
	Schema model.JSON `json:"schema"`
}

func (h *V1APIHandler) createSchemaVersion(c echo.Context) error {
	formModel, ok := h.ownedForm(c)
	if !ok {
		return nil
	}
	var request createVersionRequest
	if err := decodeJSON(c, &request); err != nil {
		return h.writeError(c, http.StatusBadRequest, "invalid_request", err.Error(), nil)
	}
	if result := h.validator.ValidateSchema(request.Schema); !result.IsValid {
		return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed", "Form schema is invalid.", prefixErrors(result.Errors, "/schema"))
	}
	version, err := h.repository.CreateSchemaVersion(c.Request().Context(), formModel.ID, request.Schema)
	if err != nil {
		return h.writeRepositoryError(c, err)
	}
	return c.JSON(http.StatusCreated, map[string]any{"data": schemaVersionResource(version)})
}

func (h *V1APIHandler) publishSchemaVersion(c echo.Context) error {
	formModel, ok := h.ownedForm(c)
	if !ok {
		return nil
	}
	versionNumber, err := positiveInt(c.Param("version"))
	if err != nil {
		return h.writeError(c, http.StatusBadRequest, "invalid_request", "Schema version must be a positive integer.", nil)
	}
	version, err := h.repository.GetSchemaVersion(c.Request().Context(), formModel.ID, versionNumber)
	if err != nil {
		return h.writeRepositoryError(c, err)
	}
	if result := h.validator.ValidateSchema(version.Schema()); !result.IsValid {
		return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed", "Form schema is invalid.", prefixErrors(result.Errors, "/schema"))
	}
	published, err := h.repository.PublishSchemaVersion(c.Request().Context(), formModel.ID, versionNumber)
	if err != nil {
		return h.writeRepositoryError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": schemaVersionResource(published)})
}

func (h *V1APIHandler) listSubmissions(c echo.Context) error {
	formModel, ok := h.ownedForm(c)
	if !ok {
		return nil
	}
	limit, err := submissionPageLimit(c.QueryParam("limit"))
	if err != nil {
		return h.writeError(c, http.StatusBadRequest, "invalid_request", err.Error(), nil)
	}
	before, beforeID, err := decodeSubmissionCursor(c.QueryParam("cursor"))
	if err != nil {
		return h.writeError(c, http.StatusBadRequest, "invalid_request", err.Error(), nil)
	}
	submissions, hasMore, err := h.repository.ListSubmissionsPage(
		c.Request().Context(), formModel.ID, before, beforeID, limit,
	)
	if err != nil {
		return h.writeRepositoryError(c, err)
	}
	data := make([]map[string]any, 0, len(submissions))
	for _, submission := range submissions {
		data = append(data, submissionResource(submission))
	}
	var nextCursor any
	if hasMore && len(submissions) > 0 {
		nextCursor = encodeSubmissionCursor(submissions[len(submissions)-1])
	}
	return c.JSON(http.StatusOK, map[string]any{"data": data, "meta": map[string]any{
		"limit": limit, "nextCursor": nextCursor,
	}})
}

func (h *V1APIHandler) getSubmission(c echo.Context) error {
	principal, _ := serviceauth.PrincipalFrom(c)
	submission, err := h.repository.GetSubmissionByOrganization(
		c.Request().Context(), principal.OwnerID, c.Param("formId"), c.Param("submissionId"),
	)
	if err != nil {
		return h.writeRepositoryError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": submissionResource(submission)})
}

type putWebhookRequest struct {
	URL           string            `json:"url"`
	Headers       map[string]string `json:"headers"`
	SigningSecret string            `json:"signingSecret"`
	Enabled       *bool             `json:"enabled"`
}

func (h *V1APIHandler) putWebhook(c echo.Context) error {
	formModel, ok := h.ownedForm(c)
	if !ok {
		return nil
	}
	if h.webhooks == nil {
		return h.writeError(c, http.StatusServiceUnavailable, "webhooks_disabled",
			"Webhook delivery is not configured on this service.", nil)
	}
	var request putWebhookRequest
	if err := decodeJSON(c, &request); err != nil {
		return h.writeError(c, http.StatusBadRequest, "invalid_request", err.Error(), nil)
	}
	target, err := h.destinations.Validate(c.Request().Context(), request.URL)
	if err != nil {
		return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed", "Webhook destination is invalid.",
			[]validation.Error{{Pointer: "/url", Code: "unsafe_destination", Message: err.Error()}})
	}
	if len(request.SigningSecret) < 32 || len(request.SigningSecret) > 256 {
		return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed", "Webhook secret is invalid.",
			[]validation.Error{{Pointer: "/signingSecret", Code: "length",
				Message: "Signing secret must contain between 32 and 256 characters."}})
	}
	if fieldError := validateWebhookHeaders(request.Headers); fieldError != nil {
		return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed",
			"Webhook headers are invalid.", []validation.Error{*fieldError})
	}
	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	endpoint, err := h.webhooks.PutWebhookEndpoint(c.Request().Context(), formModel.ID, target.String(),
		domainwebhook.SecretConfig{DestinationURL: target.String(),
			Headers: request.Headers, SigningSecret: request.SigningSecret}, enabled)
	if err != nil {
		if errors.Is(err, domainwebhook.ErrDisabled) {
			return h.writeError(c, http.StatusServiceUnavailable, "webhooks_disabled",
				"Webhook delivery is not configured on this service.", nil)
		}
		return h.writeRepositoryError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": webhookEndpointResource(endpoint)})
}

func validateWebhookHeaders(headers map[string]string) *validation.Error {
	if len(headers) > 20 {
		return &validation.Error{Pointer: "/headers", Code: "maxProperties",
			Message: "At most 20 custom headers are allowed."}
	}
	reserved := map[string]struct{}{
		"content-length": {}, "content-type": {}, "connection": {}, "host": {}, "transfer-encoding": {},
		"user-agent": {}, strings.ToLower(deliveryapp.HeaderDeliveryID): {},
		strings.ToLower(deliveryapp.HeaderTimestamp): {}, strings.ToLower(deliveryapp.HeaderSignature): {},
	}
	total := 0
	for name, value := range headers {
		total += len(name) + len(value)
		if len(name) > 64 || !httpguts.ValidHeaderFieldName(name) {
			return &validation.Error{Pointer: "/headers", Code: "invalid_header",
				Message: "Custom header names must be valid and at most 64 bytes."}
		}
		if _, denied := reserved[strings.ToLower(name)]; denied {
			return &validation.Error{Pointer: "/headers/" + escapeJSONPointer(name), Code: "reserved_header",
				Message: "This header is controlled by GoFormX."}
		}
		if len(value) > 1024 || !httpguts.ValidHeaderFieldValue(value) {
			return &validation.Error{Pointer: "/headers/" + escapeJSONPointer(name), Code: "invalid_header",
				Message: "Custom header values must be valid and at most 1024 bytes."}
		}
	}
	if total > 8192 {
		return &validation.Error{Pointer: "/headers", Code: "maxLength",
			Message: "Custom headers must not exceed 8192 bytes in total."}
	}
	return nil
}

func escapeJSONPointer(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

func (h *V1APIHandler) getWebhook(c echo.Context) error {
	formModel, ok := h.ownedForm(c)
	if !ok {
		return nil
	}
	if h.webhooks == nil {
		return h.writeError(c, http.StatusServiceUnavailable, "webhooks_disabled",
			"Webhook delivery is not configured on this service.", nil)
	}
	endpoint, err := h.webhooks.GetWebhookEndpoint(c.Request().Context(), formModel.ID)
	if errors.Is(err, domainwebhook.ErrNotFound) {
		return h.writeError(c, http.StatusNotFound, "not_found", "Webhook endpoint was not found.", nil)
	}
	if err != nil {
		return h.writeRepositoryError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]any{"data": webhookEndpointResource(endpoint)})
}

func (h *V1APIHandler) deleteWebhook(c echo.Context) error {
	formModel, ok := h.ownedForm(c)
	if !ok {
		return nil
	}
	if h.webhooks == nil {
		return h.writeError(c, http.StatusServiceUnavailable, "webhooks_disabled",
			"Webhook delivery is not configured on this service.", nil)
	}
	if err := h.webhooks.DeleteWebhookEndpoint(c.Request().Context(), formModel.ID); err != nil {
		if errors.Is(err, domainwebhook.ErrNotFound) {
			return h.writeError(c, http.StatusNotFound, "not_found", "Webhook endpoint was not found.", nil)
		}
		return h.writeRepositoryError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *V1APIHandler) listWebhookDeliveries(c echo.Context) error {
	formModel, ok := h.ownedForm(c)
	if !ok {
		return nil
	}
	if h.webhooks == nil {
		return h.writeError(c, http.StatusServiceUnavailable, "webhooks_disabled",
			"Webhook delivery is not configured on this service.", nil)
	}
	limit, err := submissionPageLimit(c.QueryParam("limit"))
	if err != nil {
		return h.writeError(c, http.StatusBadRequest, "invalid_request", err.Error(), nil)
	}
	deliveries, err := h.webhooks.ListWebhookDeliveries(c.Request().Context(), formModel.ID, limit)
	if err != nil {
		return h.writeRepositoryError(c, err)
	}
	data := make([]map[string]any, 0, len(deliveries))
	for _, delivery := range deliveries {
		data = append(data, webhookDeliveryResource(delivery))
	}
	return c.JSON(http.StatusOK, map[string]any{"data": data, "meta": map[string]any{"limit": limit}})
}

func (h *V1APIHandler) replayWebhookDelivery(c echo.Context) error {
	formModel, ok := h.ownedForm(c)
	if !ok {
		return nil
	}
	if h.webhooks == nil {
		return h.writeError(c, http.StatusServiceUnavailable, "webhooks_disabled",
			"Webhook delivery is not configured on this service.", nil)
	}
	deliveryID := c.Param("deliveryId")
	if _, err := uuid.Parse(deliveryID); err != nil {
		return h.writeError(c, http.StatusNotFound, "not_found", "Webhook delivery was not found.", nil)
	}
	if err := h.webhooks.ReplayWebhookDelivery(c.Request().Context(), formModel.ID, deliveryID); err != nil {
		if errors.Is(err, domainwebhook.ErrNotFound) {
			return h.writeError(c, http.StatusNotFound, "not_found", "Webhook delivery was not found.", nil)
		}
		return h.writeRepositoryError(c, err)
	}
	return c.JSON(http.StatusAccepted, map[string]any{"data": map[string]string{
		"id": deliveryID, "status": string(domainwebhook.DeliveryPending),
	}})
}

func webhookEndpointResource(endpoint *domainwebhook.Endpoint) map[string]any {
	return map[string]any{"id": endpoint.ID, "formId": endpoint.FormID, "origin": endpoint.Origin,
		"enabled": endpoint.Enabled, "createdAt": endpoint.CreatedAt, "updatedAt": endpoint.UpdatedAt}
}

func webhookDeliveryResource(delivery *domainwebhook.Delivery) map[string]any {
	return map[string]any{"id": delivery.ID, "submissionId": delivery.SubmissionID,
		"status": delivery.Status, "attemptCount": delivery.AttemptCount,
		"nextAttemptAt": delivery.NextAttemptAt, "deliveredAt": delivery.DeliveredAt,
		"lastHttpStatus": delivery.LastHTTPStatus, "lastErrorCategory": delivery.LastErrorCategory,
		"createdAt": delivery.CreatedAt, "updatedAt": delivery.UpdatedAt}
}

const (
	defaultSubmissionPageSize = 25
	maxSubmissionPageSize     = 100
)

type submissionCursor struct {
	SubmittedAt time.Time `json:"submittedAt"`
	ID          string    `json:"id"`
}

func submissionPageLimit(value string) (int, error) {
	if value == "" {
		return defaultSubmissionPageSize, nil
	}
	limit, err := positiveInt(value)
	if err != nil || limit > maxSubmissionPageSize {
		return 0, fmt.Errorf("limit must be between 1 and %d", maxSubmissionPageSize)
	}
	return limit, nil
}

func decodeSubmissionCursor(value string) (time.Time, string, error) {
	if value == "" {
		return time.Time{}, "", nil
	}
	encoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return time.Time{}, "", errors.New("cursor is invalid")
	}
	var cursor submissionCursor
	if err := json.Unmarshal(encoded, &cursor); err != nil || cursor.SubmittedAt.IsZero() {
		return time.Time{}, "", errors.New("cursor is invalid")
	}
	if _, err := uuid.Parse(cursor.ID); err != nil {
		return time.Time{}, "", errors.New("cursor is invalid")
	}
	return cursor.SubmittedAt.UTC(), cursor.ID, nil
}

func encodeSubmissionCursor(submission *model.FormSubmission) string {
	encoded, _ := json.Marshal(submissionCursor{SubmittedAt: submission.SubmittedAt.UTC(), ID: submission.ID})
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func (h *V1APIHandler) getPublishedSchema(c echo.Context) error {
	version := 0
	var err error
	if value := c.QueryParam("version"); value != "" {
		version, err = positiveInt(value)
		if err != nil {
			return h.writeError(c, http.StatusBadRequest, "invalid_request", "Schema version must be a positive integer.", nil)
		}
	}
	_, schemaVersion, err := h.repository.GetPublishedSchemaVersion(c.Request().Context(), c.Param("publicKey"), version)
	if err != nil {
		return h.writeError(c, http.StatusNotFound, "not_found", "Published form schema was not found.", nil)
	}
	schema := schemaVersion.Schema()
	digest, _ := json.Marshal(schema)
	hash := sha256.Sum256(digest)
	etag := `"` + base64.RawURLEncoding.EncodeToString(hash[:]) + `"`
	c.Response().Header().Set("ETag", etag)
	c.Response().Header().Set(constants.HeaderSchemaVersion, strconv.Itoa(schemaVersion.Version()))
	c.Response().Header().Set(echo.HeaderContentType, constants.ContentTypeJSONSchema)
	return c.JSONBlob(http.StatusOK, digest)
}

type submissionRequest struct {
	Data model.JSON `json:"data"`
}

func (h *V1APIHandler) createSubmission(c echo.Context) error {
	idempotencyKey, ok := h.requireIdempotencyKey(c)
	if !ok {
		return nil
	}
	version := 0
	var err error
	if value := c.Request().Header.Get(constants.HeaderSchemaVersion); value != "" {
		version, err = positiveInt(value)
		if err != nil {
			return h.writeError(c, http.StatusBadRequest, "invalid_request", "Schema version must be a positive integer.", nil)
		}
	}
	formModel, schemaVersion, err := h.repository.GetPublishedSchemaVersion(c.Request().Context(), c.Param("publicKey"), version)
	if err != nil {
		return h.writeError(c, http.StatusNotFound, "not_found", "Published form schema was not found.", nil)
	}
	if !h.admission.allow(formModel.ID) {
		c.Response().Header().Set(echo.HeaderRetryAfter, "1")
		return h.writeError(c, http.StatusTooManyRequests, "rate_limited", "This form is receiving too many submissions.", nil)
	}
	var request submissionRequest
	if err := decodeJSON(c, &request); err != nil {
		return h.writeError(c, http.StatusBadRequest, "invalid_request", err.Error(), nil)
	}
	result := h.validator.ValidateVersion(schemaVersion, request.Data)
	if !result.IsValid {
		return h.writeError(c, http.StatusUnprocessableEntity, "validation_failed",
			fmt.Sprintf("Submission does not match schema version %d.", schemaVersion.Version()), prefixErrors(result.Errors, "/data"))
	}
	now := time.Now().UTC()
	submission := &model.FormSubmission{FormID: formModel.ID, SchemaVersion: schemaVersion.Version(),
		RequestID: requestID(c), IdempotencyKey: idempotencyKey, Data: request.Data, SubmittedAt: now,
		Status: model.SubmissionStatusAccepted, Metadata: model.JSON{}}
	stored, replayed, err := h.repository.CreateSubmissionIdempotent(c.Request().Context(), submission)
	if err != nil {
		if errors.Is(err, domainform.ErrSubmissionLimitExceeded) {
			c.Response().Header().Set(echo.HeaderRetryAfter, "86400")
			return h.writeError(c, http.StatusTooManyRequests, "submission_limit_reached",
				"This form has reached its rolling submission limit.", nil)
		}
		return h.writeRepositoryError(c, err)
	}
	if replayed && (stored.SchemaVersion != submission.SchemaVersion || !reflect.DeepEqual(stored.Data, submission.Data)) {
		return h.writeError(c, http.StatusConflict, "idempotency_conflict",
			"The idempotency key was already used with a different submission.", nil)
	}
	if replayed {
		c.Response().Header().Set(constants.HeaderReplay, "true")
	}
	return c.JSON(http.StatusAccepted, map[string]any{"data": submissionResource(stored)})
}

func (h *V1APIHandler) requireIdempotencyKey(c echo.Context) (string, bool) {
	key := strings.TrimSpace(c.Request().Header.Get(constants.HeaderIdempotencyKey))
	if len(key) < 16 || len(key) > 128 {
		_ = h.writeError(c, http.StatusBadRequest, "invalid_idempotency_key",
			"Idempotency-Key must contain between 16 and 128 characters.", nil)
		return "", false
	}
	return key, true
}

func decodeJSON(c echo.Context, destination any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(c.Response(), c.Request().Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("request body must be valid JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func positiveInt(value string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return 0, errors.New("positive integer required")
	}
	return parsed, nil
}

func prefixErrors(items []validation.Error, prefix string) []validation.Error {
	prefixed := make([]validation.Error, len(items))
	for index, item := range items {
		pointer := item.Pointer
		if pointer == "/" {
			pointer = ""
		}
		prefixed[index] = validation.Error{Pointer: prefix + pointer, Code: item.Code, Message: item.Message}
	}
	return prefixed
}

func validateOrigins(origins []string) []validation.Error {
	seen := make(map[string]struct{}, len(origins))
	var fieldErrors []validation.Error
	for index, origin := range origins {
		parsed, err := url.Parse(origin)
		pointer := fmt.Sprintf("/allowedOrigins/%d", index)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" ||
			parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
			fieldErrors = append(fieldErrors, validation.Error{Pointer: pointer, Code: "format",
				Message: "Must be an http or https origin without a path, query, credentials, or fragment."})
			continue
		}
		canonical := parsed.Scheme + "://" + parsed.Host
		if _, duplicate := seen[canonical]; duplicate {
			fieldErrors = append(fieldErrors, validation.Error{Pointer: pointer, Code: "uniqueItems",
				Message: "Origin must not be repeated."})
			continue
		}
		seen[canonical] = struct{}{}
	}
	return fieldErrors
}

func formResource(formModel *model.Form) map[string]any {
	return map[string]any{"id": formModel.ID, "organizationId": formModel.OrganizationID,
		"name": formModel.Name, "title": formModel.Title,
		"description": formModel.Description, "publicKey": formModel.PublicKey, "status": formModel.Status,
		"currentVersion": formModel.CurrentSchemaVersion, "createdAt": formModel.CreatedAt.UTC().Format(time.RFC3339),
		"updatedAt": formModel.UpdatedAt.UTC().Format(time.RFC3339)}
}

func schemaVersionResource(version *model.SchemaVersion) map[string]any {
	resource := map[string]any{"formId": version.FormID(), "version": version.Version(), "state": version.State(),
		"schema": version.Schema(), "createdAt": version.CreatedAt().UTC().Format(time.RFC3339)}
	if version.PublishedAt() != nil {
		resource["publishedAt"] = version.PublishedAt().UTC().Format(time.RFC3339)
	}
	return resource
}

func submissionResource(submission *model.FormSubmission) map[string]any {
	return map[string]any{"id": submission.ID, "formId": submission.FormID, "schemaVersion": submission.SchemaVersion,
		"requestId": submission.RequestID, "status": submission.Status, "data": submission.Data,
		"submittedAt": submission.SubmittedAt.UTC().Format(time.RFC3339)}
}

func (h *V1APIHandler) writeRepositoryError(c echo.Context, err error) error {
	if errors.Is(err, model.ErrPreconditionFailed) {
		return h.writeError(c, http.StatusPreconditionFailed, "precondition_failed", "The form was modified by another request.", nil)
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "not found") || strings.Contains(message, "record not found") {
		return h.writeError(c, http.StatusNotFound, "not_found", "The requested resource was not found.", nil)
	}
	if strings.Contains(message, "duplicate") || strings.Contains(message, "unique") {
		return h.writeError(c, http.StatusConflict, "conflict", "The resource already exists.", nil)
	}
	if strings.Contains(message, "invalid input") || strings.Contains(message, "invalid uuid") {
		return h.writeError(c, http.StatusBadRequest, "invalid_request", "The resource identifier is invalid.", nil)
	}
	if h.logger != nil {
		h.logger.Error("v1 API repository failure", "request_id", requestID(c), "error", err)
	}
	return h.writeError(c, http.StatusInternalServerError, "internal_error", "The request could not be completed.", nil)
}

func formETag(formModel *model.Form) string {
	value := formModel.UpdatedAt.UTC().Format(time.RFC3339Nano)
	return `"form-` + base64.RawURLEncoding.EncodeToString([]byte(value)) + `"`
}

func formListOptions(c echo.Context) (model.FormListOptions, error) {
	limit, err := boundedInt(c.QueryParam("limit"), 25, 1, 100, "form page limit")
	if err != nil {
		return model.FormListOptions{}, err
	}
	offset, err := boundedInt(c.QueryParam("offset"), 0, 0, 10000, "form page offset")
	if err != nil {
		return model.FormListOptions{}, err
	}
	sort := model.FormSort(c.QueryParam("sort"))
	if sort == "" {
		sort = model.FormSortCreatedDesc
	}
	options := model.FormListOptions{
		Status: model.LifecycleStatus(c.QueryParam("status")), Query: strings.TrimSpace(c.QueryParam("q")),
		Sort: sort, Limit: limit, Offset: offset,
	}
	return options, options.Validate()
}

func boundedInt(raw string, defaultValue, minimum, maximum int, name string) (int, error) {
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("%s must be between %d and %d", name, minimum, maximum)
	}
	return value, nil
}

func requestID(c echo.Context) string {
	id := c.Request().Header.Get(constants.HeaderTraceID)
	if !traceIDPattern.MatchString(id) {
		id = "req_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	c.Response().Header().Set(constants.HeaderTraceID, id)
	return id
}

func (h *V1APIHandler) writeError(
	c echo.Context,
	status int,
	code string,
	message string,
	fields []validation.Error,
) error {
	errorResource := map[string]any{"code": code, "message": message, "requestId": requestID(c)}
	if len(fields) > 0 {
		errorResource["fields"] = fields
	}
	return c.JSON(status, map[string]any{"error": errorResource})
}
