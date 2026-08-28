package webhook

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCipherRoundTripAndFormBinding(t *testing.T) {
	t.Parallel()
	key := base64.RawStdEncoding.EncodeToString(make([]byte, EncryptionKeyBytes))
	cipher, err := NewCipher(key)
	require.NoError(t, err)
	input := SecretConfig{Headers: map[string]string{"Authorization": "Bearer secret"},
		SigningSecret: "a-secret-long-enough-for-signatures"}
	encrypted, err := cipher.Encrypt(input, "form-a")
	require.NoError(t, err)
	require.NotContains(t, string(encrypted), "Bearer secret")

	output, err := cipher.Decrypt(encrypted, "form-a")
	require.NoError(t, err)
	require.Equal(t, input, output)
	_, err = cipher.Decrypt(encrypted, "form-b")
	require.Error(t, err)
}

func TestCipherRejectsInvalidKey(t *testing.T) {
	t.Parallel()
	_, err := NewCipher("short")
	require.Error(t, err)
}
