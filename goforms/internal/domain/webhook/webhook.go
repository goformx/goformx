// Package webhook defines the durable webhook delivery contract.
package webhook

import (
	"errors"
	"time"
	"unicode/utf8"
)

var (
	ErrDisabled      = errors.New("webhook delivery is not configured")
	ErrNotFound      = errors.New("webhook resource not found")
	ErrInvalidChange = errors.New("exactly one valid webhook lifecycle change is required")
)

// EndpointChange never requires reading a stored secret back through the API.
type EndpointChange struct {
	Enabled       *bool   `json:"enabled"`
	SigningSecret *string `json:"signingSecret"`
}

func (c EndpointChange) Validate() error {
	if (c.Enabled == nil) == (c.SigningSecret == nil) {
		return ErrInvalidChange
	}
	if c.SigningSecret != nil && !ValidSigningSecret(*c.SigningSecret) {
		return ErrInvalidChange
	}
	return nil
}

// ValidSigningSecret uses Unicode character counts, as required by the OpenAPI
// minLength/maxLength contract. It never estimates the secret's entropy.
func ValidSigningSecret(secret string) bool {
	length := utf8.RuneCountInString(secret)
	return utf8.ValidString(secret) && length >= 32 && length <= 256
}

type DeliveryStatus string

const (
	DeliveryPending    DeliveryStatus = "pending"
	DeliveryProcessing DeliveryStatus = "processing"
	DeliveryDelivered  DeliveryStatus = "delivered"
	DeliveryDeadLetter DeliveryStatus = "dead_letter"
)

// SecretConfig is encrypted as one authenticated value and is never returned by an API.
type SecretConfig struct {
	DestinationURL string            `json:"destinationUrl"`
	Headers        map[string]string `json:"headers"`
	SigningSecret  string            `json:"signingSecret"`
}

type Endpoint struct {
	ID              string
	FormID          string
	Origin          string
	EncryptedConfig []byte
	Enabled         bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Delivery struct {
	ID                string
	SubmissionID      string
	FormID            string
	EndpointID        string
	DestinationOrigin string
	EncryptedConfig   []byte
	Status            DeliveryStatus
	AttemptCount      int
	NextAttemptAt     time.Time
	LockedAt          *time.Time
	DeliveredAt       *time.Time
	LastHTTPStatus    *int
	LastErrorCategory string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Event struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	CreatedAt     time.Time      `json:"createdAt"`
	SubmissionID  string         `json:"submissionId"`
	FormID        string         `json:"formId"`
	SchemaVersion int            `json:"schemaVersion"`
	Data          map[string]any `json:"data"`
}
