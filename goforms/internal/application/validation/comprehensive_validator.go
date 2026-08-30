// Package validation validates canonical form definitions and submissions.
package validation

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/goformx/goforms/contracts"
	"github.com/goformx/goforms/internal/domain/form/model"
	"github.com/goformx/goforms/internal/domain/submission"
)

const (
	canonicalDialect = model.JSONSchemaDraft202012URI
	resourceURL      = "https://goformx.com/runtime/form-definition.json"
	maxSchemaDepth   = 32
	maxSchemaNodes   = 4096
	maxPatternLength = 512
	maxCompiledCache = 128
)

// ComprehensiveValidator wraps the maintained Draft 2020-12 implementation used by the core path.
type ComprehensiveValidator struct {
	cacheMu    sync.RWMutex
	compiled   map[[sha256.Size]byte]*jsonschema.Schema
	cacheOrder [][sha256.Size]byte
}

// NewComprehensiveValidator creates a canonical JSON Schema validator.
func NewComprehensiveValidator() *ComprehensiveValidator {
	return &ComprehensiveValidator{compiled: make(map[[sha256.Size]byte]*jsonschema.Schema)}
}

// ValidateDefinition implements the domain schema-compilation boundary.
func (v *ComprehensiveValidator) ValidateDefinition(schema model.JSON) error {
	result := v.ValidateSchema(schema)
	if result.IsValid {
		return nil
	}
	return fmt.Errorf("%s: %s", result.Errors[0].Code, result.Errors[0].Message)
}

// ValidateVersion validates against the immutable version supplied by the caller.
func (v *ComprehensiveValidator) ValidateVersion(version *model.SchemaVersion, submission model.JSON) Result {
	if version == nil {
		return invalidResult("/", "schema_version_required", "schema version is required")
	}
	return v.ValidateForm(version.Schema(), submission)
}

// ValidateSchema compiles a form definition without validating an instance.
func (v *ComprehensiveValidator) ValidateSchema(schema model.JSON) Result {
	_, result := v.compile(schema)
	return result
}

// ValidateForm validates submission data against the exact schema supplied by the caller.
func (v *ComprehensiveValidator) ValidateForm(schema, submission model.JSON) Result {
	compiled, result := v.compile(schema)
	if !result.IsValid {
		return result
	}
	if err := compiled.Validate(map[string]any(submission)); err != nil {
		var validationErr *jsonschema.ValidationError
		if !errors.As(err, &validationErr) {
			return invalidResult("/", "validation_failed", err.Error())
		}
		return Result{IsValid: false, Errors: flattenValidationError(validationErr)}
	}
	return Result{IsValid: true, Errors: []Error{}}
}

// GenerateClientValidation returns the canonical schema itself. Renderers derive constraints from JSON Schema.
func (v *ComprehensiveValidator) GenerateClientValidation(schema model.JSON) (map[string]any, error) {
	result := v.ValidateSchema(schema)
	if !result.IsValid {
		return nil, fmt.Errorf("invalid canonical schema: %s", result.Errors[0].Message)
	}
	return map[string]any{"dialect": canonicalDialect, "schema": map[string]any(schema)}, nil
}

func (v *ComprehensiveValidator) compile(schema model.JSON) (*jsonschema.Schema, Result) {
	if schema == nil {
		return nil, invalidResult("/", "invalid_schema", "schema is required")
	}
	dialect, ok := schema["$schema"].(string)
	if !ok || dialect != canonicalDialect {
		return nil, invalidResult("/$schema", "invalid_dialect", "schema must declare JSON Schema Draft 2020-12")
	}
	if schemaType, ok := schema["type"].(string); !ok || schemaType != "object" {
		return nil, invalidResult("/type", "invalid_type", "form schema type must be object")
	}
	if _, err := submission.SensitivePaths(schema); err != nil {
		return nil, invalidResult("/"+submission.SensitiveAnnotation, "invalid_redaction_policy", "sensitive paths must be unique bounded JSON Pointers")
	}
	if policyError := inspectSchemaPolicy(map[string]any(schema), "", 0, new(int)); policyError != nil {
		return nil, Result{IsValid: false, Errors: []Error{*policyError}}
	}
	digestSource, err := json.Marshal(schema)
	if err != nil {
		return nil, invalidResult("/", "invalid_schema", "schema must contain JSON-compatible values")
	}
	digest := sha256.Sum256(digestSource)
	if compiled := v.cached(digest); compiled != nil {
		return compiled, Result{IsValid: true, Errors: []Error{}}
	}
	definition, err := compiledDefinitionContract()
	if err != nil {
		return nil, invalidResult("/", "invalid_schema", "canonical definition contract could not be compiled")
	}
	if err := definition.Validate(map[string]any(schema)); err != nil {
		var validationErr *jsonschema.ValidationError
		if errors.As(err, &validationErr) {
			items := flattenValidationError(validationErr)
			for index := range items {
				items[index].Code = "invalid_schema"
			}
			return nil, Result{IsValid: false, Errors: items}
		}
		return nil, invalidResult("/", "invalid_schema", err.Error())
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.AssertFormat()
	if err := compiler.AddResource(resourceURL, map[string]any(schema)); err != nil {
		return nil, invalidResult("/", "invalid_schema", err.Error())
	}
	compiled, err := compiler.Compile(resourceURL)
	if err != nil {
		return nil, invalidResult("/", "invalid_schema", err.Error())
	}
	v.remember(digest, compiled)
	return compiled, Result{IsValid: true, Errors: []Error{}}
}

// Compile the published envelope once. Draft metaschema references are provided
// by the maintained compiler itself; no deployment filesystem or HTTP fetch is needed.
var compiledDefinitionContract = sync.OnceValues(func() (*jsonschema.Schema, error) {
	var document map[string]any
	if err := json.Unmarshal([]byte(contracts.FormDefinition()), &document); err != nil {
		return nil, err
	}
	id, ok := document["$id"].(string)
	if !ok || id == "" {
		return nil, errors.New("canonical definition contract must declare an ID")
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	compiler.AssertFormat()
	if err := compiler.AddResource(id, document); err != nil {
		return nil, err
	}
	return compiler.Compile(id)
})

func (v *ComprehensiveValidator) cached(digest [sha256.Size]byte) *jsonschema.Schema {
	v.cacheMu.RLock()
	defer v.cacheMu.RUnlock()
	return v.compiled[digest]
}

func (v *ComprehensiveValidator) remember(digest [sha256.Size]byte, compiled *jsonschema.Schema) {
	v.cacheMu.Lock()
	defer v.cacheMu.Unlock()
	if _, exists := v.compiled[digest]; exists {
		return
	}
	if len(v.cacheOrder) == maxCompiledCache {
		delete(v.compiled, v.cacheOrder[0])
		v.cacheOrder = v.cacheOrder[1:]
	}
	v.compiled[digest] = compiled
	v.cacheOrder = append(v.cacheOrder, digest)
}

func inspectSchemaPolicy(value any, pointer string, depth int, nodes *int) *Error {
	if depth > maxSchemaDepth {
		return &Error{Pointer: schemaPointer(pointer), Code: "schema_too_deep",
			Message: fmt.Sprintf("schema nesting must not exceed %d levels", maxSchemaDepth)}
	}
	*nodes++
	if *nodes > maxSchemaNodes {
		return &Error{Pointer: schemaPointer(pointer), Code: "schema_too_complex",
			Message: fmt.Sprintf("schema must not exceed %d nodes", maxSchemaNodes)}
	}

	switch typed := value.(type) {
	case model.JSON:
		return inspectSchemaObject(map[string]any(typed), pointer, depth, nodes)
	case map[string]any:
		return inspectSchemaObject(typed, pointer, depth, nodes)
	case []any:
		for index, item := range typed {
			if policyError := inspectSchemaPolicy(item, pointer+"/"+fmt.Sprint(index), depth+1, nodes); policyError != nil {
				return policyError
			}
		}
	}
	return nil
}

func inspectSchemaObject(object map[string]any, pointer string, depth int, nodes *int) *Error {
	for key, value := range object {
		childPointer := pointer + "/" + escapePointerToken(key)
		if key == "$ref" || key == "$dynamicRef" || key == "$recursiveRef" {
			reference, ok := value.(string)
			if !ok || !strings.HasPrefix(reference, "#") {
				return &Error{Pointer: schemaPointer(childPointer), Code: "external_reference",
					Message: "schema references must use a local JSON Pointer fragment"}
			}
		}
		if key == "pattern" {
			pattern, ok := value.(string)
			if !ok || len(pattern) > maxPatternLength {
				return &Error{Pointer: schemaPointer(childPointer), Code: "pattern_too_long",
					Message: fmt.Sprintf("schema patterns must not exceed %d bytes", maxPatternLength)}
			}
		}
		if policyError := inspectSchemaPolicy(value, childPointer, depth+1, nodes); policyError != nil {
			return policyError
		}
	}
	return nil
}

func escapePointerToken(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

func schemaPointer(pointer string) string {
	if pointer == "" {
		return "/"
	}
	return pointer
}

func invalidResult(pointer, code, message string) Result {
	return Result{IsValid: false, Errors: []Error{{Pointer: pointer, Code: code, Message: message}}}
}

func flattenValidationError(root *jsonschema.ValidationError) []Error {
	if len(root.Causes) == 0 {
		return []Error{toFieldError(root)}
	}
	errors := make([]Error, 0, len(root.Causes))
	for _, cause := range root.Causes {
		errors = append(errors, flattenValidationError(cause)...)
	}
	return errors
}

func toFieldError(err *jsonschema.ValidationError) Error {
	code := "validation_failed"
	if err.ErrorKind != nil {
		path := err.ErrorKind.KeywordPath()
		if len(path) > 0 {
			code = path[len(path)-1]
		}
	}
	parts := make([]string, len(err.InstanceLocation))
	for i, part := range err.InstanceLocation {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~", "~0"), "/", "~1")
	}
	pointer := "/"
	if len(parts) > 0 {
		pointer += strings.Join(parts, "/")
	}
	return Error{Pointer: pointer, Code: code, Message: err.Error()}
}
