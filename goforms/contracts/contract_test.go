package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

type operation struct {
	OperationID string                `yaml:"operationId"`
	Security    []map[string][]string `yaml:"security"`
	Responses   map[string]any        `yaml:"responses"`
}

type pathItem struct {
	Get   *operation `yaml:"get"`
	Post  *operation `yaml:"post"`
	Patch *operation `yaml:"patch"`
}

type contract struct {
	OpenAPI           string              `yaml:"openapi"`
	JSONSchemaDialect string              `yaml:"jsonSchemaDialect"`
	Paths             map[string]pathItem `yaml:"paths"`
}

func TestV1ContractDeclaresCanonicalDialectAndOperationSemantics(t *testing.T) {
	t.Parallel()

	document, err := os.ReadFile("openapi.v1.yaml")
	require.NoError(t, err)

	var api contract
	require.NoError(t, yaml.Unmarshal(document, &api))
	require.Equal(t, "3.1.0", api.OpenAPI)
	require.Equal(t, "https://json-schema.org/draft/2020-12/schema", api.JSONSchemaDialect)
	require.NotEmpty(t, api.Paths)

	seenIDs := make(map[string]struct{})
	for path, item := range api.Paths {
		for method, op := range map[string]*operation{"get": item.Get, "post": item.Post, "patch": item.Patch} {
			if op == nil {
				continue
			}
			require.NotEmpty(t, op.OperationID, "%s %s needs operationId", method, path)
			_, duplicate := seenIDs[op.OperationID]
			require.False(t, duplicate, "duplicate operationId %s", op.OperationID)
			seenIDs[op.OperationID] = struct{}{}
			require.NotEmpty(t, op.Responses, "%s %s needs responses", method, path)
			require.Contains(t, op.Responses, "default", "%s %s needs stable error semantics", method, path)

			if strings.HasPrefix(path, "/v1/public/") || path == "/health" || path == "/ready" {
				require.Empty(t, op.Security, "%s %s must remain public", method, path)
			} else {
				require.NotEmpty(t, op.Security, "%s %s must declare scoped auth", method, path)
			}
		}
	}
}

func TestCanonicalSchemaAndReferencesExist(t *testing.T) {
	t.Parallel()

	schemaPath := filepath.Join("schema", "form-definition.schema.json")
	document, err := os.ReadFile(schemaPath)
	require.NoError(t, err)

	var schema map[string]any
	require.NoError(t, json.Unmarshal(document, &schema))
	require.Equal(t, "https://json-schema.org/draft/2020-12/schema", schema["$schema"])
	require.Equal(t, "object", schema["type"])
	require.NotEmpty(t, schema["examples"])

	openAPI, err := os.ReadFile("openapi.v1.yaml")
	require.NoError(t, err)
	require.Contains(t, string(openAPI), "./schema/form-definition.schema.json")
}
