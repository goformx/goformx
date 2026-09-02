package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/application/constants"
	"github.com/goformx/goforms/internal/application/middleware/serviceauth"
	"github.com/goformx/goforms/internal/domain/auth"
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

func TestManagementAuditIdentitySeparatesAuthenticatedRequestsFromCallerCorrelation(t *testing.T) {
	t.Parallel()
	servicePrincipal := serviceauth.Principal{CredentialClass: auth.CredentialClassServiceToken}
	contextFor := func(trace string) echo.Context {
		request := httptest.NewRequest(http.MethodPost, "/", nil)
		request.Header.Set(constants.HeaderTraceID, trace)
		return echo.New().NewContext(request, httptest.NewRecorder())
	}

	first := contextFor("repeated-caller-trace")
	firstRequest, firstCorrelation := managementAuditRequestIdentity(first, servicePrincipal)
	repeatedRequest, repeatedCorrelation := managementAuditRequestIdentity(first, servicePrincipal)
	require.Equal(t, firstRequest, repeatedRequest, "one request must keep one audit request identity")
	require.Equal(t, "repeated-caller-trace", firstCorrelation)
	require.Equal(t, firstCorrelation, repeatedCorrelation)

	secondRequest, secondCorrelation := managementAuditRequestIdentity(contextFor("repeated-caller-trace"), servicePrincipal)
	require.NotEqual(t, firstRequest, secondRequest, "caller correlation cannot choose event request identity")
	require.Equal(t, firstCorrelation, secondCorrelation)

	invalidRequest, invalidCorrelation := managementAuditRequestIdentity(contextFor("not valid!"), servicePrincipal)
	require.NotEmpty(t, invalidRequest)
	require.Empty(t, invalidCorrelation)

	signed := "44444444-4444-4444-8444-444444444444"
	assertionRequest, assertionCorrelation := managementAuditRequestIdentity(contextFor("ignored-caller-trace"),
		serviceauth.Principal{CredentialClass: auth.CredentialClassFirstPartyAssertion, RequestID: signed})
	require.Equal(t, signed, assertionRequest)
	require.Empty(t, assertionCorrelation)
}
