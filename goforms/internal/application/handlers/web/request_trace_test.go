package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/application/constants"
)

func TestRequestTraceIsStableAcrossAuditResponseAndLogging(t *testing.T) {
	t.Parallel()
	for _, supplied := range []string{"", "invalid\ntrace", "valid-caller-trace"} {
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.Header.Set(constants.HeaderTraceID, supplied)
		response := httptest.NewRecorder()
		c := echo.New().NewContext(request, response)
		initial := requestID(c)
		for range 3 {
			require.Equal(t, initial, requestID(c))
			require.Equal(t, initial, response.Header().Get(constants.HeaderTraceID))
		}
	}
}
