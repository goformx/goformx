package webhook

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fixedResolver map[string][]netip.Addr

func (r fixedResolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	return r[host], nil
}

func TestSafeClientRechecksPortBeforeDial(t *testing.T) {
	t.Parallel()
	policy := NewDestinationPolicy(fixedResolver{
		"public.example": {netip.MustParseAddr("8.8.8.8")},
	})
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://public.example/hooks", nil)
	require.NoError(t, err)
	response, err := policy.Client(time.Second).Do(request)
	if response != nil {
		require.NoError(t, response.Body.Close())
	}
	require.ErrorContains(t, err, "HTTPS port 443")
}

func TestDestinationPolicyBlocksSSRFClasses(t *testing.T) {
	t.Parallel()
	policy := NewDestinationPolicy(fixedResolver{
		"public.example":  {netip.MustParseAddr("8.8.8.8")},
		"private.example": {netip.MustParseAddr("10.0.0.1")},
		"mixed.example":   {netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("127.0.0.1")},
	})
	target, err := policy.Validate(t.Context(), "https://public.example/hooks")
	require.NoError(t, err)
	require.Equal(t, "public.example", target.Hostname())
	for _, candidate := range []string{
		"http://public.example/hooks",
		"https://localhost/hooks",
		"https://private.example/hooks",
		"https://mixed.example/hooks",
		"https://public.example:8443/hooks",
		"https://user:secret@public.example/hooks",
		"https://public.example/hooks?token=secret",
	} {
		_, err := policy.Validate(t.Context(), candidate)
		require.Error(t, err, candidate)
	}
}

func TestSignatureAuthenticatesBodyAndReplayWindow(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_800_000_000, 0).UTC()
	timestamp := "1800000000"
	body := []byte(`{"submissionId":"abc"}`)
	deliveryID := "delivery-a"
	signature := Sign("signing-secret", deliveryID, timestamp, body)
	require.NoError(t, Verify("signing-secret", deliveryID, timestamp, signature, body, now, 5*time.Minute))
	require.Error(t, Verify("signing-secret", "delivery-b", timestamp, signature, body, now, 5*time.Minute))
	require.Error(t, Verify("signing-secret", deliveryID, timestamp, signature, []byte("{}"), now, 5*time.Minute))
	require.Error(t, Verify("signing-secret", deliveryID, timestamp, signature, body, now.Add(6*time.Minute), 5*time.Minute))
}

func TestEncryptionKeyFixtureIsThirtyTwoBytes(t *testing.T) {
	t.Parallel()
	sum := sha256.Sum256([]byte("test webhook encryption key"))
	require.Len(t, base64.RawStdEncoding.EncodeToString(sum[:]), 43)
}
