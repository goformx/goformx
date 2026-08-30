package web

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/domain/form/model"
)

func TestFormResourceAlwaysIncludesStoredAllowedOrigins(t *testing.T) {
	t.Parallel()
	for name, origins := range map[string]model.JSON{
		"unset":      nil,
		"empty":      {"origins": []string{}},
		"in-memory":  {"origins": []string{"https://example.test"}},
		"rehydrated": {"origins": []any{"https://example.test"}},
	} {
		t.Run(name, func(t *testing.T) {
			form := &model.Form{CorsOrigins: origins}
			encoded, err := json.Marshal(formResource(form))
			require.NoError(t, err)
			var resource map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(encoded, &resource))
			expected := `[]`
			if name == "in-memory" || name == "rehydrated" {
				expected = `["https://example.test"]`
			}
			require.JSONEq(t, expected, string(resource["allowedOrigins"]))
			require.NotContains(t, resource, "cors_origins")
		})
	}
}
