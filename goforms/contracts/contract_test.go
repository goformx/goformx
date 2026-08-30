package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"github.com/goformx/goforms/contracts"
)

type operation struct {
	OperationID    string                `yaml:"operationId"`
	Security       []map[string][]string `yaml:"security"`
	RequiredScopes []string              `yaml:"x-goformx-required-scopes"`
	Parameters     []parameter           `yaml:"parameters"`
	Responses      map[string]any        `yaml:"responses"`
}

type parameter struct {
	Ref string `yaml:"$ref"`
}

type pathItem struct {
	Get    *operation `yaml:"get"`
	Post   *operation `yaml:"post"`
	Patch  *operation `yaml:"patch"`
	Put    *operation `yaml:"put"`
	Delete *operation `yaml:"delete"`
}

type contract struct {
	OpenAPI           string              `yaml:"openapi"`
	JSONSchemaDialect string              `yaml:"jsonSchemaDialect"`
	Paths             map[string]pathItem `yaml:"paths"`
	Components        struct {
		SecuritySchemes map[string]securityScheme `yaml:"securitySchemes"`
	} `yaml:"components"`
}

type securityScheme struct {
	Type            string `yaml:"type"`
	Scheme          string `yaml:"scheme"`
	BearerFormat    string `yaml:"bearerFormat"`
	CredentialClass string `yaml:"x-goformx-credential-class"`
	JWTProfile      string `yaml:"x-goformx-jwt-profile"`
	JWKSURI         string `yaml:"x-goformx-jwks-uri"`
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
	allowedScopes := map[string]struct{}{
		"forms:read": {}, "forms:write": {}, "forms:publish": {}, "submissions:read": {},
		"tokens:read": {}, "tokens:write": {}, "webhooks:read": {}, "webhooks:write": {},
	}
	for path, item := range api.Paths {
		for method, op := range map[string]*operation{
			"get": item.Get, "post": item.Post, "patch": item.Patch, "put": item.Put, "delete": item.Delete,
		} {
			if op == nil {
				continue
			}
			require.NotEmpty(t, op.OperationID, "%s %s needs operationId", method, path)
			_, duplicate := seenIDs[op.OperationID]
			require.False(t, duplicate, "duplicate operationId %s", op.OperationID)
			seenIDs[op.OperationID] = struct{}{}
			require.NotEmpty(t, op.Responses, "%s %s needs responses", method, path)
			require.Contains(t, op.Responses, "default", "%s %s needs stable error semantics", method, path)
			switch op.OperationID {
			case "listForms":
				require.Equal(t, []string{
					"#/components/parameters/FormStatusFilter", "#/components/parameters/FormQuery",
					"#/components/parameters/FormSort", "#/components/parameters/PageLimit",
					"#/components/parameters/PageOffset",
				}, parameterRefs(op.Parameters))
			case "listSchemaVersions":
				require.Equal(t, []string{
					"#/components/parameters/PageLimit", "#/components/parameters/PageOffset",
				}, parameterRefs(op.Parameters))
			case "updateForm":
				require.Equal(t, []string{"#/components/parameters/IfMatch"}, parameterRefs(op.Parameters))
				require.Contains(t, op.Responses, "412")
				require.Contains(t, op.Responses, "428")
			}

			if strings.HasPrefix(path, "/v1/public/") || path == "/health" || path == "/ready" {
				require.Empty(t, op.Security, "%s %s must remain public", method, path)
				require.Empty(t, op.RequiredScopes, "%s %s must not require a management scope", method, path)
			} else {
				require.Equal(t, []map[string][]string{
					{"serviceToken": {}},
					{"firstPartyAssertion": {}},
				}, op.Security, "%s %s must accept either credential class without making them interchangeable", method, path)
				require.Len(t, op.RequiredScopes, 1, "%s %s must declare exactly one canonical scope", method, path)
				_, allowed := allowedScopes[op.RequiredScopes[0]]
				require.True(t, allowed, "%s %s declares unknown scope %q", method, path, op.RequiredScopes[0])
			}
		}
	}

	serviceToken := api.Components.SecuritySchemes["serviceToken"]
	require.Equal(t, "http", serviceToken.Type)
	require.Equal(t, "bearer", serviceToken.Scheme)
	require.Equal(t, "gfst_ opaque token", serviceToken.BearerFormat)
	require.Equal(t, "external-service-token", serviceToken.CredentialClass)
	require.Empty(t, serviceToken.JWTProfile)

	assertion := api.Components.SecuritySchemes["firstPartyAssertion"]
	require.Equal(t, "http", assertion.Type)
	require.Equal(t, "bearer", assertion.Scheme)
	require.Equal(t, "JWT (EdDSA)", assertion.BearerFormat)
	require.Equal(t, "first-party-assertion", assertion.CredentialClass)
	require.Equal(t, "gofx-fpa-v1", assertion.JWTProfile)
	require.Equal(t, "https://goformx.com/.well-known/goformx-control-plane-jwks.json", assertion.JWKSURI)
}

func parameterRefs(parameters []parameter) []string {
	refs := make([]string, 0, len(parameters))
	for _, item := range parameters {
		refs = append(refs, item.Ref)
	}
	return refs
}

func TestFirstPartyAssertionContractAndNegativeFixtures(t *testing.T) {
	t.Parallel()

	schemaDocument, err := os.ReadFile(filepath.Join("auth", "first-party-assertion.claims.schema.json"))
	require.NoError(t, err)

	var schema struct {
		Dialect              string                     `json:"$schema"`
		AdditionalProperties bool                       `json:"additionalProperties"`
		Required             []string                   `json:"required"`
		Properties           map[string]json.RawMessage `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(schemaDocument, &schema))
	require.Equal(t, "https://json-schema.org/draft/2020-12/schema", schema.Dialect)
	require.False(t, schema.AdditionalProperties)
	require.ElementsMatch(t,
		[]string{"iss", "aud", "sub", "org", "scp", "iat", "nbf", "exp", "jti", "rid", "ver"},
		schema.Required,
	)
	require.Len(t, schema.Properties, 11)
	require.JSONEq(t, `{"const":"https://goformx.com"}`, string(schema.Properties["iss"]))
	require.JSONEq(t, `{"const":"https://api.goformx.com"}`, string(schema.Properties["aud"]))
	require.JSONEq(t, `{"const":1}`, string(schema.Properties["ver"]))
	require.JSONEq(t, `{
		"type":"array",
		"minItems":1,
		"uniqueItems":true,
		"items":{"enum":[
			"forms:read","forms:write","forms:publish","submissions:read",
			"tokens:read","tokens:write","webhooks:read","webhooks:write"
		]}
	}`, string(schema.Properties["scp"]))
	var schemaResource any
	require.NoError(t, json.Unmarshal(schemaDocument, &schemaResource))
	compiler := jsonschema.NewCompiler()
	require.NoError(t, compiler.AddResource("first-party-assertion.claims.schema.json", schemaResource))
	compiledSchema, err := compiler.Compile("first-party-assertion.claims.schema.json")
	require.NoError(t, err)

	fixtureDocument, err := os.ReadFile(filepath.Join("auth", "first-party-assertion.examples.json"))
	require.NoError(t, err)

	var fixtures struct {
		Profile string `json:"profile"`
		Valid   struct {
			Header map[string]any `json:"header"`
			Claims map[string]any `json:"claims"`
		} `json:"valid"`
		Negative []struct {
			Name string `json:"name"`
		} `json:"negative"`
	}
	require.NoError(t, json.Unmarshal(fixtureDocument, &fixtures))
	require.Equal(t, "gofx-fpa-v1", fixtures.Profile)
	require.Equal(t, "EdDSA", fixtures.Valid.Header["alg"])
	require.Equal(t, "gofx-fpa+jwt", fixtures.Valid.Header["typ"])
	require.Equal(t, "https://goformx.com", fixtures.Valid.Claims["iss"])
	require.Equal(t, "https://api.goformx.com", fixtures.Valid.Claims["aud"])
	require.NoError(t, compiledSchema.Validate(fixtures.Valid.Claims))
	require.Equal(t, float64(60), fixtures.Valid.Claims["exp"].(float64)-fixtures.Valid.Claims["iat"].(float64))

	negativeNames := make([]string, 0, len(fixtures.Negative))
	for _, fixture := range fixtures.Negative {
		negativeNames = append(negativeNames, fixture.Name)
	}
	require.ElementsMatch(t, []string{
		"wrong-issuer", "wrong-audience", "wrong-organization", "missing-scope", "expired", "replayed-jti", "revoked-key",
	}, negativeNames)
}

func TestCanonicalSchemaAndReferencesExist(t *testing.T) {
	t.Parallel()

	schemaPath := filepath.Join("schema", "form-definition.schema.json")
	document, err := os.ReadFile(schemaPath)
	require.NoError(t, err)
	require.Equal(t, string(document), contracts.FormDefinition(), "runtime validation must embed the published source contract")

	var schema map[string]any
	require.NoError(t, json.Unmarshal(document, &schema))
	require.Equal(t, "https://json-schema.org/draft/2020-12/schema", schema["$schema"])
	require.Equal(t, "object", schema["type"])
	require.NotEmpty(t, schema["examples"])

	openAPI, err := os.ReadFile("openapi.v1.yaml")
	require.NoError(t, err)
	require.Contains(t, string(openAPI), "./schema/form-definition.schema.json")
}

func TestPublishedOpenAPIExamplesMatchTheirSchemas(t *testing.T) {
	t.Parallel()
	document, err := os.ReadFile("generated/openapi.json")
	require.NoError(t, err)
	var api map[string]any
	require.NoError(t, json.Unmarshal(document, &api))
	tested := 0
	var visit func(any, string)
	visit = func(value any, path string) {
		switch node := value.(type) {
		case map[string]any:
			if schema, hasSchema := node["schema"].(map[string]any); hasSchema {
				check := func(name string, example any) {
					tested++
					t.Run(path+"/"+name, func(t *testing.T) {
						compiler := jsonschema.NewCompiler()
						resource := map[string]any{
							"$schema":    "https://json-schema.org/draft/2020-12/schema",
							"components": api["components"], "allOf": []any{schema},
						}
						require.NoError(t, compiler.AddResource("example.json", resource))
						compiled, compileErr := compiler.Compile("example.json")
						require.NoError(t, compileErr)
						require.NoError(t, compiled.Validate(example))
					})
				}
				if example, present := node["example"]; present {
					check("example", example)
				}
				if examples, present := node["examples"].(map[string]any); present {
					for name, candidate := range examples {
						example, ok := candidate.(map[string]any)
						require.True(t, ok, "example %s must be inline and machine-testable", name)
						value, ok := example["value"]
						require.True(t, ok, "example %s needs a value; external examples are not covered", name)
						check(name, value)
					}
				}
			}
			for key, child := range node {
				visit(child, path+"/"+key)
			}
		case []any:
			for _, child := range node {
				visit(child, path+"/item")
			}
		}
	}
	visit(api, "openapi")
	require.GreaterOrEqual(t, tested, 3, "published request and error examples must not silently disappear")
}
