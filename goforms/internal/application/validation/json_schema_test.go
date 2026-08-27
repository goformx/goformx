package validation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/application/validation"
	"github.com/goformx/goforms/internal/domain/form/model"
)

func canonicalSchema() model.JSON {
	return model.JSON{
		"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
		"properties": map[string]any{
			"email": map[string]any{"type": "string", "format": "email"},
			"profile": map[string]any{"type": "object", "properties": map[string]any{
				"name": map[string]any{"type": "string", "minLength": 2},
			}, "required": []any{"name"}, "additionalProperties": false},
			"tags": map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 2}},
		},
		"required": []any{"email", "profile"}, "additionalProperties": false,
	}
}

func TestCanonicalValidatorAcceptsNestedValidSubmission(t *testing.T) {
	t.Parallel()
	result := validation.NewComprehensiveValidator().ValidateForm(canonicalSchema(), model.JSON{
		"email": "ada@example.com", "profile": map[string]any{"name": "Ada"}, "tags": []any{"go", "ai"},
	})
	require.True(t, result.IsValid, "%+v", result.Errors)
	require.Empty(t, result.Errors)
}

func TestCanonicalValidatorReturnsPathAddressableErrors(t *testing.T) {
	t.Parallel()
	result := validation.NewComprehensiveValidator().ValidateForm(canonicalSchema(), model.JSON{
		"email": "not-an-email", "profile": map[string]any{"unexpected": true},
		"tags": []any{"ok", "x"}, "extra": true,
	})
	require.False(t, result.IsValid)
	pointers := make([]string, 0, len(result.Errors))
	for _, err := range result.Errors {
		pointers = append(pointers, err.Pointer)
		assert.NotEmpty(t, err.Code)
		assert.NotEmpty(t, err.Message)
	}
	assert.Contains(t, pointers, "/email")
	assert.Contains(t, pointers, "/profile")
	assert.Contains(t, pointers, "/tags/1")
	assert.Contains(t, pointers, "/")
}

func TestCanonicalValidatorRejectsInvalidOrWrongDialectSchemas(t *testing.T) {
	t.Parallel()
	validator := validation.NewComprehensiveValidator()
	wrongDialect := canonicalSchema()
	delete(wrongDialect, "$schema")
	result := validator.ValidateSchema(wrongDialect)
	require.False(t, result.IsValid)
	require.Equal(t, "/$schema", result.Errors[0].Pointer)
	invalid := canonicalSchema()
	invalid["required"] = "email"
	result = validator.ValidateSchema(invalid)
	require.False(t, result.IsValid)
	require.Equal(t, "invalid_schema", result.Errors[0].Code)
}
