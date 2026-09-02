package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/domain/form/model"
	repositorycommon "github.com/goformx/goforms/internal/infrastructure/repository/common"
)

func TestRepositoryErrorsUseStableCategoriesNotMessageText(t *testing.T) {
	tests := map[string]struct {
		err    error
		status int
		code   string
	}{
		"wrapped not found":  {fmt.Errorf("outer: %w", repositorycommon.NewNotFoundError("get", "form", "id")), http.StatusNotFound, "not_found"},
		"wrapped conflict":   {fmt.Errorf("outer: %w", repositorycommon.NewConflictError("create", "form", "id", errors.New("private"))), http.StatusConflict, "conflict"},
		"wrapped invalid":    {fmt.Errorf("outer: %w", repositorycommon.NewInvalidInputError("get", "form", "id", errors.New("private"))), http.StatusBadRequest, "invalid_request"},
		"precondition":       {fmt.Errorf("outer: %w", model.ErrPreconditionFailed), http.StatusPreconditionFailed, "precondition_failed"},
		"misleading unknown": {errors.New("record not found duplicate unique invalid input private-canary"), http.StatusInternalServerError, "internal_error"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/", nil), recorder)
			require.NoError(t, (&V1APIHandler{}).writeRepositoryError(context, test.err))
			require.Equal(t, test.status, recorder.Code)
			require.Contains(t, recorder.Body.String(), `"code":"`+test.code+`"`)
			require.NotContains(t, recorder.Body.String(), "private-canary")
		})
	}
}
