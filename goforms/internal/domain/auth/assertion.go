package auth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	FirstPartyAssertionType      = "gofx-fpa+jwt"
	FirstPartyAssertionAlgorithm = "EdDSA"
	FirstPartyAssertionVersion   = 1
	FirstPartyAssertionMaxTTL    = 60 * time.Second
	FirstPartyAssertionMaxSkew   = 5 * time.Second
)

var (
	ErrInvalidFirstPartyAssertion = errors.New("invalid first-party assertion")
	ErrFirstPartyAssertionReplay  = errors.New("first-party assertion replayed")
	ErrFirstPartyAuthUnavailable  = errors.New("first-party authentication unavailable")
)

type VerificationKeyState string

const (
	VerificationKeyNext     VerificationKeyState = "next"
	VerificationKeyActive   VerificationKeyState = "active"
	VerificationKeyRetiring VerificationKeyState = "retiring"
	VerificationKeyRevoked  VerificationKeyState = "revoked"
)

type VerificationKey struct {
	ID        string
	PublicKey ed25519.PublicKey
	State     VerificationKeyState
}

// VerificationKeyProvider resolves only deployment-configured first-party keys.
type VerificationKeyProvider interface {
	FindKey(context.Context, string) (VerificationKey, error)
}

type AssertionReplay struct {
	Issuer         string
	AssertionID    string
	ExpiresAt      time.Time
	FirstSeenAt    time.Time
	SubjectID      string
	OrganizationID string
	KeyID          string
}

// AssertionReplayStore atomically consumes an issuer/assertion-ID pair.
type AssertionReplayStore interface {
	Consume(context.Context, AssertionReplay) error
}

type FirstPartyPrincipal struct {
	AssertionID    string
	SubjectID      string
	OrganizationID string
	RequestID      string
	KeyID          string
	Scopes         map[Scope]struct{}
}

type FirstPartyVerifier struct {
	issuer   string
	audience string
	keys     VerificationKeyProvider
	replays  AssertionReplayStore
}

func NewFirstPartyVerifier(
	issuer string,
	audience string,
	keys VerificationKeyProvider,
	replays AssertionReplayStore,
) (*FirstPartyVerifier, error) {
	if issuer == "" || audience == "" || keys == nil || replays == nil {
		return nil, errors.New("issuer, audience, key provider, and replay store are required")
	}
	return &FirstPartyVerifier{issuer: issuer, audience: audience, keys: keys, replays: replays}, nil
}

type assertionHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
	KeyID     string `json:"kid"`
}

type assertionClaims struct {
	Issuer         string  `json:"iss"`
	Audience       string  `json:"aud"`
	SubjectID      string  `json:"sub"`
	OrganizationID string  `json:"org"`
	Scopes         []Scope `json:"scp"`
	IssuedAt       int64   `json:"iat"`
	NotBefore      int64   `json:"nbf"`
	ExpiresAt      int64   `json:"exp"`
	AssertionID    string  `json:"jti"`
	RequestID      string  `json:"rid"`
	Version        int     `json:"ver"`
}

// IsFirstPartyAssertion classifies a credential by its protected type without authenticating it.
func IsFirstPartyAssertion(compact string) bool {
	segments := strings.Split(compact, ".")
	if len(segments) != 3 {
		return false
	}
	encoded, err := base64.RawURLEncoding.DecodeString(segments[0])
	if err != nil {
		return false
	}
	var header struct {
		Type string `json:"typ"`
	}
	return json.Unmarshal(encoded, &header) == nil && header.Type == FirstPartyAssertionType
}

// VerifyAndConsume validates one compact assertion and consumes its replay identity.
func (v *FirstPartyVerifier) VerifyAndConsume(
	ctx context.Context,
	compact string,
	now time.Time,
) (FirstPartyPrincipal, error) {
	segments := strings.Split(compact, ".")
	if len(segments) != 3 {
		return FirstPartyPrincipal{}, ErrInvalidFirstPartyAssertion
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(segments[0])
	if err != nil {
		return FirstPartyPrincipal{}, ErrInvalidFirstPartyAssertion
	}
	claimsBytes, err := base64.RawURLEncoding.DecodeString(segments[1])
	if err != nil {
		return FirstPartyPrincipal{}, ErrInvalidFirstPartyAssertion
	}
	signature, err := base64.RawURLEncoding.DecodeString(segments[2])
	if err != nil || len(signature) != ed25519.SignatureSize {
		return FirstPartyPrincipal{}, ErrInvalidFirstPartyAssertion
	}
	var header assertionHeader
	if strictJSON(headerBytes, &header) != nil || header.Algorithm != FirstPartyAssertionAlgorithm ||
		header.Type != FirstPartyAssertionType || header.KeyID == "" {
		return FirstPartyPrincipal{}, ErrInvalidFirstPartyAssertion
	}
	key, err := v.keys.FindKey(ctx, header.KeyID)
	if err != nil {
		if errors.Is(err, ErrFirstPartyAuthUnavailable) {
			return FirstPartyPrincipal{}, ErrFirstPartyAuthUnavailable
		}
		return FirstPartyPrincipal{}, ErrInvalidFirstPartyAssertion
	}
	if key.ID != header.KeyID || len(key.PublicKey) != ed25519.PublicKeySize || !key.State.acceptsAssertions() {
		return FirstPartyPrincipal{}, ErrInvalidFirstPartyAssertion
	}
	message := []byte(segments[0] + "." + segments[1])
	if !ed25519.Verify(key.PublicKey, message, signature) {
		return FirstPartyPrincipal{}, ErrInvalidFirstPartyAssertion
	}
	var claims assertionClaims
	if strictJSON(claimsBytes, &claims) != nil || v.validateClaims(claims, now.UTC()) != nil {
		return FirstPartyPrincipal{}, ErrInvalidFirstPartyAssertion
	}
	if err := v.replays.Consume(ctx, AssertionReplay{
		Issuer: claims.Issuer, AssertionID: claims.AssertionID,
		ExpiresAt: time.Unix(claims.ExpiresAt, 0).UTC().Add(FirstPartyAssertionMaxSkew), FirstSeenAt: now.UTC(),
		SubjectID: claims.SubjectID, OrganizationID: claims.OrganizationID, KeyID: header.KeyID,
	}); err != nil {
		if errors.Is(err, ErrFirstPartyAssertionReplay) {
			return FirstPartyPrincipal{}, ErrInvalidFirstPartyAssertion
		}
		return FirstPartyPrincipal{}, fmt.Errorf("%w: consume assertion replay identity", ErrFirstPartyAuthUnavailable)
	}
	scopes := make(map[Scope]struct{}, len(claims.Scopes))
	for _, scope := range claims.Scopes {
		scopes[scope] = struct{}{}
	}
	return FirstPartyPrincipal{AssertionID: claims.AssertionID, SubjectID: claims.SubjectID,
		OrganizationID: claims.OrganizationID, RequestID: claims.RequestID, KeyID: header.KeyID, Scopes: scopes}, nil
}

func (v *FirstPartyVerifier) validateClaims(claims assertionClaims, now time.Time) error {
	if claims.Issuer != v.issuer || claims.Audience != v.audience || claims.Version != FirstPartyAssertionVersion {
		return ErrInvalidFirstPartyAssertion
	}
	if !uuidValue(claims.SubjectID) || !uuidValue(claims.OrganizationID) ||
		!uuidV4(claims.AssertionID) || !uuidValue(claims.RequestID) {
		return ErrInvalidFirstPartyAssertion
	}
	if len(claims.Scopes) == 0 {
		return ErrInvalidFirstPartyAssertion
	}
	seen := make(map[Scope]struct{}, len(claims.Scopes))
	for _, scope := range claims.Scopes {
		if !scope.Valid() {
			return ErrInvalidFirstPartyAssertion
		}
		if _, duplicate := seen[scope]; duplicate {
			return ErrInvalidFirstPartyAssertion
		}
		seen[scope] = struct{}{}
	}
	if claims.IssuedAt < 0 || claims.NotBefore != claims.IssuedAt || claims.ExpiresAt <= claims.IssuedAt ||
		claims.ExpiresAt-claims.IssuedAt > int64(FirstPartyAssertionMaxTTL/time.Second) {
		return ErrInvalidFirstPartyAssertion
	}
	issuedAt := time.Unix(claims.IssuedAt, 0).UTC()
	notBefore := time.Unix(claims.NotBefore, 0).UTC()
	expiresAt := time.Unix(claims.ExpiresAt, 0).UTC()
	if issuedAt.After(now.Add(FirstPartyAssertionMaxSkew)) || notBefore.After(now.Add(FirstPartyAssertionMaxSkew)) ||
		now.After(expiresAt.Add(FirstPartyAssertionMaxSkew)) {
		return ErrInvalidFirstPartyAssertion
	}
	return nil
}

func (state VerificationKeyState) acceptsAssertions() bool {
	return state == VerificationKeyNext || state == VerificationKeyActive || state == VerificationKeyRetiring
}

func uuidV4(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.Version() == 4
}

func uuidValue(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func strictJSON(document []byte, target any) error {
	if err := rejectDuplicateJSONKeys(document); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values are not allowed")
	}
	return nil
}

func rejectDuplicateJSONKeys(document []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(document))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if err := walkJSONValue(decoder, token); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values are not allowed")
	}
	return nil
}

func walkJSONValue(decoder *json.Decoder, token json.Token) error {
	delimiter, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("JSON object key is invalid")
			}
			if _, duplicate := seen[key]; duplicate {
				return errors.New("duplicate JSON object key")
			}
			seen[key] = struct{}{}
			value, err := decoder.Token()
			if err != nil {
				return err
			}
			if err := walkJSONValue(decoder, value); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			value, err := decoder.Token()
			if err != nil {
				return err
			}
			if err := walkJSONValue(decoder, value); err != nil {
				return err
			}
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	closing, err := decoder.Token()
	if err != nil {
		return err
	}
	expected := json.Delim('}')
	if delimiter == '[' {
		expected = ']'
	}
	if closing != expected {
		return errors.New("mismatched JSON delimiter")
	}
	return nil
}
