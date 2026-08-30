package authn

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/goformx/goforms/internal/domain/auth"
)

const maxJWKSBytes = 1 << 20

type JWKSProviderConfig struct {
	Snapshot        string
	URL             string
	RefreshInterval time.Duration
	HTTPClient      *http.Client
}

// JWKSProvider resolves Ed25519 verification keys from a deployable snapshot and one pinned URL.
type JWKSProvider struct {
	mu              sync.RWMutex
	keys            map[string]auth.VerificationKey
	url             string
	refreshInterval time.Duration
	lastRefresh     time.Time
	lastUnknown     time.Time
	refreshSequence uint64
	appliedSequence uint64
	client          *http.Client
}

func NewJWKSProvider(config JWKSProviderConfig) (*JWKSProvider, error) {
	keys, err := parseJWKS([]byte(config.Snapshot))
	if err != nil {
		return nil, fmt.Errorf("parse JWKS snapshot: %w", err)
	}
	if config.URL != "" {
		parsed, parseErr := url.Parse(config.URL)
		if parseErr != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
			return nil, errors.New("JWKS URL must be a pinned HTTPS URL")
		}
	}
	interval := config.RefreshInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	clientCopy := *client
	clientCopy.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &JWKSProvider{keys: keys, url: config.URL, refreshInterval: interval, lastRefresh: time.Now(), client: &clientCopy}, nil
}

func (p *JWKSProvider) FindKey(ctx context.Context, keyID string) (auth.VerificationKey, error) {
	if keyID == "" {
		return auth.VerificationKey{}, auth.ErrInvalidFirstPartyAssertion
	}
	if p.url != "" {
		p.refreshScheduled(ctx)
	}
	if key, ok := p.find(keyID); ok {
		return key, nil
	}
	if p.url != "" {
		p.refreshUnknown(ctx)
		if key, ok := p.find(keyID); ok {
			return key, nil
		}
	}
	return auth.VerificationKey{}, auth.ErrInvalidFirstPartyAssertion
}

func (p *JWKSProvider) find(keyID string) (auth.VerificationKey, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	key, ok := p.keys[keyID]
	if !ok {
		return auth.VerificationKey{}, false
	}
	key.PublicKey = append(ed25519.PublicKey(nil), key.PublicKey...)
	return key, true
}

func (p *JWKSProvider) refreshUnknown(ctx context.Context) {
	p.mu.Lock()
	if time.Since(p.lastUnknown) < p.refreshInterval {
		p.mu.Unlock()
		return
	}
	p.lastUnknown = time.Now()
	p.mu.Unlock()
	p.refresh(ctx, true)
}

func (p *JWKSProvider) refreshScheduled(ctx context.Context) {
	p.mu.RLock()
	due := time.Since(p.lastRefresh) >= p.refreshInterval
	p.mu.RUnlock()
	if due {
		p.refresh(ctx, false)
	}
}

func (p *JWKSProvider) refresh(ctx context.Context, force bool) {
	p.mu.Lock()
	if !force && time.Since(p.lastRefresh) < p.refreshInterval {
		p.mu.Unlock()
		return
	}
	p.lastRefresh = time.Now()
	if !force {
		p.lastUnknown = p.lastRefresh
	}
	p.refreshSequence++
	sequence := p.refreshSequence
	p.mu.Unlock()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return
	}
	request.Header.Set("Accept", "application/json")
	response, err := p.client.Do(request)
	if err != nil {
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return
	}
	document, err := io.ReadAll(io.LimitReader(response.Body, maxJWKSBytes+1))
	if err != nil || len(document) > maxJWKSBytes {
		return
	}
	keys, err := parseJWKS(document)
	if err != nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	// Negative trust information wins even if this response arrived late: key IDs
	// cannot legitimately be un-revoked or reused by a newer discovery document.
	for id, incoming := range keys {
		if incoming.State == auth.VerificationKeyRevoked {
			if previous, exists := p.keys[id]; !exists || previous.State != auth.VerificationKeyRevoked {
				p.keys[id] = incoming
			}
		}
	}
	if sequence <= p.appliedSequence {
		return // A later-started refresh has already supplied the accepted set.
	}
	// Explicit revocation is a tombstone for this provider's lifetime. Discovery
	// must not undo a deployed or previously observed revocation, even by omitting
	// the key temporarily and later reintroducing its ID with different material.
	for id, previous := range p.keys {
		if previous.State == auth.VerificationKeyRevoked {
			keys[id] = previous
		}
	}
	p.keys = keys
	p.appliedSequence = sequence
}

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	KeyType    string                    `json:"kty"`
	Curve      string                    `json:"crv"`
	Coordinate string                    `json:"x"`
	KeyID      string                    `json:"kid"`
	Use        string                    `json:"use,omitempty"`
	Algorithm  string                    `json:"alg"`
	State      auth.VerificationKeyState `json:"state"`
}

func parseJWKS(document []byte) (map[string]auth.VerificationKey, error) {
	if len(strings.TrimSpace(string(document))) == 0 {
		return nil, errors.New("JWKS snapshot is required")
	}
	decoder := json.NewDecoder(strings.NewReader(string(document)))
	decoder.DisallowUnknownFields()
	var set jwksDocument
	if err := decoder.Decode(&set); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("JWKS contains multiple JSON values")
	}
	if len(set.Keys) == 0 {
		return nil, errors.New("JWKS must contain at least one key")
	}
	keys := make(map[string]auth.VerificationKey, len(set.Keys))
	for _, item := range set.Keys {
		if item.KeyType != "OKP" || item.Curve != "Ed25519" || item.Algorithm != auth.FirstPartyAssertionAlgorithm ||
			item.KeyID == "" || (item.Use != "" && item.Use != "sig") || !validKeyState(item.State) {
			return nil, errors.New("JWKS contains an unsupported verification key")
		}
		decoded, err := base64.RawURLEncoding.DecodeString(item.Coordinate)
		if err != nil || len(decoded) != ed25519.PublicKeySize {
			return nil, errors.New("JWKS contains an invalid Ed25519 public key")
		}
		if _, duplicate := keys[item.KeyID]; duplicate {
			return nil, errors.New("JWKS contains a duplicate key ID")
		}
		keys[item.KeyID] = auth.VerificationKey{ID: item.KeyID, PublicKey: ed25519.PublicKey(decoded), State: item.State}
	}
	return keys, nil
}

func validKeyState(state auth.VerificationKeyState) bool {
	return state == auth.VerificationKeyNext || state == auth.VerificationKeyActive ||
		state == auth.VerificationKeyRetiring || state == auth.VerificationKeyRevoked
}
