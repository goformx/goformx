package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/submission"
	mockform "github.com/goformx/goforms/test/mocks/form"
)

func requestRawExport(router *echo.Echo, path, body, credential string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+credential)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestExportFilterNumbersHonorIntegerSemanticsWithoutRounding(t *testing.T) {
	for _, number := range []string{"1", "1.0", "1e0", "0.01e2"} {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"format":"json","schemaVersion":`+number+`}`))
		format, filters, err := decodeExportRequest(echo.New().NewContext(request, httptest.NewRecorder()))
		require.NoError(t, err)
		require.Equal(t, submission.ExportJSON, format)
		require.Equal(t, 1, filters.SchemaVersion)
	}
	for _, number := range []string{"1.00000000000000000001", "1e999999999", "null", `"1"`, "2147483648", "-1", "0"} {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"format":"json","schemaVersion":`+number+`}`))
		_, _, err := decodeExportRequest(echo.New().NewContext(request, httptest.NewRecorder()))
		require.ErrorIs(t, err, submission.ErrExportRequest)
	}
}

func TestExportRejectsAmbiguousUnsupportedAndOversizeInputs(t *testing.T) {
	for _, body := range []string{
		`{}`, `null`, `[]`, `{"format":null}`, `{"format":"xml"}`, `{"format":"json","format":"csv"}`,
		`{"format":"json","cursor":"private-canary"}`, `{"format":"json","limit":1000}`, `{"format":"json","organizationId":"private-canary"}`,
		`{"format":"json","status":""}`, `{"format":"json","status":null}`, `{"format":"json","schemaVersion":0}`,
		`{"format":"json","schemaVersion":2147483648}`, `{"format":"json","schemaVersion":1.1}`, `{"format":"json","schemaVersion":"1"}`,
		`{"format":"json","receivedFrom":"2026-08-30"}`, `{"format":"json","receivedFrom":"2026-08-30T00:00:00.1234567Z"}`,
		`{"format":"json","receivedFrom":"2026-08-31T00:00:00Z","receivedBefore":"2026-08-30T00:00:00Z"}`,
		`{"format":"json"} {}`, `{"format":"json","status":"` + strings.Repeat("x", maxSubmissionQueryBytes) + `"}`,
	} {
		t.Run(body[:min(len(body), 90)], func(t *testing.T) {
			repository := mockform.NewMockRepository(gomock.NewController(t))
			owner, formID := uuid.NewString(), uuid.NewString()
			token, credential, err := auth.Issue(owner, []auth.Scope{auth.ScopeSubmissionsRead}, time.Hour, time.Now())
			require.NoError(t, err)
			repository.EXPECT().GetFormByID(gomock.Any(), owner, formID).Return(&model.Form{ID: formID, OrganizationID: owner}, nil)
			router := echo.New()
			handler := NewV1APIHandler(repository, fixedTokenRepository{token: token}, nil)
			handler.RegisterRoutes(router)
			response := requestRawExport(router, "/v1/forms/"+formID+"/submissions/export", body, credential)
			require.False(t, handler.exportActive.Load(), "invalid input must release admission")
			require.Equal(t, http.StatusBadRequest, response.Code)
			require.NotContains(t, response.Body.String(), "private-canary")
			require.Empty(t, response.Header().Get("Content-Disposition"))
		})
	}
}

func TestExportFailuresNeverReleaseOrAuditPartialDownloads(t *testing.T) {
	for _, scenario := range []string{"source_limit", "timeout", "wrong_form", "invalid_policy", "output_limit", "audit_failed", "query_filter"} {
		t.Run(scenario, func(t *testing.T) {
			repository := mockform.NewMockRepository(gomock.NewController(t))
			owner, formID := uuid.NewString(), uuid.NewString()
			token, credential, err := auth.Issue(owner, []auth.Scope{auth.ScopeSubmissionsRead}, time.Hour, time.Now())
			require.NoError(t, err)
			repository.EXPECT().GetFormByID(gomock.Any(), owner, formID).Return(&model.Form{ID: formID, OrganizationID: owner}, nil)
			row := submission.ExportRecord{Submission: &model.FormSubmission{ID: uuid.NewString(), FormID: formID, SchemaVersion: 1,
				Data: model.JSON{"secret": "private-canary"}}, SchemaFormID: formID, AcceptedVersion: 1,
				Policy: model.JSON{submission.SensitiveAnnotation: []string{"/secret"}}}
			var readErr error
			status := http.StatusInternalServerError
			path := "/v1/forms/" + formID + "/submissions/export"
			switch scenario {
			case "source_limit":
				readErr, status = submission.ErrExportLimit, http.StatusRequestEntityTooLarge
			case "timeout":
				readErr, status = context.DeadlineExceeded, http.StatusGatewayTimeout
			case "wrong_form":
				row.Submission.FormID = uuid.NewString()
			case "invalid_policy":
				row.Policy[submission.SensitiveAnnotation] = nil
			case "output_limit":
				row.Submission.Data["large"] = strings.Repeat("x", submission.MaxExportBytes)
				status = http.StatusRequestEntityTooLarge
			case "audit_failed":
				status = http.StatusServiceUnavailable
				repository.EXPECT().SaveSubmissionExportAudit(gomock.Any(), gomock.Any()).Return(errors.New("private-canary-audit-error"))
			case "query_filter":
				path += "?status=accepted"
				status = http.StatusBadRequest
			}
			if scenario != "query_filter" {
				repository.EXPECT().ReadSubmissionExport(gomock.Any(), owner, formID, submission.ExportFilters{}).DoAndReturn(
					func(ctx context.Context, _, _ string, _ submission.ExportFilters) ([]submission.ExportRecord, error) {
						deadline, bounded := ctx.Deadline()
						require.True(t, bounded)
						require.LessOrEqual(t, time.Until(deadline), submission.ExportTimeout)
						return []submission.ExportRecord{row}, readErr
					})
			}
			router := echo.New()
			handler := NewV1APIHandler(repository, fixedTokenRepository{token: token}, nil)
			handler.RegisterRoutes(router)
			response := requestRawExport(router, path, `{"format":"json"}`, credential)
			require.False(t, handler.exportActive.Load(), "failure must release admission")
			require.Equal(t, status, response.Code, response.Body.String())
			require.NotContains(t, response.Body.String(), "private-canary")
			require.NotContains(t, response.Body.String(), `"data":`)
			require.Empty(t, response.Header().Get("Content-Disposition"))
			require.Empty(t, response.Header().Get("X-GoFormX-Export-ID"))
			require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
		})
	}
}

func TestExportAdmissionRejectsConcurrencyAndReleasesAfterCompletion(t *testing.T) {
	// The production admission mechanism is deliberately a single slot. Keep
	// its observable behavior coupled to the public contract's declared limit.
	require.Equal(t, 1, submission.MaxConcurrentExports)
	repository := mockform.NewMockRepository(gomock.NewController(t))
	owner, formID := uuid.NewString(), uuid.NewString()
	token, credential, err := auth.Issue(owner, []auth.Scope{auth.ScopeSubmissionsRead}, time.Hour, time.Now())
	require.NoError(t, err)
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	repository.EXPECT().GetFormByID(gomock.Any(), owner, formID).
		Return(&model.Form{ID: formID, OrganizationID: owner}, nil).Times(2)
	firstRead := repository.EXPECT().ReadSubmissionExport(gomock.Any(), owner, formID, submission.ExportFilters{}).
		DoAndReturn(func(ctx context.Context, _, _ string, _ submission.ExportFilters) ([]submission.ExportRecord, error) {
			close(entered)
			select {
			case <-release:
				return nil, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		})
	repository.EXPECT().ReadSubmissionExport(gomock.Any(), owner, formID, submission.ExportFilters{}).
		After(firstRead.Call).Return(nil, nil)
	repository.EXPECT().SaveSubmissionExportAudit(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	router := echo.New()
	handler := NewV1APIHandler(repository, fixedTokenRepository{token: token}, nil)
	handler.RegisterRoutes(router)
	path := "/v1/forms/" + formID + "/submissions/export"
	finished := make(chan *httptest.ResponseRecorder, 1)
	go func() { finished <- requestRawExport(router, path, `{"format":"json"}`, credential) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first export did not reach repository")
	}
	busy := requestRawExport(router, path, `{"format":"csv"}`, credential)
	require.Equal(t, http.StatusTooManyRequests, busy.Code, busy.Body.String())
	require.Equal(t, "1", busy.Header().Get("Retry-After"))
	require.Equal(t, "no-store", busy.Header().Get("Cache-Control"))
	require.Empty(t, busy.Header().Get("Content-Disposition"))
	require.Empty(t, busy.Header().Get("X-GoFormX-Export-ID"))
	require.NotContains(t, busy.Body.String(), `"data":`)
	// Admission is behind authentication, so capacity cannot disguise denial.
	anonymous := requestRawExport(router, path, `{"format":"json"}`, "")
	require.Equal(t, http.StatusUnauthorized, anonymous.Code)
	require.Empty(t, anonymous.Header().Get("Retry-After"))
	unblock()
	select {
	case response := <-finished:
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	case <-time.After(5 * time.Second):
		t.Fatal("first export did not finish")
	}
	require.False(t, handler.exportActive.Load())
	next := requestRawExport(router, path, `{"format":"csv"}`, credential)
	require.Equal(t, http.StatusOK, next.Code, next.Body.String())
	require.False(t, handler.exportActive.Load())
}
