package web //nolint:testpackage // internal test for unexported handler methods

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/goformx/goforms/internal/application/response"
	mockform "github.com/goformx/goforms/test/mocks/form"
	mocklogging "github.com/goformx/goforms/test/mocks/logging"
	mocksanitization "github.com/goformx/goforms/test/mocks/sanitization"
)

// buildUsageHandler constructs a minimal FormAPIHandler for testing usage endpoints.
func buildUsageHandler(
	t *testing.T,
	formService *mockform.MockService,
	logger *mocklogging.MockLogger,
) *FormAPIHandler {
	t.Helper()

	ctrl := gomock.NewController(t)
	sanitizer := mocksanitization.NewMockService(ctrl)
	errHandler := response.NewErrorHandler(logger, sanitizer)

	base := &BaseHandler{
		Logger:       logger,
		ErrorHandler: errHandler,
	}

	formBase := NewFormBaseHandler(base, formService, nil)

	return &FormAPIHandler{
		FormBaseHandler: formBase,
	}
}

func TestHandleFormsCount_ReturnsCount(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	formService := mockform.NewMockService(ctrl)
	logger := mocklogging.NewMockLogger(ctrl)

	formService.EXPECT().CountFormsByUser(gomock.Any(), "user123").Return(5, nil)

	handler := buildUsageHandler(t, formService, logger)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/forms/usage/forms-count", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "user123")

	err := handler.handleFormsCount(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.APIResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, float64(5), data["count"], 0)
}

func TestHandleSubmissionsCount_ReturnsMonthlyCount(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	formService := mockform.NewMockService(ctrl)
	logger := mocklogging.NewMockLogger(ctrl)

	const expectedCount = 150
	formService.EXPECT().CountSubmissionsByUserMonth(gomock.Any(), "user123", 2026, 2).Return(expectedCount, nil)

	handler := buildUsageHandler(t, formService, logger)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/forms/usage/submissions-count?month=2026-02", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "user123")

	err := handler.handleSubmissionsCount(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp response.APIResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.InDelta(t, float64(expectedCount), data["count"], 0)
	assert.Equal(t, "2026-02", data["month"])
}

func TestHandleSubmissionsCount_InvalidMonth_ReturnsBadRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	formService := mockform.NewMockService(ctrl)
	logger := mocklogging.NewMockLogger(ctrl)

	handler := buildUsageHandler(t, formService, logger)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/forms/usage/submissions-count?month=invalid", http.NoBody)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", "user123")

	err := handler.handleSubmissionsCount(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var resp response.APIResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
}

func TestParseYearMonth(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantYear  int
		wantMonth int
		wantErr   bool
	}{
		{"valid", "2026-02", 2026, 2, false},
		{"december", "2025-12", 2025, 12, false},
		{"january", "2026-01", 2026, 1, false},
		{"invalid format", "202602", 0, 0, true},
		{"empty", "", 0, 0, true},
		{"month too high", "2026-13", 0, 0, true},
		{"month zero", "2026-00", 0, 0, true},
		{"bad year", "abc-02", 0, 0, true},
		{"bad month", "2026-xy", 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			year, month, err := parseYearMonth(tt.input)
			if tt.wantErr {
				assert.Error(t, err)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantYear, year)
			assert.Equal(t, tt.wantMonth, month)
		})
	}
}
