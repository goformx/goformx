package model

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	// JSONSchemaDraft202012URI is the immutable canonical schema dialect for GoFormX v1.
	JSONSchemaDraft202012URI = "https://json-schema.org/draft/2020-12/schema"
	// PublicKeyPrefix distinguishes browser-safe form identifiers from internal UUIDs.
	PublicKeyPrefix = "gfpk_"
)

type LifecycleStatus string

const (
	LifecycleDraft     LifecycleStatus = "draft"
	LifecyclePublished LifecycleStatus = "published"
	LifecycleDisabled  LifecycleStatus = "disabled"
	LifecycleArchived  LifecycleStatus = "archived"
)

// IsValid reports whether the value is a supported persisted lifecycle state.
func (s LifecycleStatus) IsValid() bool {
	switch s {
	case LifecycleDraft, LifecyclePublished, LifecycleDisabled, LifecycleArchived:
		return true
	default:
		return false
	}
}

// NewPublicKey returns a browser-safe, unguessable identifier distinct from internal IDs.
func NewPublicKey() (string, error) { return newPublicKey(rand.Reader) }

func newPublicKey(source io.Reader) (string, error) {
	bytes := make([]byte, 24)
	if _, err := io.ReadFull(source, bytes); err != nil {
		return "", fmt.Errorf("generate public key: %w", err)
	}
	return PublicKeyPrefix + base64.RawURLEncoding.EncodeToString(bytes), nil
}

// Lifecycle enforces public visibility independently from persistence and HTTP.
type Lifecycle struct {
	status         LifecycleStatus
	publicKey      string
	currentVersion int
}

func NewLifecycle(publicKey string) (*Lifecycle, error) {
	if !strings.HasPrefix(publicKey, PublicKeyPrefix) || len(publicKey) < 25 {
		return nil, errors.New("invalid public form key")
	}
	return &Lifecycle{status: LifecycleDraft, publicKey: publicKey}, nil
}

func (l *Lifecycle) Status() LifecycleStatus    { return l.status }
func (l *Lifecycle) PublicKey() string          { return l.publicKey }
func (l *Lifecycle) CurrentVersion() int        { return l.currentVersion }
func (l *Lifecycle) CanAcceptSubmissions() bool { return l.status == LifecyclePublished }

func (l *Lifecycle) Publish(version int) error {
	if l.status == LifecycleArchived {
		return errors.New("archived forms cannot be published")
	}
	if version < 1 || version < l.currentVersion {
		return errors.New("published version cannot move backwards")
	}
	l.currentVersion = version
	l.status = LifecyclePublished
	return nil
}

func (l *Lifecycle) Disable() error {
	if l.status != LifecyclePublished {
		return errors.New("only a published form can be disabled")
	}
	l.status = LifecycleDisabled
	return nil
}

func (l *Lifecycle) Enable() error {
	if l.status != LifecycleDisabled || l.currentVersion < 1 {
		return errors.New("only a disabled published form can be enabled")
	}
	l.status = LifecyclePublished
	return nil
}

func (l *Lifecycle) Archive() { l.status = LifecycleArchived }

// ResolvePublicVersion selects only a known published version while the form is public.
func (l *Lifecycle) ResolvePublicVersion(requested int, published map[int]struct{}) (int, error) {
	if !l.CanAcceptSubmissions() {
		return 0, errors.New("form is not accepting public submissions")
	}
	version := requested
	if version == 0 {
		version = l.currentVersion
	}
	if _, ok := published[version]; !ok {
		return 0, errors.New("schema version is not published")
	}
	return version, nil
}
