// Package model contains domain models and error definitions for forms.
package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"database/sql/driver"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	// MinTitleLength is the minimum length for a form title
	MinTitleLength = 3
	// MaxTitleLength is the maximum length for a form title
	MaxTitleLength = 100
	// MaxDescriptionLength is the maximum length for a form description
	MaxDescriptionLength = 500
)

var (
	// ErrInvalidJSON represents an invalid JSON error
	ErrInvalidJSON = errors.New("invalid JSON")
)

// Form represents a form in the system
type Form struct {
	ID                   string          `gorm:"column:uuid;primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	OrganizationID       string          `gorm:"column:organization_id;not null;index;type:uuid"            json:"organization_id"`
	Name                 string          `gorm:"not null;size:63"                                           json:"name"`
	Title                string          `gorm:"not null;size:100"                                          json:"title"`
	Description          string          `gorm:"size:500"                                                   json:"description"`
	Schema               JSON            `gorm:"-"                                                          json:"schema"`
	Active               bool            `gorm:"not null;default:true"                                      json:"active"`
	CreatedAt            time.Time       `gorm:"not null;autoCreateTime"                                    json:"created_at"`
	UpdatedAt            time.Time       `gorm:"not null;autoUpdateTime"                                    json:"updated_at"`
	DeletedAt            gorm.DeletedAt  `gorm:"index"                                                      json:"-"`
	Status               LifecycleStatus `gorm:"size:20;not null;default:'draft'"                          json:"status"`
	PublicKey            string          `gorm:"not null;uniqueIndex"                                       json:"public_key"`
	CurrentSchemaVersion int             `gorm:"not null;default:1"                                         json:"current_schema_version"`

	// CORS settings for form embedding
	CorsOrigins JSON `gorm:"type:json" json:"cors_origins"`
	CorsMethods JSON `gorm:"type:json" json:"cors_methods"`
	CorsHeaders JSON `gorm:"type:json" json:"cors_headers"`
}

// GetID returns the form's ID
func (f *Form) GetID() string {
	return f.ID
}

// SetID sets the form's ID
func (f *Form) SetID(id string) {
	f.ID = id
}

// TableName specifies the table name for the Form model
func (f *Form) TableName() string {
	return "forms"
}

// BeforeCreate is a GORM hook that runs before creating a form
func (f *Form) BeforeCreate(_ *gorm.DB) error {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}

	if !f.Active {
		f.Active = true
	}

	if f.Status == "" {
		f.Status = LifecycleDraft
	}
	if f.Name == "" {
		f.Name = slugifyName(f.Title)
	}
	if f.PublicKey == "" {
		publicKey, err := NewPublicKey()
		if err != nil {
			return err
		}
		f.PublicKey = publicKey
	}
	if f.CurrentSchemaVersion == 0 {
		f.CurrentSchemaVersion = 1
	}

	// Ensure CORS fields are properly initialized
	if f.CorsOrigins == nil {
		f.CorsOrigins = JSON{}
	}

	if f.CorsMethods == nil {
		f.CorsMethods = JSON{}
	}

	if f.CorsHeaders == nil {
		f.CorsHeaders = JSON{}
	}

	return nil
}

func slugifyName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') {
			builder.WriteRune(character)
			lastDash = false
		} else if builder.Len() > 0 && !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	name := strings.Trim(builder.String(), "-")
	if len(name) > 63 {
		name = strings.TrimRight(name[:63], "-")
	}
	if len(name) < 3 {
		name = "form-" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	}
	return name
}

// BeforeUpdate is a GORM hook that runs before updating a form
func (f *Form) BeforeUpdate(_ *gorm.DB) error {
	f.UpdatedAt = time.Now()

	return nil
}

// BeforeSave is a GORM hook that runs before saving a form
func (f *Form) BeforeSave(_ *gorm.DB) error {
	// Ensure CORS fields are properly initialized
	if f.CorsOrigins == nil {
		f.CorsOrigins = JSON{}
	}

	if f.CorsMethods == nil {
		f.CorsMethods = JSON{}
	}

	if f.CorsHeaders == nil {
		f.CorsHeaders = JSON{}
	}

	return nil
}

// JSON is a custom type for handling JSON data
type JSON map[string]any

// Scan implements the sql.Scanner interface for JSON
func (j *JSON) Scan(value any) error {
	if value == nil {
		*j = nil

		return nil
	}

	encoded, ok := value.([]byte)
	if !ok {
		return errors.New("JSON scan value must be bytes")
	}

	// First try to unmarshal as an object
	var result map[string]any

	err := decodeExactJSON(encoded, &result)
	if err == nil {
		*j = JSON(result)

		return nil
	}

	// If that fails, try to unmarshal as an array and convert to object
	var arrayResult []any

	err = decodeExactJSON(encoded, &arrayResult)
	if err != nil {
		return fmt.Errorf("unmarshal JSON scan value: %w", err)
	}

	// Convert array to object with "data" key
	*j = JSON{"data": arrayResult}

	return nil
}

// Value implements the driver.Valuer interface for JSON
func (j *JSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, ErrInvalidJSON
	}

	data, err := json.Marshal(*j)
	if err != nil {
		return nil, fmt.Errorf("marshal JSON value: %w", err)
	}

	return data, nil
}

// MarshalJSON implements the json.Marshaler interface
func (j *JSON) MarshalJSON() ([]byte, error) {
	if j == nil {
		return nil, ErrInvalidJSON
	}

	data, err := json.Marshal(*j)
	if err != nil {
		return nil, fmt.Errorf("marshal JSON to bytes: %w", err)
	}

	return data, nil
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (j *JSON) UnmarshalJSON(data []byte) error {
	if j == nil {
		return ErrInvalidJSON
	}

	var result map[string]any
	if err := decodeExactJSON(data, &result); err != nil {
		return fmt.Errorf("unmarshal JSON from bytes: %w", err)
	}
	*j = JSON(result)

	return nil
}

// A JSON number is not necessarily representable by float64. Keep the original
// numeric token through validation and database round trips. PostgreSQL JSONB
// schema/metadata values may normalize notation; immutable submission JSON keeps
// the accepted token spelling. Both storage types must preserve the numeric value.
func decodeExactJSON(data []byte, destination any) error {
	if err := validateJSONNumbers(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("JSON must contain exactly one value")
	}
	return nil
}

// Bound arbitrary-precision work before a schema compiler or PostgreSQL can
// expand a short exponent into an enormous value. These are lexical budgets,
// not rounding rules: rejected input is never silently changed to zero/infinity.
const (
	MaxJSONNumberBytes    = 4096
	MaxJSONExponent       = 1024
	MaxJSONIntegerDigits  = 1024
	MaxJSONFractionDigits = 1024
)

func validateJSONNumbers(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		number, ok := token.(json.Number)
		if !ok {
			continue
		}
		if len(number) > MaxJSONNumberBytes {
			return fmt.Errorf("JSON numeric tokens must not exceed %d bytes", MaxJSONNumberBytes)
		}
		mantissa := strings.TrimPrefix(string(number), "-")
		var exponent int64
		if index := strings.IndexAny(mantissa, "eE"); index >= 0 {
			var err error
			exponent, err = strconv.ParseInt(mantissa[index+1:], 10, 32)
			if err != nil || exponent < -MaxJSONExponent || exponent > MaxJSONExponent {
				return fmt.Errorf("JSON numeric exponents must be between -%d and %d", MaxJSONExponent, MaxJSONExponent)
			}
			mantissa = mantissa[:index]
		}
		whole, fraction, _ := strings.Cut(mantissa, ".")
		digits := whole + fraction
		point := int64(len(whole)) + exponent
		// PostgreSQL's JSON types preserve the numeric value and JSONB preserves
		// fractional scale. Count those places so every stored value remains in
		// budget regardless of the column's intentional JSON/JSONB choice.
		if int64(len(digits))-point > MaxJSONFractionDigits {
			return fmt.Errorf("JSON numeric values must fit %d fractional decimal places", MaxJSONFractionDigits)
		}
		nonzero := strings.TrimLeft(digits, "0")
		if nonzero == "" {
			continue
		}
		first := len(digits) - len(nonzero)
		// Value-based limits stay valid when JSONB expands exponent notation.
		if point-int64(first) > MaxJSONIntegerDigits {
			return fmt.Errorf("JSON numeric values must fit %d integer decimal places", MaxJSONIntegerDigits)
		}
	}
}

// NewForm creates a new form instance
func NewForm(organizationID, title, description string, schema JSON) *Form {
	now := time.Now()

	return &Form{
		ID:             uuid.New().String(),
		OrganizationID: organizationID,
		Title:          title,
		Description:    description,
		Schema:         schema,
		Active:         true,
		Status:         LifecycleDraft,
		CreatedAt:      now,
		UpdatedAt:      now,
		DeletedAt:      gorm.DeletedAt{},
		CorsOrigins:    JSON{},
		CorsMethods:    JSON{},
		CorsHeaders:    JSON{},
	}
}

// Validate checks metadata and delegates schema policy to the canonical authority.
func (f *Form) Validate(validator DefinitionValidator) error {
	if validator == nil {
		return errors.New("schema validator is required")
	}
	if f.Title == "" {
		return errors.New("title is required")
	}

	if len(f.Title) < MinTitleLength {
		return fmt.Errorf("title must be between %d and %d characters", MinTitleLength, MaxTitleLength)
	}

	if len(f.Title) > MaxTitleLength {
		return fmt.Errorf("title must be between %d and %d characters", MinTitleLength, MaxTitleLength)
	}

	if len(f.Description) > MaxDescriptionLength {
		return fmt.Errorf("description must not exceed %d characters", MaxDescriptionLength)
	}

	return validator.ValidateDefinition(f.Schema)
}

// Update updates the form with new values
func (f *Form) Update(title, description string, schema JSON) {
	f.Title = title
	f.Description = description

	if schema != nil {
		f.Schema = schema
	}

	f.UpdatedAt = time.Now()
}

// Deactivate marks the form as inactive
func (f *Form) Deactivate() {
	f.Active = false
	f.UpdatedAt = time.Now()
}

// Activate marks the form as active
func (f *Form) Activate() {
	f.Active = true
	f.UpdatedAt = time.Now()
}

// extractStringSlice extracts a string slice from JSON array
func extractStringSlice(data JSON, key string) []string {
	var result []string
	if data == nil {
		return result
	}

	// First try to get the value directly by key
	if arr, ok := data[key].([]any); ok {
		for _, item := range arr {
			if str, strOk := item.(string); strOk {
				result = append(result, str)
			}
		}

		return result
	}
	if arr, ok := data[key].([]string); ok {
		return append(result, arr...)
	}

	// If not found by key, check if the data itself is an array (stored under "data" key)
	if arr, ok := data["data"].([]any); ok {
		for _, item := range arr {
			if str, strOk := item.(string); strOk {
				result = append(result, str)
			}
		}

		return result
	}
	if arr, ok := data["data"].([]string); ok {
		return append(result, arr...)
	}

	return result
}

// GetCorsConfig returns the CORS configuration for this form
func (f *Form) GetCorsConfig() (origins, methods, headers []string) {
	origins = extractStringSlice(f.CorsOrigins, "origins")
	methods = extractStringSlice(f.CorsMethods, "methods")
	headers = extractStringSlice(f.CorsHeaders, "headers")

	return origins, methods, headers
}

// SetCorsConfig sets the CORS configuration for this form
func (f *Form) SetCorsConfig(origins, methods, headers []string) {
	f.CorsOrigins = JSON{"origins": origins}
	f.CorsMethods = JSON{"methods": methods}
	f.CorsHeaders = JSON{"headers": headers}
}
