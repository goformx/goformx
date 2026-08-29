package auth_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/domain/auth"
)

const (
	testIssuer   = "https://goformx.com"
	testAudience = "https://api.goformx.com"
	testKeyID    = "gofx-fpa-test-a"
)

type assertionKeyProvider struct {
	key auth.VerificationKey
	err error
}

func (p assertionKeyProvider) FindKey(context.Context, string) (auth.VerificationKey, error) {
	return p.key, p.err
}

type assertionReplayStore struct {
	consumed map[string]auth.AssertionReplay
	err      error
}

func (s *assertionReplayStore) Consume(_ context.Context, replay auth.AssertionReplay) error {
	if s.err != nil {
		return s.err
	}
	if _, exists := s.consumed[replay.Issuer+replay.AssertionID]; exists {
		return auth.ErrFirstPartyAssertionReplay
	}
	s.consumed[replay.Issuer+replay.AssertionID] = replay
	return nil
}

func TestFirstPartyVerifierAcceptsCanonicalAssertionOnce(t *testing.T) {
	t.Parallel()
	now := time.Unix(1788033600, 0).UTC()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	replays := &assertionReplayStore{consumed: map[string]auth.AssertionReplay{}}
	verifier, err := auth.NewFirstPartyVerifier(testIssuer, testAudience,
		assertionKeyProvider{key: auth.VerificationKey{ID: testKeyID, PublicKey: publicKey, State: auth.VerificationKeyActive}},
		replays,
	)
	require.NoError(t, err)
	compact := signAssertion(t, privateKey, canonicalHeader(), canonicalClaims(now))
	require.True(t, auth.IsFirstPartyAssertion(compact))

	principal, err := verifier.VerifyAndConsume(t.Context(), compact, now)
	require.NoError(t, err)
	require.Equal(t, "22222222-2222-4222-8222-222222222222", principal.OrganizationID)
	require.Equal(t, "11111111-1111-4111-8111-111111111111", principal.SubjectID)
	require.Equal(t, "44444444-4444-4444-8444-444444444444", principal.RequestID)
	require.Contains(t, principal.Scopes, auth.ScopeFormsRead)
	require.Contains(t, principal.Scopes, auth.ScopeFormsWrite)
	require.Equal(t, now.Add(65*time.Second), replays.consumed[testIssuer+principal.AssertionID].ExpiresAt)

	_, err = verifier.VerifyAndConsume(t.Context(), compact, now)
	require.ErrorIs(t, err, auth.ErrInvalidFirstPartyAssertion)
}

func TestFirstPartyVerifierAcceptsStableNonV4IdentityAndCorrelationUUIDs(t *testing.T) {
	t.Parallel()
	now := time.Unix(1788033600, 0).UTC()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	verifier, err := auth.NewFirstPartyVerifier(testIssuer, testAudience,
		assertionKeyProvider{key: auth.VerificationKey{ID: testKeyID, PublicKey: publicKey, State: auth.VerificationKeyActive}},
		&assertionReplayStore{consumed: map[string]auth.AssertionReplay{}},
	)
	require.NoError(t, err)
	claims := canonicalClaims(now)
	claims["sub"] = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
	claims["org"] = "6ba7b811-9dad-11d1-80b4-00c04fd430c8"
	claims["rid"] = "6ba7b812-9dad-11d1-80b4-00c04fd430c8"

	principal, err := verifier.VerifyAndConsume(t.Context(), signAssertion(t, privateKey, canonicalHeader(), claims), now)
	require.NoError(t, err)
	require.Equal(t, claims["sub"], principal.SubjectID)
	require.Equal(t, claims["org"], principal.OrganizationID)
	require.Equal(t, claims["rid"], principal.RequestID)
}

func TestFirstPartyVerifierAcceptsEveryRotationOverlapState(t *testing.T) {
	t.Parallel()
	now := time.Unix(1788033600, 0).UTC()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	compact := signAssertion(t, privateKey, canonicalHeader(), canonicalClaims(now))

	for _, state := range []auth.VerificationKeyState{
		auth.VerificationKeyNext,
		auth.VerificationKeyActive,
		auth.VerificationKeyRetiring,
	} {
		t.Run(string(state), func(t *testing.T) {
			t.Parallel()
			verifier, createErr := auth.NewFirstPartyVerifier(testIssuer, testAudience,
				assertionKeyProvider{key: auth.VerificationKey{ID: testKeyID, PublicKey: publicKey, State: state}},
				&assertionReplayStore{consumed: map[string]auth.AssertionReplay{}},
			)
			require.NoError(t, createErr)
			_, verifyErr := verifier.VerifyAndConsume(t.Context(), compact, now)
			require.NoError(t, verifyErr)
		})
	}
}

func TestFirstPartyVerifierRejectsInvalidProfiles(t *testing.T) {
	t.Parallel()
	now := time.Unix(1788033600, 0).UTC()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	tests := map[string]struct {
		header map[string]any
		claims map[string]any
		at     time.Time
		state  auth.VerificationKeyState
		mutate func(string) string
	}{
		"wrong algorithm":          {header: merge(canonicalHeader(), "alg", "HS256")},
		"wrong type":               {header: merge(canonicalHeader(), "typ", "JWT")},
		"embedded key URL":         {header: merge(canonicalHeader(), "jku", "https://attacker.example/jwks")},
		"wrong issuer":             {claims: merge(canonicalClaims(now), "iss", "https://attacker.example")},
		"wrong audience":           {claims: merge(canonicalClaims(now), "aud", "https://other.example")},
		"audience array":           {claims: merge(canonicalClaims(now), "aud", []string{testAudience})},
		"unknown claim":            {claims: merge(canonicalClaims(now), "email", "person@example.test")},
		"duplicate scope":          {claims: merge(canonicalClaims(now), "scp", []string{"forms:read", "forms:read"})},
		"unknown scope":            {claims: merge(canonicalClaims(now), "scp", []string{"admin"})},
		"non-v4 assertion ID":      {claims: merge(canonicalClaims(now), "jti", "33333333-3333-3333-8333-333333333333")},
		"not-before mismatch":      {claims: merge(canonicalClaims(now), "nbf", now.Add(time.Second).Unix())},
		"lifetime too long":        {claims: merge(canonicalClaims(now), "exp", now.Add(61*time.Second).Unix())},
		"issued too far in future": {claims: canonicalClaims(now.Add(6 * time.Second)), at: now},
		"expired beyond skew":      {claims: canonicalClaims(now), at: now.Add(66 * time.Second)},
		"revoked key":              {state: auth.VerificationKeyRevoked},
		"corrupt signature": {mutate: func(compact string) string {
			parts := strings.Split(compact, ".")
			parts[2] = base64.RawURLEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
			return strings.Join(parts, ".")
		}},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			header := test.header
			if header == nil {
				header = canonicalHeader()
			}
			claims := test.claims
			if claims == nil {
				claims = canonicalClaims(now)
			}
			state := test.state
			if state == "" {
				state = auth.VerificationKeyActive
			}
			verifier, createErr := auth.NewFirstPartyVerifier(testIssuer, testAudience,
				assertionKeyProvider{key: auth.VerificationKey{ID: testKeyID, PublicKey: publicKey, State: state}},
				&assertionReplayStore{consumed: map[string]auth.AssertionReplay{}},
			)
			require.NoError(t, createErr)
			compact := signAssertion(t, privateKey, header, claims)
			if test.mutate != nil {
				compact = test.mutate(compact)
			}
			verificationTime := test.at
			if verificationTime.IsZero() {
				verificationTime = now
			}
			_, verifyErr := verifier.VerifyAndConsume(t.Context(), compact, verificationTime)
			require.ErrorIs(t, verifyErr, auth.ErrInvalidFirstPartyAssertion)
		})
	}
}

func TestFirstPartyVerifierDistinguishesInfrastructureFailure(t *testing.T) {
	t.Parallel()
	now := time.Unix(1788033600, 0).UTC()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	compact := signAssertion(t, privateKey, canonicalHeader(), canonicalClaims(now))

	verifier, err := auth.NewFirstPartyVerifier(testIssuer, testAudience,
		assertionKeyProvider{key: auth.VerificationKey{ID: testKeyID, PublicKey: publicKey, State: auth.VerificationKeyActive}},
		&assertionReplayStore{consumed: map[string]auth.AssertionReplay{}, err: errors.New("database unavailable")},
	)
	require.NoError(t, err)
	_, err = verifier.VerifyAndConsume(t.Context(), compact, now)
	require.ErrorIs(t, err, auth.ErrFirstPartyAuthUnavailable)
}

func canonicalHeader() map[string]any {
	return map[string]any{"alg": auth.FirstPartyAssertionAlgorithm, "typ": auth.FirstPartyAssertionType, "kid": testKeyID}
}

func canonicalClaims(now time.Time) map[string]any {
	return map[string]any{
		"iss": testIssuer, "aud": testAudience,
		"sub": "11111111-1111-4111-8111-111111111111", "org": "22222222-2222-4222-8222-222222222222",
		"scp": []string{"forms:read", "forms:write"}, "iat": now.Unix(), "nbf": now.Unix(),
		"exp": now.Add(time.Minute).Unix(), "jti": "33333333-3333-4333-8333-333333333333",
		"rid": "44444444-4444-4444-8444-444444444444", "ver": 1,
	}
}

func merge(source map[string]any, key string, value any) map[string]any {
	copy := make(map[string]any, len(source)+1)
	for name, item := range source {
		copy[name] = item
	}
	copy[key] = value
	return copy
}

func signAssertion(t *testing.T, privateKey ed25519.PrivateKey, header, claims map[string]any) string {
	t.Helper()
	headerJSON, err := json.Marshal(header)
	require.NoError(t, err)
	claimsJSON, err := json.Marshal(claims)
	require.NoError(t, err)
	message := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	signature := ed25519.Sign(privateKey, []byte(message))
	return message + "." + base64.RawURLEncoding.EncodeToString(signature)
}
