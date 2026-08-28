package validation

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/domain/form/model"
)

func securitySchema() model.JSON {
	return model.JSON{
		"$schema": model.JSONSchemaDraft202012URI,
		"type":    "object",
		"properties": map[string]any{
			"email": map[string]any{"type": "string", "format": "email"},
		},
	}
}

func TestSchemaPolicyRejectsNetworkReferences(t *testing.T) {
	t.Parallel()
	for _, keyword := range []string{"$ref", "$dynamicRef", "$recursiveRef"} {
		t.Run(keyword, func(t *testing.T) {
			schema := securitySchema()
			schema["properties"] = map[string]any{
				"profile": map[string]any{keyword: "http://169.254.169.254/latest/meta-data"},
			}
			result := NewComprehensiveValidator().ValidateSchema(schema)
			require.False(t, result.IsValid)
			require.Equal(t, "external_reference", result.Errors[0].Code)
			require.Equal(t, "/properties/profile/"+escapePointerToken(keyword), result.Errors[0].Pointer)
		})
	}

	local := securitySchema()
	local["$defs"] = map[string]any{"email": map[string]any{"type": "string"}}
	local["properties"] = map[string]any{"email": map[string]any{"$ref": "#/$defs/email"}}
	require.True(t, NewComprehensiveValidator().ValidateSchema(local).IsValid)
}

func TestSchemaPolicyRejectsExcessiveDepthNodesAndPatterns(t *testing.T) {
	t.Parallel()

	deep := securitySchema()
	current := map[string]any{"type": "object"}
	deep["properties"] = map[string]any{"root": current}
	for depth := 0; depth <= maxSchemaDepth; depth++ {
		next := map[string]any{"type": "object"}
		current["properties"] = map[string]any{"child": next}
		current = next
	}
	result := NewComprehensiveValidator().ValidateSchema(deep)
	require.False(t, result.IsValid)
	require.Equal(t, "schema_too_deep", result.Errors[0].Code)

	wide := securitySchema()
	properties := make(map[string]any, maxSchemaNodes)
	for index := 0; index < maxSchemaNodes; index++ {
		properties[fmt.Sprintf("field_%d", index)] = map[string]any{"type": "string"}
	}
	wide["properties"] = properties
	result = NewComprehensiveValidator().ValidateSchema(wide)
	require.False(t, result.IsValid)
	require.Equal(t, "schema_too_complex", result.Errors[0].Code)

	pattern := securitySchema()
	pattern["properties"] = map[string]any{
		"value": map[string]any{"type": "string", "pattern": string(make([]byte, maxPatternLength+1))},
	}
	result = NewComprehensiveValidator().ValidateSchema(pattern)
	require.False(t, result.IsValid)
	require.Equal(t, "pattern_too_long", result.Errors[0].Code)
}

func TestCompiledSchemaCacheIsBoundedAndConcurrent(t *testing.T) {
	t.Parallel()
	validator := NewComprehensiveValidator()
	schema := securitySchema()

	var wait sync.WaitGroup
	for range 32 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result := validator.ValidateForm(schema, model.JSON{"email": "ada@example.com"})
			require.True(t, result.IsValid)
		}()
	}
	wait.Wait()
	require.Len(t, validator.compiled, 1)

	for index := 0; index < maxCompiledCache+10; index++ {
		variant := securitySchema()
		variant["title"] = fmt.Sprintf("schema-%d", index)
		require.True(t, validator.ValidateSchema(variant).IsValid)
	}
	require.Len(t, validator.compiled, maxCompiledCache)
}
