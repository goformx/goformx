package model

import (
	"errors"
	"fmt"
	"time"
)

// DefinitionValidator is the domain boundary for canonical schema compilation.
type DefinitionValidator interface {
	ValidateDefinition(schema JSON) error
}

// SchemaVersionState is the immutable version's publication state.
type SchemaVersionState string

const (
	SchemaVersionDraft     SchemaVersionState = "draft"
	SchemaVersionPublished SchemaVersionState = "published"
	SchemaVersionRetired   SchemaVersionState = "retired"
)

// SchemaVersion is an immutable form-definition snapshot.
type SchemaVersion struct {
	formID      string
	version     int
	schema      JSON
	state       SchemaVersionState
	createdAt   time.Time
	publishedAt *time.Time
}

// NewSchemaVersion validates and snapshots a canonical definition.
func NewSchemaVersion(formID string, version int, schema JSON, validator DefinitionValidator) (*SchemaVersion, error) {
	if formID == "" {
		return nil, errors.New("form ID is required")
	}
	if version < 1 {
		return nil, errors.New("schema version must be positive")
	}
	if validator == nil {
		return nil, errors.New("schema validator is required")
	}
	if err := validator.ValidateDefinition(schema); err != nil {
		return nil, fmt.Errorf("validate schema version: %w", err)
	}

	return &SchemaVersion{
		formID: formID, version: version, schema: cloneJSON(schema),
		state: SchemaVersionDraft, createdAt: time.Now().UTC(),
	}, nil
}

func (v *SchemaVersion) FormID() string            { return v.formID }
func (v *SchemaVersion) Version() int              { return v.version }
func (v *SchemaVersion) State() SchemaVersionState { return v.state }
func (v *SchemaVersion) CreatedAt() time.Time      { return v.createdAt }
func (v *SchemaVersion) PublishedAt() *time.Time   { return v.publishedAt }

// Schema returns a defensive copy so a version cannot be mutated after creation.
func (v *SchemaVersion) Schema() JSON { return cloneJSON(v.schema) }

// Publish returns a published copy; the draft remains unchanged.
func (v *SchemaVersion) Publish(now time.Time) (*SchemaVersion, error) {
	if v.state != SchemaVersionDraft {
		return nil, errors.New("only a draft schema version can be published")
	}
	published := *v
	published.state = SchemaVersionPublished
	timestamp := now.UTC()
	published.publishedAt = &timestamp
	published.schema = cloneJSON(v.schema)
	return &published, nil
}

func cloneJSON(value JSON) JSON {
	cloned := make(JSON, len(value))
	for key, item := range value {
		cloned[key] = cloneValue(item)
	}
	return cloned
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		copy := make(map[string]any, len(typed))
		for key, item := range typed {
			copy[key] = cloneValue(item)
		}
		return copy
	case JSON:
		return cloneJSON(typed)
	case []any:
		copy := make([]any, len(typed))
		for i, item := range typed {
			copy[i] = cloneValue(item)
		}
		return copy
	default:
		return value
	}
}
