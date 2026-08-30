package validation_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/application/validation"
	"github.com/goformx/goforms/internal/domain/form/model"
)

func TestFormAndVersionAcceptTheSameCanonicalPropertySchemas(t *testing.T) {
	t.Parallel()
	for name, property := range map[string]any{
		"empty": map[string]any{}, "boolean true": true, "boolean false": false,
		"nullable union": map[string]any{"type": []any{"string", "null"}},
		"null":           map[string]any{"type": "null"}, "local reference": map[string]any{"$ref": "#/$defs/text"},
		"typed": map[string]any{"type": "string"},
	} {
		t.Run(name, func(t *testing.T) {
			schema := model.JSON{"$schema": model.JSONSchemaDraft202012URI, "type": "object",
				"properties": map[string]any{"value": property}, "$defs": map[string]any{"text": map[string]any{"type": "string"}}}
			validator := validation.NewComprehensiveValidator()
			require.NoError(t, validator.ValidateDefinition(schema))
			require.NoError(t, model.NewForm("organization", "Canonical form", "", schema).Validate(validator))
			version, err := model.NewSchemaVersion("form", 1, schema, validator)
			require.NoError(t, err)
			published, err := version.Publish(version.CreatedAt())
			require.NoError(t, err)
			require.True(t, validator.ValidateVersion(published, model.JSON{}).IsValid)
		})
	}
}

func TestPublishedDefinitionEnvelopeIsEnforcedByTheCanonicalValidator(t *testing.T) {
	t.Parallel()
	for name, properties := range map[string]any{"missing": nil, "empty": map[string]any{}, "array": []any{}, "invalid property": map[string]any{"value": 42}} {
		t.Run(name, func(t *testing.T) {
			schema := model.JSON{"$schema": model.JSONSchemaDraft202012URI, "type": "object"}
			if properties != nil {
				schema["properties"] = properties
			}
			validator := validation.NewComprehensiveValidator()
			result := validator.ValidateSchema(schema)
			require.False(t, result.IsValid)
			require.NotEmpty(t, result.Errors)
			require.NotEmpty(t, result.Errors[0].Pointer)
			require.Error(t, model.NewForm("organization", "Invalid form", "", schema).Validate(validator))
			_, err := model.NewSchemaVersion("form", 1, schema, validator)
			require.Error(t, err)
		})
	}
}
