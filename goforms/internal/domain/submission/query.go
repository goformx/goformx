// Package submission defines submission read and export policy independently of transport and storage.
package submission

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/goformx/goforms/internal/domain/form/model"
)

const (
	DefaultPageLimit = 25
	MaxPageLimit     = 100
	MaxCursorLength  = 1024
	MaxSchemaVersion = 2147483647
)

// ListOptions contains selectors only. Organization and form ownership are
// separate required repository arguments, never inferred from a cursor/filter.
type ListOptions struct {
	Limit          int
	Before         time.Time
	BeforeID       string
	ReceivedFrom   *time.Time
	ReceivedBefore *time.Time
	Status         model.SubmissionStatus
	SchemaVersion  int
}

func (o ListOptions) Validate() error {
	if o.Limit < 1 || o.Limit > MaxPageLimit {
		return errors.New("submission page limit must be between 1 and 100")
	}
	if o.Before.IsZero() != (o.BeforeID == "") {
		return errors.New("submission cursor requires both time and identifier")
	}
	if o.BeforeID != "" {
		if o.Before.UTC().Year() < 1 || o.Before.UTC().Year() > 9999 {
			return errors.New("submission cursor timestamp is invalid")
		}
		if _, err := uuid.Parse(o.BeforeID); err != nil {
			return errors.New("submission cursor identifier is invalid")
		}
	}
	if o.Status != "" && o.Status != model.SubmissionStatusAccepted {
		return errors.New("submission status filter must be accepted")
	}
	if o.SchemaVersion < 0 || o.SchemaVersion > MaxSchemaVersion {
		return errors.New("submission schema version filter is invalid")
	}
	for _, bound := range []*time.Time{o.ReceivedFrom, o.ReceivedBefore} {
		if bound != nil && (bound.UTC().Year() < 1 || bound.UTC().Year() > 9999 || bound.Nanosecond()%1000 != 0) {
			return errors.New("submission time filters require years 1 through 9999 and microsecond precision")
		}
	}
	if o.ReceivedFrom != nil && o.ReceivedBefore != nil && !o.ReceivedFrom.Before(*o.ReceivedBefore) {
		return errors.New("receivedFrom must be earlier than receivedBefore")
	}
	return nil
}
