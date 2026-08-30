package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/submission"
	mockform "github.com/goformx/goforms/test/mocks/form"
)

func TestSubmissionFiltersRejectMalformedSelectorsBeforeListing(t *testing.T) {
	t.Parallel()
	for _, query := range []string{
		"limit=0", "limit=101", "limit=1&limit=2", "cursor=invalid", "status=processing", "status=",
		"schemaVersion=0", "schemaVersion=-1", "schemaVersion=2147483648", "schemaVersion=1.5",
		"receivedFrom=2026-08-30", "receivedFrom=", "receivedBefore=private-synthetic-text",
		"receivedFrom=2026-08-30T00:00:00.000000001Z", "receivedFrom=0000-01-01T00:00:00Z",
		"receivedFrom=2026-08-30T00:00:00%2B24:00", "receivedFrom=2026-08-30T00:00:00%2B01:60",
		"receivedFrom=2026-08-30T00:00:00,123Z", "receivedFrom=2026-02-30T00:00:00Z",
		"receivedFrom=2026-08-31T00:00:00Z&receivedBefore=2026-08-30T00:00:00Z",
		"receivedFrom=2026-08-30T00:00:00Z&receivedBefore=2026-08-30T00:00:00Z",
		"organization_id=other-tenant", "payload=private-synthetic-text", "receivedFrom=%ZZ", "status=accepted;schemaVersion=2",
		"cursor=" + strings.Repeat("a", 4097),
	} {
		t.Run(query[:min(len(query), 90)], func(t *testing.T) {
			repository := mockform.NewMockRepository(gomock.NewController(t))
			token, credential, err := auth.Issue("owner-a", []auth.Scope{auth.ScopeSubmissionsRead}, time.Hour, time.Now())
			require.NoError(t, err)
			repository.EXPECT().GetFormByID(gomock.Any(), "owner-a", "form-a").Return(&model.Form{ID: "form-a", OrganizationID: "owner-a"}, nil)
			router := echo.New()
			NewV1APIHandler(repository, fixedTokenRepository{token: token}, nil).RegisterRoutes(router)
			request := httptest.NewRequest(http.MethodGet, "/v1/forms/form-a/submissions", nil)
			request.URL.RawQuery = query
			request.Header.Set("Authorization", "Bearer "+credential)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
			require.NotContains(t, response.Body.String(), "private-synthetic-text")
		})
	}
}

func TestSubmissionFiltersPassExactOptionsAndPreserveTimePrecision(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 8, 30, 12, 0, 0, 123456000, time.UTC)
	end := start.Add(time.Hour)
	last := &model.FormSubmission{ID: "11111111-1111-4111-8111-111111111111", SubmittedAt: start, Data: model.JSON{}}
	parameters := url.Values{"limit": {"2"}, "status": {"accepted"}, "schemaVersion": {"2"},
		"receivedFrom": {start.Format(time.RFC3339Nano)}, "receivedBefore": {end.Format(time.RFC3339Nano)},
		"cursor": {encodeSubmissionCursor(last)}}
	request := httptest.NewRequest(http.MethodGet, "/?"+parameters.Encode(), nil)
	options, err := submissionListOptions(echo.New().NewContext(request, httptest.NewRecorder()))
	require.NoError(t, err)
	require.Equal(t, submission.ListOptions{Limit: 2, Before: start, BeforeID: last.ID,
		ReceivedFrom: &start, ReceivedBefore: &end, Status: model.SubmissionStatusAccepted, SchemaVersion: 2}, options)
	last.FormID, last.SchemaVersion = "form-a", 1
	version, err := model.RestoreSchemaVersion("form-a", 1, model.JSON{}, model.SchemaVersionPublished, start, &start)
	require.NoError(t, err)
	resource, err := submissionResource(last, version)
	require.NoError(t, err)
	require.Equal(t, "2026-08-30T12:00:00.123456Z", resource.SubmittedAt)
	// Explicit offsets identify the same instant, independent of the server zone.
	parameters.Set("receivedFrom", "2026-08-30T08:00:00.123456-04:00")
	request.URL.RawQuery = parameters.Encode()
	options, err = submissionListOptions(echo.New().NewContext(request, httptest.NewRecorder()))
	require.NoError(t, err)
	require.True(t, options.ReceivedFrom.Equal(start))
}
