package webhook

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	domainwebhook "github.com/goformx/goforms/internal/domain/webhook"
)

type dispatchStore struct {
	delivery  *domainwebhook.Delivery
	event     *domainwebhook.Event
	delivered bool
	failed    bool
	retryable bool
	category  string
}

func (s *dispatchStore) ClaimDelivery(context.Context, time.Duration) (*domainwebhook.Delivery, *domainwebhook.Event, error) {
	delivery, event := s.delivery, s.event
	s.delivery, s.event = nil, nil
	return delivery, event, nil
}

func (s *dispatchStore) MarkDeliveryDelivered(context.Context, string, int, time.Time) error {
	s.delivered = true
	return nil
}

func (s *dispatchStore) MarkDeliveryFailed(
	_ context.Context,
	_, category string,
	_ *int,
	retryable bool,
	_ int,
	_, _ time.Duration,
	_ time.Time,
) error {
	s.failed, s.retryable, s.category = true, retryable, category
	return nil
}

type httpClientFunc func(*http.Request) (*http.Response, error)

func (f httpClientFunc) Do(request *http.Request) (*http.Response, error) { return f(request) }

func testCipher(t *testing.T) *domainwebhook.Cipher {
	t.Helper()
	key := sha256.Sum256([]byte("dispatcher test encryption key"))
	cipher, err := domainwebhook.NewCipher(base64.RawStdEncoding.EncodeToString(key[:]))
	require.NoError(t, err)
	return cipher
}

func TestDispatcherSignsStableDeliveryAndPreservesCustomHeaders(t *testing.T) {
	t.Parallel()
	cipher := testCipher(t)
	secret := "dispatcher-signing-secret-long-enough"
	encrypted, err := cipher.Encrypt(domainwebhook.SecretConfig{
		DestinationURL: "https://hooks.example/receive/token-value",
		Headers:        map[string]string{"Authorization": "Bearer encrypted-value"}, SigningSecret: secret,
	}, "form-a")
	require.NoError(t, err)
	oldKey := sha256.Sum256([]byte("dispatcher test encryption key"))
	newKey := sha256.Sum256([]byte("rotated dispatcher encryption key"))
	rotator, err := domainwebhook.NewKeyring("new",
		map[string]string{"new": base64.RawStdEncoding.EncodeToString(newKey[:])},
		base64.RawStdEncoding.EncodeToString(oldKey[:]))
	require.NoError(t, err)
	encrypted, changed, err := rotator.Reencrypt(encrypted, "form-a")
	require.NoError(t, err)
	require.True(t, changed)
	// The worker holds only the new key, but receivers must see the same signing
	// secret, custom headers, destination and stable delivery ID after rotation.
	cipher, err = domainwebhook.NewKeyring("new",
		map[string]string{"new": base64.RawStdEncoding.EncodeToString(newKey[:])}, "")
	require.NoError(t, err)
	now := time.Unix(1_800_000_000, 0).UTC()
	store := &dispatchStore{
		delivery: &domainwebhook.Delivery{ID: "delivery-a", FormID: "form-a",
			DestinationOrigin: "https://hooks.example", EncryptedConfig: encrypted, AttemptCount: 1},
		event: &domainwebhook.Event{ID: "delivery-a", Type: "submission.accepted", FormID: "form-a",
			SubmissionID: "submission-a", CreatedAt: now, Data: map[string]any{"email": "ada@example.com"}},
	}
	client := httpClientFunc(func(request *http.Request) (*http.Response, error) {
		body, readErr := io.ReadAll(request.Body)
		require.NoError(t, readErr)
		require.Equal(t, "delivery-a", request.Header.Get(HeaderDeliveryID))
		require.Equal(t, "Bearer encrypted-value", request.Header.Get("Authorization"))
		require.NoError(t, Verify(secret, request.Header.Get(HeaderDeliveryID), request.Header.Get(HeaderTimestamp),
			request.Header.Get(HeaderSignature), body, now, 5*time.Minute))
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	dispatcher := NewDispatcher(store, cipher, client, nil, DispatcherConfig{
		PollInterval: time.Second, LockTimeout: time.Minute, MaxAttempts: 8,
		BackoffBase: time.Second, BackoffMax: time.Minute,
	})
	dispatcher.now = func() time.Time { return now }
	processed, err := dispatcher.DispatchOne(t.Context())
	require.NoError(t, err)
	require.True(t, processed)
	require.True(t, store.delivered)
	require.False(t, store.failed)
}

func TestDispatcherClassifiesRetryableAndPermanentResponses(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		status    int
		retryable bool
		category  string
	}{
		{status: http.StatusTooManyRequests, retryable: true, category: "http_retryable"},
		{status: http.StatusBadRequest, retryable: false, category: "http_4xx"},
	} {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			cipher := testCipher(t)
			encrypted, err := cipher.Encrypt(domainwebhook.SecretConfig{
				DestinationURL: "https://hooks.example/receive",
				SigningSecret:  "dispatcher-signing-secret-long-enough",
			}, "form-a")
			require.NoError(t, err)
			store := &dispatchStore{
				delivery: &domainwebhook.Delivery{ID: "delivery-a", FormID: "form-a",
					DestinationOrigin: "https://hooks.example", EncryptedConfig: encrypted, AttemptCount: 1},
				event: &domainwebhook.Event{ID: "delivery-a", Data: map[string]any{}},
			}
			client := httpClientFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(""))}, nil
			})
			dispatcher := NewDispatcher(store, cipher, client, nil, DispatcherConfig{
				PollInterval: time.Second, LockTimeout: time.Minute, MaxAttempts: 8,
				BackoffBase: time.Second, BackoffMax: time.Minute,
			})
			processed, err := dispatcher.DispatchOne(t.Context())
			require.NoError(t, err)
			require.True(t, processed)
			require.True(t, store.failed)
			require.Equal(t, test.retryable, store.retryable)
			require.Equal(t, test.category, store.category)
		})
	}
}
