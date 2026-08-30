// Package managementaudit defines secret-free, durable management change records.
package managementaudit

import (
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"

	"github.com/goformx/goforms/internal/domain/auth"
)

type Kind string

const (
	TokenCreated Kind = "service_token.created"
	TokenRevoked Kind = "service_token.revoked"
	TokenRotated Kind = "service_token.rotated"
)

var (
	ErrInvalid        = errors.New("invalid management audit event")
	ErrUnavailable    = errors.New("management audit unavailable")
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._:-]{0,127}$`)
)

// Event has no arbitrary metadata bag, token name, secret/hash or request body.
// RelatedID identifies the replacement token only for an atomic rotation.
type Event struct {
	ID         string
	Actor      auth.AuditActor
	Kind       Kind
	TargetID   string
	RelatedID  string
	Scopes     []auth.Scope
	ExpiresAt  *time.Time
	OccurredAt time.Time
}

func (e Event) Validate() error {
	if _, err := uuid.Parse(e.ID); err != nil || e.Actor.Validate() != nil ||
		!identifierPattern.MatchString(e.TargetID) || e.OccurredAt.IsZero() {
		return ErrInvalid
	}
	switch e.Kind {
	case TokenCreated, TokenRevoked:
		if e.RelatedID != "" {
			return ErrInvalid
		}
	case TokenRotated:
		if !identifierPattern.MatchString(e.RelatedID) || e.RelatedID == e.TargetID {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	if e.Kind == TokenRevoked {
		if len(e.Scopes) != 0 || e.ExpiresAt != nil {
			return ErrInvalid
		}
		return nil
	}
	if len(e.Scopes) == 0 || e.ExpiresAt == nil || e.ExpiresAt.IsZero() {
		return ErrInvalid
	}
	seen := make(map[auth.Scope]bool)
	for _, scope := range e.Scopes {
		if !scope.Valid() || seen[scope] {
			return ErrInvalid
		}
		seen[scope] = true
	}
	return nil
}
