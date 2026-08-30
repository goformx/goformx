package authn_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/goformx/goforms/internal/domain/auth"
	"github.com/goformx/goforms/internal/infrastructure/authn"
)

func TestJWKSProviderSupportsRotationStatesAndPinnedRefresh(t *testing.T) {
	t.Parallel()
	active, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	next, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		response.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(response, jwks("next", next, auth.VerificationKeyNext))
	}))
	defer server.Close()

	provider, err := authn.NewJWKSProvider(authn.JWKSProviderConfig{
		Snapshot: jwks("active", active, auth.VerificationKeyActive), URL: server.URL,
		HTTPClient: server.Client(), RefreshInterval: time.Minute,
	})
	require.NoError(t, err)
	key, err := provider.FindKey(t.Context(), "active")
	require.NoError(t, err)
	require.Equal(t, auth.VerificationKeyActive, key.State)
	require.Zero(t, requests.Load())
	key, err = provider.FindKey(t.Context(), "next")
	require.NoError(t, err)
	require.Equal(t, auth.VerificationKeyNext, key.State)
	require.EqualValues(t, 1, requests.Load())
	_, err = provider.FindKey(t.Context(), "attacker-controlled-unknown")
	require.ErrorIs(t, err, auth.ErrInvalidFirstPartyAssertion)
	require.EqualValues(t, 1, requests.Load(), "unknown kids must not bypass the refresh cooldown")
}

func TestJWKSProviderRejectsUnsafeSnapshots(t *testing.T) {
	t.Parallel()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	tests := map[string]string{
		"empty":               "",
		"symmetric":           `{"keys":[{"kty":"oct","crv":"Ed25519","x":"AA","kid":"bad","alg":"EdDSA","state":"active"}]}`,
		"unknown field":       `{"keys":[{"kty":"OKP","crv":"Ed25519","x":"AA","kid":"bad","alg":"EdDSA","state":"active","jku":"https://attacker.example"}]}`,
		"revoked is retained": jwks("revoked", publicKey, auth.VerificationKeyRevoked),
	}
	for name, snapshot := range tests {
		t.Run(name, func(t *testing.T) {
			provider, createErr := authn.NewJWKSProvider(authn.JWKSProviderConfig{Snapshot: snapshot})
			if name == "revoked is retained" {
				require.NoError(t, createErr)
				key, findErr := provider.FindKey(t.Context(), "revoked")
				require.NoError(t, findErr)
				require.Equal(t, auth.VerificationKeyRevoked, key.State)
				return
			}
			require.Error(t, createErr)
		})
	}
}

func TestJWKSProviderRequiresPinnedHTTPSURL(t *testing.T) {
	t.Parallel()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	_, err = authn.NewJWKSProvider(authn.JWKSProviderConfig{
		Snapshot: jwks("active", publicKey, auth.VerificationKeyActive), URL: "http://goformx.com/jwks.json",
	})
	require.ErrorContains(t, err, "pinned HTTPS")
}

func TestJWKSProviderDoesNotFollowRedirectsAwayFromPinnedURL(t *testing.T) {
	t.Parallel()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	var redirectedRequests atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		redirectedRequests.Add(1)
		_, _ = fmt.Fprint(response, jwks("redirected", publicKey, auth.VerificationKeyActive))
	}))
	defer target.Close()
	pinned := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		http.Redirect(response, nil, target.URL, http.StatusFound)
	}))
	defer pinned.Close()
	provider, err := authn.NewJWKSProvider(authn.JWKSProviderConfig{
		Snapshot: jwks("active", publicKey, auth.VerificationKeyActive), URL: pinned.URL,
		HTTPClient: pinned.Client(), RefreshInterval: time.Minute,
	})
	require.NoError(t, err)
	_, err = provider.FindKey(t.Context(), "redirected")
	require.ErrorIs(t, err, auth.ErrInvalidFirstPartyAssertion)
	require.Zero(t, redirectedRequests.Load())
}

func TestJWKSProviderRefreshesKnownKeyStateAfterInterval(t *testing.T) {
	t.Parallel()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(response, jwks("active", publicKey, auth.VerificationKeyRevoked))
	}))
	defer server.Close()
	provider, err := authn.NewJWKSProvider(authn.JWKSProviderConfig{
		Snapshot: jwks("active", publicKey, auth.VerificationKeyActive), URL: server.URL,
		HTTPClient: server.Client(), RefreshInterval: time.Millisecond,
	})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		key, findErr := provider.FindKey(t.Context(), "active")
		return findErr == nil && key.State == auth.VerificationKeyRevoked
	}, time.Second, time.Millisecond)
}

func jwks(keyID string, publicKey ed25519.PublicKey, state auth.VerificationKeyState) string {
	return fmt.Sprintf(`{"keys":[{"kty":"OKP","crv":"Ed25519","x":"%s","kid":"%s","use":"sig","alg":"EdDSA","state":"%s"}]}`,
		base64.RawURLEncoding.EncodeToString(publicKey), keyID, state)
}

func TestJWKSRefreshCannotReactivateRevokedSnapshotKey(t *testing.T) {
	t.Parallel()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = fmt.Fprint(response, jwks("compromised", publicKey, auth.VerificationKeyActive))
	}))
	defer server.Close()
	provider, err := authn.NewJWKSProvider(authn.JWKSProviderConfig{
		Snapshot: jwks("compromised", publicKey, auth.VerificationKeyRevoked), URL: server.URL,
		HTTPClient: server.Client(), RefreshInterval: time.Minute,
	})
	require.NoError(t, err)
	_, err = provider.FindKey(t.Context(), "unknown-refresh-trigger")
	require.ErrorIs(t, err, auth.ErrInvalidFirstPartyAssertion)
	require.EqualValues(t, 1, requests.Load(), "exercise the real HTTPS refresh")
	key, err := provider.FindKey(t.Context(), "compromised")
	require.NoError(t, err)
	require.Equal(t, auth.VerificationKeyRevoked, key.State, "a stale discovery response must not undo deployed revocation")
}

func TestOlderJWKSResponseCannotRestoreRemovedKey(t *testing.T) {
	t.Parallel()
	oldKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	newKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	release := sync.OnceFunc(func() { close(releaseFirst) })
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if requests.Add(1) == 1 {
			close(firstStarted)
			select {
			case <-releaseFirst:
				_, _ = fmt.Fprint(response, jwks("old", oldKey, auth.VerificationKeyActive))
			case <-request.Context().Done():
			}
			return
		}
		_, _ = fmt.Fprint(response, jwks("new", newKey, auth.VerificationKeyActive))
	}))
	defer server.Close()
	defer release()
	provider, err := authn.NewJWKSProvider(authn.JWKSProviderConfig{
		Snapshot: jwks("old", oldKey, auth.VerificationKeyActive), URL: server.URL,
		HTTPClient: server.Client(), RefreshInterval: time.Nanosecond,
	})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	oldResult := make(chan error, 1)
	go func() {
		_, lookupErr := provider.FindKey(ctx, "old")
		oldResult <- lookupErr
	}()
	select {
	case <-firstStarted:
	case <-ctx.Done():
		t.Fatal("first refresh did not reach the controlled server")
	}
	key, err := provider.FindKey(ctx, "new")
	require.NoError(t, err)
	require.Equal(t, "new", key.ID, "newer response must have completed before releasing the stale one")
	release()
	select {
	case lookupErr := <-oldResult:
		require.ErrorIs(t, lookupErr, auth.ErrInvalidFirstPartyAssertion,
			"late completion of an older request must not restore the retired key")
	case <-ctx.Done():
		t.Fatal("old lookup did not complete")
	}
}

func TestObservedRevocationSurvivesOmissionAndKeyIDReuse(t *testing.T) {
	t.Parallel()
	oldKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	newKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	var document atomic.Value
	document.Store(jwks("compromised", oldKey, auth.VerificationKeyRevoked))
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(response, document.Load().(string))
	}))
	defer server.Close()
	provider, err := authn.NewJWKSProvider(authn.JWKSProviderConfig{
		Snapshot: jwks("compromised", oldKey, auth.VerificationKeyActive), URL: server.URL,
		HTTPClient: server.Client(), RefreshInterval: time.Nanosecond,
	})
	require.NoError(t, err)
	key, err := provider.FindKey(t.Context(), "compromised")
	require.NoError(t, err)
	require.Equal(t, auth.VerificationKeyRevoked, key.State)
	document.Store(jwks("replacement", newKey, auth.VerificationKeyActive))
	_, err = provider.FindKey(t.Context(), "replacement")
	require.NoError(t, err)
	document.Store(jwks("compromised", newKey, auth.VerificationKeyActive))
	key, err = provider.FindKey(t.Context(), "compromised")
	require.NoError(t, err)
	require.Equal(t, auth.VerificationKeyRevoked, key.State)
	require.Equal(t, oldKey, key.PublicKey, "reusing a revoked ID cannot replace its tombstone")
}

func TestLateRevocationStillOverridesNewerActiveResponse(t *testing.T) {
	t.Parallel()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	firstStarted, releaseFirst := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(releaseFirst) })
	var requests atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if requests.Add(1) == 1 {
			close(firstStarted)
			select {
			case <-releaseFirst:
				_, _ = fmt.Fprint(response, jwks("compromised", publicKey, auth.VerificationKeyRevoked))
			case <-request.Context().Done():
			}
			return
		}
		_, _ = fmt.Fprint(response, jwks("compromised", publicKey, auth.VerificationKeyActive))
	}))
	defer server.Close()
	defer release()
	provider, err := authn.NewJWKSProvider(authn.JWKSProviderConfig{
		Snapshot: jwks("compromised", publicKey, auth.VerificationKeyActive), URL: server.URL,
		HTTPClient: server.Client(), RefreshInterval: time.Nanosecond,
	})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	result := make(chan auth.VerificationKey, 1)
	go func() {
		key, _ := provider.FindKey(ctx, "compromised")
		result <- key
	}()
	select {
	case <-firstStarted:
	case <-ctx.Done():
		t.Fatal("first refresh did not start")
	}
	key, err := provider.FindKey(ctx, "compromised")
	require.NoError(t, err)
	require.Equal(t, auth.VerificationKeyActive, key.State, "revocation has not arrived yet")
	release()
	select {
	case key = <-result:
		require.Equal(t, auth.VerificationKeyRevoked, key.State, "explicit revocation must dominate response ordering")
	case <-ctx.Done():
		t.Fatal("delayed revocation did not finish")
	}
}
