package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

type Scope string

// ServiceTokenPrefix distinguishes server-side credentials from other identifiers.
const ServiceTokenPrefix = "gfst_"

const (
	ScopeFormsRead       Scope = "forms:read"
	ScopeFormsWrite      Scope = "forms:write"
	ScopeFormsPublish    Scope = "forms:publish"
	ScopeSubmissionsRead Scope = "submissions:read"
	ScopeTokensRead      Scope = "tokens:read"
	ScopeTokensWrite     Scope = "tokens:write"
	ScopeWebhooksRead    Scope = "webhooks:read"
	ScopeWebhooksWrite   Scope = "webhooks:write"
)

var canonicalScopes = [...]Scope{
	ScopeFormsRead,
	ScopeFormsWrite,
	ScopeFormsPublish,
	ScopeSubmissionsRead,
	ScopeTokensRead,
	ScopeTokensWrite,
	ScopeWebhooksRead,
	ScopeWebhooksWrite,
}

// AllScopes returns a copy of the versioned management scope registry.
func AllScopes() []Scope {
	return append([]Scope(nil), canonicalScopes[:]...)
}

// ScopeCount is the runtime size of the versioned management scope registry.
// Persisted schema bounds must change through an explicit migration when this
// value changes.
func ScopeCount() int {
	return len(canonicalScopes)
}

// Valid reports whether the scope belongs to the versioned management contract.
func (s Scope) Valid() bool {
	for _, candidate := range canonicalScopes {
		if s == candidate {
			return true
		}
	}
	return false
}

// ServiceToken is the persisted token metadata. Only Hash is stored; Plaintext is returned once.
type ServiceToken struct {
	ID                string
	Name              string
	OwnerID           string
	Hash              [sha256.Size]byte
	Scopes            map[Scope]struct{}
	CreatedAt         time.Time
	ExpiresAt         time.Time
	RevokedAt         *time.Time
	LastUsedAt        *time.Time
	ReplacedByTokenID string
	RevocationReason  string
}

// HasScope reports whether the token was granted one exact canonical scope.
func (t *ServiceToken) HasScope(scope Scope) bool {
	_, ok := t.Scopes[scope]
	return ok
}

func Issue(ownerID string, scopes []Scope, ttl time.Duration, now time.Time) (*ServiceToken, string, error) {
	if ownerID == "" {
		return nil, "", errors.New("token owner is required")
	}
	if len(scopes) == 0 {
		return nil, "", errors.New("at least one scope is required")
	}
	if ttl <= 0 {
		return nil, "", errors.New("token TTL must be positive")
	}

	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}
	plaintext := ServiceTokenPrefix + base64.RawURLEncoding.EncodeToString(random)
	hash := sha256.Sum256([]byte(plaintext))
	id := base64.RawURLEncoding.EncodeToString(hash[:12])
	scopeSet := make(map[Scope]struct{}, len(scopes))
	for _, scope := range scopes {
		if !scope.Valid() {
			return nil, "", fmt.Errorf("unsupported scope %q", scope)
		}
		scopeSet[scope] = struct{}{}
	}

	return &ServiceToken{ID: id, Name: "Service token", OwnerID: ownerID, Hash: hash, Scopes: scopeSet,
		CreatedAt: now.UTC(), ExpiresAt: now.Add(ttl).UTC()}, plaintext, nil
}

// Authenticate verifies the opaque token value, organization binding, and lifecycle.
func (t *ServiceToken) Authenticate(plaintext, ownerID string, now time.Time) error {
	if t.RevokedAt != nil {
		return errors.New("service token is revoked")
	}
	if !now.Before(t.ExpiresAt) {
		return errors.New("service token is expired")
	}
	if ownerID == "" || ownerID != t.OwnerID {
		return errors.New("service token owner mismatch")
	}
	candidate := sha256.Sum256([]byte(plaintext))
	if subtle.ConstantTimeCompare(candidate[:], t.Hash[:]) != 1 {
		return errors.New("invalid service token")
	}
	return nil
}

func (t *ServiceToken) Authorize(plaintext, ownerID string, required Scope, now time.Time) error {
	if err := t.Authenticate(plaintext, ownerID, now); err != nil {
		return err
	}
	if !t.HasScope(required) {
		return errors.New("service token scope denied")
	}
	return nil
}

func (t *ServiceToken) Revoke(now time.Time) {
	timestamp := now.UTC()
	t.RevokedAt = &timestamp
}

// LookupID derives the non-secret lookup identifier from a presented token.
func LookupID(plaintext string) string {
	hash := sha256.Sum256([]byte(plaintext))
	return base64.RawURLEncoding.EncodeToString(hash[:12])
}
