package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/goformx/goforms/internal/application/constants"
	"github.com/goformx/goforms/internal/application/validation"
	"github.com/goformx/goforms/internal/domain/auth"
	domainform "github.com/goformx/goforms/internal/domain/form"
	"github.com/goformx/goforms/internal/domain/form/model"
	domainsubmission "github.com/goformx/goforms/internal/domain/submission"
	mockform "github.com/goformx/goforms/test/mocks/form"
)

type fixedTokenRepository struct{ token *auth.ServiceToken }

func (r fixedTokenRepository) FindByID(_ context.Context, tokenID string) (*auth.ServiceToken, error) {
	if r.token != nil && r.token.ID == tokenID {
		return r.token, nil
	}
	return nil, nil
}

func (r fixedTokenRepository) MarkUsed(_ context.Context, _ string, _ time.Time) error {
	return nil
}

func TestV1ContactFormVerticalSlice(t *testing.T) {
	ctrl := gomock.NewController(t)
	repository := mockform.NewMockRepository(ctrl)
	validator := validation.NewComprehensiveValidator()
	now := time.Now().UTC()
	token, plaintext, err := auth.Issue("owner-a", []auth.Scope{
		auth.ScopeFormsRead, auth.ScopeFormsWrite, auth.ScopeFormsPublish, auth.ScopeSubmissionsRead,
	}, time.Hour, now)
	require.NoError(t, err)

	var formModel *model.Form
	versions := map[int]*model.SchemaVersion{}
	submissions := map[string]*model.FormSubmission{}

	repository.EXPECT().CreateForm(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, candidate *model.Form) error {
			candidate.ID = "11111111-1111-4111-8111-111111111111"
			candidate.PublicKey = "gfpk_abcdefghijklmnopqrstuvwxyz123456"
			candidate.CurrentSchemaVersion = 1
			candidate.Status = model.LifecycleDraft
			candidate.CreatedAt, candidate.UpdatedAt = now, now
			formModel = candidate
			version, createErr := model.NewSchemaVersion(candidate.ID, 1, candidate.Schema, validator)
			require.NoError(t, createErr)
			versions[1] = version
			return nil
		},
	)
	repository.EXPECT().GetFormByID(gomock.Any(), "owner-a", gomock.Any()).DoAndReturn(
		func(_ context.Context, _, _ string) (*model.Form, error) { return formModel, nil },
	).AnyTimes()
	repository.EXPECT().CreateSchemaVersion(gomock.Any(), "owner-a", gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, formID string, schema model.JSON) (*model.SchemaVersion, error) {
			version, createErr := model.NewSchemaVersion(formID, 2, schema, validator)
			if createErr == nil {
				versions[2] = version
			}
			return version, createErr
		},
	)
	repository.EXPECT().ListSchemaVersions(gomock.Any(), "owner-a", gomock.Any(), 25, 0).DoAndReturn(
		func(context.Context, string, string, int, int) ([]*model.SchemaVersion, int64, error) {
			return []*model.SchemaVersion{versions[2], versions[1]}, 2, nil
		},
	)
	repository.EXPECT().GetSchemaVersion(gomock.Any(), "owner-a", gomock.Any(), 2).DoAndReturn(
		func(context.Context, string, string, int) (*model.SchemaVersion, error) { return versions[2], nil },
	).Times(4)
	repository.EXPECT().PublishSchemaVersion(gomock.Any(), "owner-a", gomock.Any(), 2).DoAndReturn(
		func(_ context.Context, _, _ string, _ int) (*model.SchemaVersion, error) {
			published, publishErr := versions[2].Publish(now)
			if publishErr == nil {
				versions[2] = published
				formModel.Status = model.LifecyclePublished
				formModel.CurrentSchemaVersion = 2
			}
			return published, publishErr
		},
	)
	repository.EXPECT().GetPublishedSchemaVersion(gomock.Any(), formPublicKey(), gomock.Any()).DoAndReturn(
		func(_ context.Context, _ string, requested int) (*model.Form, *model.SchemaVersion, error) {
			if requested == 0 {
				requested = formModel.CurrentSchemaVersion
			}
			return formModel, versions[requested], nil
		},
	).AnyTimes()
	repository.EXPECT().CreateSubmissionIdempotent(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, candidate *model.FormSubmission) (*model.FormSubmission, bool, error) {
			if existing := submissions[candidate.IdempotencyKey]; existing != nil {
				return existing, true, nil
			}
			candidate.ID = "22222222-2222-4222-8222-222222222222"
			submissions[candidate.IdempotencyKey] = candidate
			return candidate, false, nil
		},
	).Times(3)
	repository.EXPECT().ListSubmissionsPage(gomock.Any(), "owner-a", gomock.Any(), domainsubmission.ListOptions{Limit: 25}).DoAndReturn(
		func(context.Context, string, string, domainsubmission.ListOptions) ([]*model.FormSubmission, bool, error) {
			return []*model.FormSubmission{submissions["contact-submit-0001"]}, false, nil
		},
	)
	repository.EXPECT().GetSubmissionByOrganization(
		gomock.Any(), "owner-a", "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222",
	).DoAndReturn(func(context.Context, string, string, string) (*model.FormSubmission, error) {
		return submissions["contact-submit-0001"], nil
	})

	e := echo.New()
	handler := newV1APIHandler(repository, fixedTokenRepository{token: token}, validator, nil)
	handler.RegisterRoutes(e)

	createSchema := contactSchema("name", "email", "message")
	createResponse := requestJSON(t, e, http.MethodPost, "/v1/forms", map[string]any{
		"name": "personal-contact", "title": "Contact Russell", "schema": createSchema,
		"allowedOrigins": []string{"https://jonesrussell.github.io"},
	}, plaintext, "create-contact-0001", nil)
	require.Equal(t, http.StatusCreated, createResponse.Code, createResponse.Body.String())
	require.Equal(t, "/v1/forms/11111111-1111-4111-8111-111111111111", createResponse.Header().Get("Location"))
	require.Contains(t, createResponse.Body.String(), `"allowedOrigins":["https://jonesrussell.github.io"]`)

	versionSchema := contactSchema("name", "email", "message", "company")
	versionResponse := requestJSON(t, e, http.MethodPost,
		"/v1/forms/11111111-1111-4111-8111-111111111111/versions",
		map[string]any{"schema": versionSchema}, plaintext, "create-version-0002", nil)
	require.Equal(t, http.StatusCreated, versionResponse.Code, versionResponse.Body.String())
	listVersionsResponse := requestJSON(t, e, http.MethodGet,
		"/v1/forms/11111111-1111-4111-8111-111111111111/versions",
		nil, plaintext, "", nil)
	require.Equal(t, http.StatusOK, listVersionsResponse.Code, listVersionsResponse.Body.String())
	require.Contains(t, listVersionsResponse.Body.String(), `"total":2`)
	getVersionResponse := requestJSON(t, e, http.MethodGet,
		"/v1/forms/11111111-1111-4111-8111-111111111111/versions/2",
		nil, plaintext, "", nil)
	require.Equal(t, http.StatusOK, getVersionResponse.Code, getVersionResponse.Body.String())

	publishResponse := requestJSON(t, e, http.MethodPost,
		"/v1/forms/11111111-1111-4111-8111-111111111111/versions/2/publish",
		nil, plaintext, "", nil)
	require.Equal(t, http.StatusOK, publishResponse.Code, publishResponse.Body.String())

	schemaResponse := requestJSON(t, e, http.MethodGet, "/v1/public/forms/"+formPublicKey()+"/schema",
		nil, "", "", map[string]string{echo.HeaderOrigin: "https://jonesrussell.github.io"})
	require.Equal(t, http.StatusOK, schemaResponse.Code, schemaResponse.Body.String())
	require.Equal(t, "2", schemaResponse.Header().Get("X-GoFormX-Schema-Version"))
	require.Equal(t, "application/schema+json", schemaResponse.Header().Get(echo.HeaderContentType))
	require.Equal(t, "https://jonesrussell.github.io", schemaResponse.Header().Get(echo.HeaderAccessControlAllowOrigin))

	invalidResponse := requestJSON(t, e, http.MethodPost, "/v1/public/forms/"+formPublicKey()+"/submissions",
		map[string]any{"data": map[string]any{"name": "Ada", "email": "not-an-email", "message": "Hello"}},
		"", "contact-invalid-001", map[string]string{"X-GoFormX-Schema-Version": "2", echo.HeaderOrigin: "https://jonesrussell.github.io"})
	require.Equal(t, http.StatusUnprocessableEntity, invalidResponse.Code, invalidResponse.Body.String())
	require.Contains(t, invalidResponse.Body.String(), `"pointer":"/data/email"`)
	require.Contains(t, invalidResponse.Body.String(), `"requestId":"req_`)

	payload := map[string]any{"data": map[string]any{
		"name": "Ada", "email": "ada@example.com", "message": "Hello", "company": "Analytical Engines",
	}}
	accepted := requestJSON(t, e, http.MethodPost, "/v1/public/forms/"+formPublicKey()+"/submissions",
		payload, "", "contact-submit-0001", map[string]string{"X-GoFormX-Schema-Version": "2", echo.HeaderOrigin: "https://jonesrussell.github.io"})
	require.Equal(t, http.StatusAccepted, accepted.Code, accepted.Body.String())
	require.Contains(t, accepted.Body.String(), `"status":"accepted"`)

	replayed := requestJSON(t, e, http.MethodPost, "/v1/public/forms/"+formPublicKey()+"/submissions",
		payload, "", "contact-submit-0001", map[string]string{"X-GoFormX-Schema-Version": "2", echo.HeaderOrigin: "https://jonesrussell.github.io"})
	require.Equal(t, http.StatusAccepted, replayed.Code, replayed.Body.String())
	require.Equal(t, "true", replayed.Header().Get("Idempotency-Replayed"))
	require.JSONEq(t, accepted.Body.String(), replayed.Body.String())

	conflictingPayload := map[string]any{"data": map[string]any{
		"name": "Grace", "email": "grace@example.com", "message": "Different body",
	}}
	conflict := requestJSON(t, e, http.MethodPost, "/v1/public/forms/"+formPublicKey()+"/submissions",
		conflictingPayload, "", "contact-submit-0001",
		map[string]string{"X-GoFormX-Schema-Version": "2", echo.HeaderOrigin: "https://jonesrussell.github.io"})
	require.Equal(t, http.StatusConflict, conflict.Code, conflict.Body.String())
	require.Contains(t, conflict.Body.String(), `"code":"idempotency_conflict"`)

	listResponse := requestJSON(t, e, http.MethodGet,
		"/v1/forms/11111111-1111-4111-8111-111111111111/submissions", nil, plaintext, "", nil)
	require.Equal(t, http.StatusOK, listResponse.Code, listResponse.Body.String())
	require.Contains(t, listResponse.Body.String(), "ada@example.com")
	detailResponse := requestJSON(t, e, http.MethodGet,
		"/v1/forms/11111111-1111-4111-8111-111111111111/submissions/22222222-2222-4222-8222-222222222222",
		nil, plaintext, "", nil)
	require.Equal(t, http.StatusOK, detailResponse.Code, detailResponse.Body.String())
	require.Contains(t, detailResponse.Body.String(), `"schemaVersion":2`)
	require.Contains(t, detailResponse.Body.String(), `"requestId":"req_`)

	unauthorized := requestJSON(t, e, http.MethodGet, "/v1/forms", nil, "", "", nil)
	require.Equal(t, http.StatusUnauthorized, unauthorized.Code, unauthorized.Body.String())
	require.Contains(t, unauthorized.Body.String(), `"code":"unauthorized"`)
	require.NotEmpty(t, unauthorized.Header().Get("X-Trace-Id"))
}

func TestValidateOriginsAcceptsOriginsAndRejectsURLsOrDuplicates(t *testing.T) {
	require.Empty(t, validateOrigins([]string{"https://example.com", "http://localhost:5173"}))
	errors := validateOrigins([]string{
		"https://example.com/contact", "https://example.com", "https://example.com/", "javascript:alert(1)",
	})
	require.Len(t, errors, 3)
	require.Equal(t, "/allowedOrigins/0", errors[0].Pointer)
	require.Equal(t, "uniqueItems", errors[1].Code)
	require.Equal(t, "format", errors[2].Code)
}

func TestControlPlaneRejectsCrossOwnerFormAccess(t *testing.T) {
	t.Parallel()
	repository := mockform.NewMockRepository(gomock.NewController(t))
	repository.EXPECT().GetFormByID(gomock.Any(), "owner-a", "form-owned-by-b").Return(nil, errors.New("not found"))
	now := time.Now()
	token, plaintext, err := auth.Issue("owner-a", []auth.Scope{auth.ScopeFormsRead}, time.Hour, now)
	require.NoError(t, err)
	router := echo.New()
	newV1APIHandler(repository, fixedTokenRepository{token: token},
		validation.NewComprehensiveValidator(), nil).RegisterRoutes(router)

	response := requestJSON(t, router, http.MethodGet, "/v1/forms/form-owned-by-b",
		nil, plaintext, "", nil)
	require.Equal(t, http.StatusNotFound, response.Code)
	require.Contains(t, response.Body.String(), `"code":"not_found"`)
}

func TestSubmissionPaginationInputsAreBoundedAndOpaque(t *testing.T) {
	t.Parallel()
	require.Equal(t, domainsubmission.DefaultPageLimit, mustSubmissionLimit(t, ""))
	require.Equal(t, domainsubmission.MaxPageLimit, mustSubmissionLimit(t, "100"))
	_, err := submissionPageLimit("101")
	require.Error(t, err)

	submission := &model.FormSubmission{ID: "11111111-1111-4111-8111-111111111111", SubmittedAt: time.Now().UTC()}
	cursor := encodeSubmissionCursor(submission)
	before, beforeID, err := decodeSubmissionCursor(cursor)
	require.NoError(t, err)
	require.True(t, submission.SubmittedAt.Equal(before))
	require.Equal(t, submission.ID, beforeID)
	_, _, err = decodeSubmissionCursor("not-a-cursor")
	require.Error(t, err)
}

func TestFormListOptionsValidateFiltersSortingAndBounds(t *testing.T) {
	t.Parallel()
	router := echo.New()
	request := httptest.NewRequest(http.MethodGet,
		"/v1/forms?status=published&q=contact&sort=name&limit=50&offset=25", nil)
	context := router.NewContext(request, httptest.NewRecorder())
	options, err := formListOptions(context)
	require.NoError(t, err)
	require.Equal(t, model.LifecyclePublished, options.Status)
	require.Equal(t, "contact", options.Query)
	require.Equal(t, model.FormSortNameAsc, options.Sort)
	require.Equal(t, 50, options.Limit)
	require.Equal(t, 25, options.Offset)

	invalid := httptest.NewRequest(http.MethodGet, "/v1/forms?sort=random&limit=101", nil)
	_, err = formListOptions(router.NewContext(invalid, httptest.NewRecorder()))
	require.Error(t, err)
}

func TestFormMetadataUpdateRequiresRepositoryEnforcedETag(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC().Truncate(time.Microsecond)
	formID := "11111111-1111-4111-8111-111111111111"
	base := &model.Form{ID: formID, OrganizationID: "owner-a", Name: "contact", Title: "Contact",
		UpdatedAt: now, CreatedAt: now, Status: model.LifecycleDraft, CurrentSchemaVersion: 1}
	repository := mockform.NewMockRepository(gomock.NewController(t))
	updated := false
	repository.EXPECT().GetFormByID(gomock.Any(), "owner-a", formID).DoAndReturn(
		func(context.Context, string, string) (*model.Form, error) {
			copy := *base
			if updated {
				copy.Title = "Updated contact"
				copy.UpdatedAt = now.Add(time.Second)
			}
			return &copy, nil
		},
	).AnyTimes()
	repository.EXPECT().UpdateForm(gomock.Any(), gomock.Any(), now).DoAndReturn(
		func(_ context.Context, candidate *model.Form, expected time.Time) error {
			require.Equal(t, "Updated contact", candidate.Title)
			require.Equal(t, now, expected)
			updated = true
			return nil
		},
	)
	token, plaintext, err := auth.Issue("owner-a", []auth.Scope{auth.ScopeFormsWrite}, time.Hour, now)
	require.NoError(t, err)
	router := echo.New()
	newV1APIHandler(repository, fixedTokenRepository{token: token},
		validation.NewComprehensiveValidator(), nil).RegisterRoutes(router)
	path := "/v1/forms/" + formID

	missing := requestJSON(t, router, http.MethodPatch, path, map[string]any{"title": "Updated contact"}, plaintext, "", nil)
	require.Equal(t, http.StatusPreconditionRequired, missing.Code, missing.Body.String())
	stale := requestJSON(t, router, http.MethodPatch, path, map[string]any{"title": "Updated contact"}, plaintext, "", map[string]string{
		constants.HeaderIfMatch: `"form-stale"`,
	})
	require.Equal(t, http.StatusPreconditionFailed, stale.Code, stale.Body.String())

	currentETag := formETag(base)
	success := requestJSON(t, router, http.MethodPatch, path, map[string]any{"title": "Updated contact"}, plaintext, "", map[string]string{
		constants.HeaderIfMatch: currentETag, echo.HeaderContentType: mediaTypeMergePatch,
	})
	require.Equal(t, http.StatusOK, success.Code, success.Body.String())
	require.NotEqual(t, currentETag, success.Header().Get(constants.HeaderETag))
}

func TestRequestIDAcceptsSafeCorrelationAndReplacesUnsafeInput(t *testing.T) {
	t.Parallel()
	router := echo.New()
	safeRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	safeRequest.Header.Set("X-Trace-Id", "client.request-123:retry_2")
	safeContext := router.NewContext(safeRequest, httptest.NewRecorder())
	require.Equal(t, "client.request-123:retry_2", requestID(safeContext))

	unsafeRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	unsafeRequest.Header.Set("X-Trace-Id", "attacker\nforged-log-line")
	unsafeContext := router.NewContext(unsafeRequest, httptest.NewRecorder())
	require.Regexp(t, `^req_[a-f0-9]{32}$`, requestID(unsafeContext))
}

func mustSubmissionLimit(t *testing.T, value string) int {
	t.Helper()
	limit, err := submissionPageLimit(value)
	require.NoError(t, err)
	return limit
}

func TestPublicSubmissionAdmissionIsScopedPerForm(t *testing.T) {
	t.Parallel()
	repository := mockform.NewMockRepository(gomock.NewController(t))
	validator := validation.NewComprehensiveValidator()
	formModel := &model.Form{ID: "11111111-1111-4111-8111-111111111111", PublicKey: formPublicKey()}
	version, err := model.NewSchemaVersion(formModel.ID, 1, contactSchema("email"), validator)
	require.NoError(t, err)
	published, err := version.Publish(time.Now().UTC())
	require.NoError(t, err)
	repository.EXPECT().GetPublishedSchemaVersion(gomock.Any(), formPublicKey(), gomock.Any()).Return(
		formModel, published, nil,
	).Times(2)
	repository.EXPECT().CreateSubmissionIdempotent(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, submission *model.FormSubmission) (*model.FormSubmission, bool, error) {
			submission.ID = "22222222-2222-4222-8222-222222222222"
			return submission, false, nil
		},
	)

	router := echo.New()
	newV1APIHandlerWithLimits(repository, fixedTokenRepository{}, validator, nil,
		V1Limits{PublicSubmissionRPS: 0.001, PublicSubmissionBurst: 1}).RegisterRoutes(router)
	payload := map[string]any{"data": map[string]any{"email": "ada@example.com"}}
	first := requestJSON(t, router, http.MethodPost, "/v1/public/forms/"+formPublicKey()+"/submissions",
		payload, "", "admission-submit-0001", nil)
	require.Equal(t, http.StatusAccepted, first.Code, first.Body.String())
	second := requestJSON(t, router, http.MethodPost, "/v1/public/forms/"+formPublicKey()+"/submissions",
		payload, "", "admission-submit-0002", nil)
	require.Equal(t, http.StatusTooManyRequests, second.Code, second.Body.String())
	require.Equal(t, "1", second.Header().Get(echo.HeaderRetryAfter))
}

func TestDailySubmissionLimitReturnsRetryablePublicError(t *testing.T) {
	t.Parallel()
	repository := mockform.NewMockRepository(gomock.NewController(t))
	validator := validation.NewComprehensiveValidator()
	formModel := &model.Form{ID: "11111111-1111-4111-8111-111111111111", PublicKey: formPublicKey()}
	version, err := model.NewSchemaVersion(formModel.ID, 1, contactSchema("email"), validator)
	require.NoError(t, err)
	published, err := version.Publish(time.Now().UTC())
	require.NoError(t, err)
	repository.EXPECT().GetPublishedSchemaVersion(gomock.Any(), formPublicKey(), gomock.Any()).Return(
		formModel, published, nil,
	)
	repository.EXPECT().CreateSubmissionIdempotent(gomock.Any(), gomock.Any()).Return(
		nil, false, domainform.ErrSubmissionLimitExceeded,
	)

	router := echo.New()
	newV1APIHandler(repository, fixedTokenRepository{}, validator, nil).RegisterRoutes(router)
	response := requestJSON(t, router, http.MethodPost, "/v1/public/forms/"+formPublicKey()+"/submissions",
		map[string]any{"data": map[string]any{"email": "ada@example.com"}},
		"", "daily-limit-00001", nil)
	require.Equal(t, http.StatusTooManyRequests, response.Code, response.Body.String())
	require.Equal(t, "86400", response.Header().Get(echo.HeaderRetryAfter))
	require.Contains(t, response.Body.String(), `"code":"submission_limit_reached"`)
}

func formPublicKey() string { return "gfpk_abcdefghijklmnopqrstuvwxyz123456" }

func contactSchema(properties ...string) model.JSON {
	definitions := map[string]any{}
	required := make([]any, 0, 3)
	for _, property := range properties {
		definition := map[string]any{"type": "string"}
		if property == "email" {
			definition["format"] = "email"
		}
		definitions[property] = definition
		if property != "company" {
			required = append(required, property)
		}
	}
	return model.JSON{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
		"properties": definitions, "required": required, "additionalProperties": false}
}

func requestJSON(
	t *testing.T,
	e *echo.Echo,
	method string,
	path string,
	body any,
	token string,
	idempotencyKey string,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		require.NoError(t, err)
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	if body != nil {
		request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	if token != "" {
		request.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	e.ServeHTTP(recorder, request)
	return recorder
}
