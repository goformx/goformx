package web

import (
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

	"github.com/goformx/goforms/internal/application/constants"
	ctxmw "github.com/goformx/goforms/internal/application/middleware/context"
	"github.com/goformx/goforms/internal/application/middleware/serviceauth"
	"github.com/goformx/goforms/internal/application/validation"
	"github.com/goformx/goforms/internal/domain/auth"
	formdomain "github.com/goformx/goforms/internal/domain/form"
	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/infrastructure/logging"
	tokenstore "github.com/goformx/goforms/internal/infrastructure/repository/token"
)

var formNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{2,62}$`)

// V1APIHandler implements the schema-first control and public data planes.
type V1APIHandler struct {
	repository formdomain.Repository
	auth       *serviceauth.Middleware
	validator  *validation.ComprehensiveValidator
	logger     logging.Logger
	requests   atomic.Uint64
	errors     atomic.Uint64
}

// NewV1APIHandler wires the v1 API to the canonical repository and scoped service-token store.
func NewV1APIHandler(
	repository formdomain.Repository,
	tokens *tokenstore.Store,
	logger logging.Logger,
) *V1APIHandler {
	return newV1APIHandler(repository, tokens, validation.NewComprehensiveValidator(), logger)
}

func newV1APIHandler(
	repository formdomain.Repository,
	tokens serviceauth.Repository,
	validator *validation.ComprehensiveValidator,
	logger logging.Logger,
) *V1APIHandler {
	return &V1APIHandler{repository: repository, auth: serviceauth.New(tokens), validator: validator, logger: logger}
}

func (h *V1APIHandler) RegisterRoutes(e *echo.Echo) {
	control := e.Group("/v1/forms")
	control.GET("", h.instrument("list_forms", h.listForms), h.require(auth.ScopeFormsRead))
	control.POST("", h.instrument("create_form", h.createForm), h.require(auth.ScopeFormsWrite))
	control.GET("/:formId", h.instrument("get_form", h.getForm), h.require(auth.ScopeFormsRead))
	control.PATCH("/:formId", h.instrument("update_form", h.updateForm), h.require(auth.ScopeFormsWrite))
	control.POST("/:formId/versions", h.instrument("create_schema_version", h.createSchemaVersion), h.require(auth.ScopeFormsWrite))
	control.POST("/:formId/versions/:version/publish", h.instrument("publish_schema_version", h.publishSchemaVersion), h.require(auth.ScopeFormsPublish))
	control.GET("/:formId/submissions", h.instrument("list_submissions", h.listSubmissions), h.require(auth.ScopeSubmissionsRead))

	public := e.Group("/v1/public/forms")
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
				"ETag, "+constants.HeaderSchemaVersion+", "+ctxmw.RequestIDHeader+", "+constants.HeaderReplay)
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
				if status == http.StatusForbidden {
					code = "forbidden"
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
	c.Response().Header().Set(echo.HeaderLocation, "/v1/forms/"+formModel.ID)
	return c.JSON(http.StatusCreated, map[string]any{"data": formResource(formModel)})
}

func (h *V1APIHandler) listForms(c echo.Context) error {
	principal, _ := serviceauth.PrincipalFrom(c)
	forms, err := h.repository.ListForms(c.Request().Context(), principal.OwnerID)
	if err != nil {
		return h.writeRepositoryError(c, err)
	}
	data := make([]map[string]any, 0, len(forms))
	for _, formModel := range forms {
		data = append(data, formResource(formModel))
	}
	return c.JSON(http.StatusOK, map[string]any{"data": data})
}

func (h *V1APIHandler) ownedForm(c echo.Context) (*model.Form, bool) {
	formModel, err := h.repository.GetFormByID(c.Request().Context(), c.Param("formId"))
	if err != nil {
		_ = h.writeRepositoryError(c, err)
		return nil, false
	}
	if err := serviceauth.RequireOwner(c, formModel.UserID); err != nil {
		_ = h.writeError(c, http.StatusForbidden, "forbidden", "The service token cannot access this form.", nil)
		return nil, false
	}
	return formModel, true
}

func (h *V1APIHandler) getForm(c echo.Context) error {
	formModel, ok := h.ownedForm(c)
	if !ok {
		return nil
	}
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
	if err := h.repository.UpdateForm(c.Request().Context(), formModel); err != nil {
		return h.writeRepositoryError(c, err)
	}
	updated, err := h.repository.GetFormByID(c.Request().Context(), formModel.ID)
	if err != nil {
		return h.writeRepositoryError(c, err)
	}
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
	submissions, err := h.repository.ListSubmissions(c.Request().Context(), formModel.ID)
	if err != nil {
		return h.writeRepositoryError(c, err)
	}
	data := make([]map[string]any, 0, len(submissions))
	for _, submission := range submissions {
		data = append(data, submissionResource(submission))
	}
	return c.JSON(http.StatusOK, map[string]any{"data": data})
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
		IdempotencyKey: idempotencyKey, Data: request.Data, SubmittedAt: now,
		Status: model.SubmissionStatusAccepted, Metadata: model.JSON{}}
	stored, replayed, err := h.repository.CreateSubmissionIdempotent(c.Request().Context(), submission)
	if err != nil {
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
	return map[string]any{"id": formModel.ID, "name": formModel.Name, "title": formModel.Title,
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
		"status": submission.Status, "data": submission.Data, "submittedAt": submission.SubmittedAt.UTC().Format(time.RFC3339)}
}

func (h *V1APIHandler) writeRepositoryError(c echo.Context, err error) error {
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

func requestID(c echo.Context) string {
	id := ctxmw.GetRequestID(c.Request().Context())
	if id == "" {
		id = c.Request().Header.Get(ctxmw.RequestIDHeader)
	}
	if id == "" {
		id = "req_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	c.Response().Header().Set(ctxmw.RequestIDHeader, id)
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
