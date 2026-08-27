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

const (
	ScopeFormsRead       Scope = "forms:read"
	ScopeFormsWrite      Scope = "forms:write"
	ScopeFormsPublish    Scope = "forms:publish"
	ScopeSubmissionsRead Scope = "submissions:read"
)

// ServiceToken is the persisted token metadata. Only Hash is stored; Plaintext is returned once.
type ServiceToken struct {
	ID        string
	OwnerID   string
	Hash      [sha256.Size]byte
	Scopes    map[Scope]struct{}
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
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
	plaintext := "gfst_" + base64.RawURLEncoding.EncodeToString(random)
	hash := sha256.Sum256([]byte(plaintext))
	id := base64.RawURLEncoding.EncodeToString(hash[:12])
	scopeSet := make(map[Scope]struct{}, len(scopes))
	for _, scope := range scopes {
		scopeSet[scope] = struct{}{}
	}

	return &ServiceToken{ID: id, OwnerID: ownerID, Hash: hash, Scopes: scopeSet,
		CreatedAt: now.UTC(), ExpiresAt: now.Add(ttl).UTC()}, plaintext, nil
}

func (t *ServiceToken) Authorize(plaintext, ownerID string, required Scope, now time.Time) error {
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
	if _, ok := t.Scopes[required]; !ok {
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
