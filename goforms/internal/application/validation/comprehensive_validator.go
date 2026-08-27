// Package validation validates canonical form definitions and submissions.
package validation

import (
	"errors"
	"fmt"
	"strings"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/goformx/goforms/internal/domain/form/model"
)

const (
	canonicalDialect = model.JSONSchemaDraft202012URI
	resourceURL      = "https://goformx.com/runtime/form-definition.json"
)

// ComprehensiveValidator wraps the maintained Draft 2020-12 implementation used by the core path.
type ComprehensiveValidator struct{}

// NewComprehensiveValidator creates a canonical JSON Schema validator.
func NewComprehensiveValidator() *ComprehensiveValidator { return &ComprehensiveValidator{} }

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
	return compiled, Result{IsValid: true, Errors: []Error{}}
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
