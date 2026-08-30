package webhook

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSigningSecretRotationPreservesEncryptedFields(t *testing.T) {
	t.Parallel()
	cipher, err := NewKeyring("active", map[string]string{"active": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))}, "")
	require.NoError(t, err)
	original := []byte(`{"destinationUrl":"https://example.com/private","headers":{"Authorization":"Bearer private"},"signingSecret":"old-secret-at-least-thirty-two-characters","future":{"number":12345678901234567890}}`)
	encrypted, err := cipher.encryptBytes(original, "form-a")
	require.NoError(t, err)
	secret := strings.Repeat("new-secret", 4)
	rotated, err := cipher.RotateSigningSecret(encrypted, "form-a", secret)
	require.NoError(t, err)
	require.NotEqual(t, encrypted, rotated)
	plaintext, _, err := cipher.decryptBytes(rotated, "form-a")
	require.NoError(t, err)
	var before, after map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(original, &before))
	require.NoError(t, json.Unmarshal(plaintext, &after))
	for _, field := range []string{"destinationUrl", "headers", "future"} {
		require.JSONEq(t, string(before[field]), string(after[field]))
	}
	require.Contains(t, string(plaintext), "12345678901234567890")
	config, err := cipher.Decrypt(rotated, "form-a")
	require.NoError(t, err)
	require.Equal(t, secret, config.SigningSecret)
	_, err = cipher.RotateSigningSecret(encrypted, "form-b", secret)
	require.Error(t, err)
	_, err = cipher.RotateSigningSecret(encrypted, "form-a", "short")
	require.ErrorIs(t, err, ErrInvalidChange)
	for _, change := range []EndpointChange{{}, {Enabled: new(bool), SigningSecret: &secret}} {
		require.ErrorIs(t, change.Validate(), ErrInvalidChange)
	}
}

func TestSigningSecretLengthMatchesContractCharacters(t *testing.T) {
	t.Parallel()
	for _, character := range []string{"a", "界"} {
		for _, length := range []int{0, 31, 32, 256, 257} {
			secret := strings.Repeat(character, length)
			require.Equal(t, length >= 32 && length <= 256, ValidSigningSecret(secret))
		}
	}
	require.False(t, ValidSigningSecret(strings.Repeat("a", 32)+string([]byte{0xff})))
}
