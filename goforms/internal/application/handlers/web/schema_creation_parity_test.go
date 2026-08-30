package web

import (
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/domain/form/model"
	mockform "github.com/goformx/goforms/test/mocks/form"
)

func TestFormCreationUsesCanonicalPropertySemanticsBeforePersistence(t *testing.T) {
	t.Parallel()
	for name, property := range map[string]any{
		"empty": map[string]any{}, "allow": true, "deny": false,
		"union": map[string]any{"type": []any{"string", "null"}},
		"null":  map[string]any{"type": "null"}, "reference": map[string]any{"$ref": "#/$defs/text"},
		"typed": map[string]any{"type": "string"},
	} {
		t.Run(name, func(t *testing.T) {
			repository := mockform.NewMockRepository(gomock.NewController(t))
			repository.EXPECT().CreateForm(gomock.Any(), gomock.Cond(func(candidate *model.Form) bool {
				return candidate.OrganizationID == "owner" && candidate.Schema["properties"] != nil
			})).Return(nil)
			token, credential, err := auth.Issue("owner", []auth.Scope{auth.ScopeFormsWrite}, time.Hour, time.Now())
			require.NoError(t, err)
			router := echo.New()
			NewV1APIHandler(repository, fixedTokenRepository{token: token}, nil).RegisterRoutes(router)
			schema := model.JSON{"$schema": model.JSONSchemaDraft202012URI, "type": "object",
				"properties": map[string]any{"value": property}, "$defs": map[string]any{"text": map[string]any{"type": "string"}}}
			response := requestJSON(t, router, http.MethodPost, "/v1/forms",
				map[string]any{"name": "canonical-form", "title": "Canonical form", "schema": schema}, credential, "", nil)
			require.Equal(t, http.StatusCreated, response.Code, response.Body.String())
		})
	}
}

func TestFormCreationRejectsInvalidCanonicalEnvelopeBeforePersistence(t *testing.T) {
	t.Parallel()
	for name, properties := range map[string]any{
		"missing": nil, "empty": map[string]any{}, "invalid type": map[string]any{"value": map[string]any{"type": "invented"}},
		"external ref": map[string]any{"value": map[string]any{"$ref": "https://example.test/schema"}},
	} {
		t.Run(name, func(t *testing.T) {
			repository := mockform.NewMockRepository(gomock.NewController(t)) // No persistence call is permitted.
			token, credential, err := auth.Issue("owner", []auth.Scope{auth.ScopeFormsWrite}, time.Hour, time.Now())
			require.NoError(t, err)
			router := echo.New()
			NewV1APIHandler(repository, fixedTokenRepository{token: token}, nil).RegisterRoutes(router)
			schema := model.JSON{"$schema": model.JSONSchemaDraft202012URI, "type": "object"}
			if properties != nil {
				schema["properties"] = properties
			}
			response := requestJSON(t, router, http.MethodPost, "/v1/forms",
				map[string]any{"name": "canonical-form", "title": "Canonical form", "schema": schema}, credential, "", nil)
			require.Equal(t, http.StatusUnprocessableEntity, response.Code, response.Body.String())
			require.Contains(t, response.Body.String(), "/schema")
		})
	}
}
