package webhook

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func testEncryptionKey(value byte) string {
	return base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{value}, EncryptionKeyBytes))
}

func TestKeyringMigratesLegacyAndPreviousKeysWithoutChangingPlaintext(t *testing.T) {
	t.Parallel()
	oldKey, newKey := testEncryptionKey(1), testEncryptionKey(2)
	legacy, err := NewCipher(oldKey)
	require.NoError(t, err)
	old, err := NewKeyring("old", map[string]string{"old": oldKey}, "")
	require.NoError(t, err)
	current, err := NewKeyring("new", map[string]string{"old": oldKey, "new": newKey}, oldKey)
	require.NoError(t, err)
	newOnly, err := NewKeyring("new", map[string]string{"new": newKey}, "")
	require.NoError(t, err)
	// Unknown future fields and number spellings must survive rotation exactly.
	plaintext := []byte(`{"signingSecret":"canary-secret","future":9007199254740993,"decimal":1.000}`)
	for _, writer := range []*Cipher{legacy, old, current} {
		input, err := writer.encryptBytes(plaintext, "form-a")
		require.NoError(t, err)
		output, changed, err := current.Reencrypt(input, "form-a")
		require.NoError(t, err)
		require.Equal(t, writer != current, changed)
		decoded, keyID, err := newOnly.decryptBytes(output, "form-a")
		require.NoError(t, err)
		require.Equal(t, "new", keyID)
		require.Equal(t, plaintext, decoded)
		require.NotContains(t, string(output), "canary-secret")
		again, changed, err := current.Reencrypt(output, "form-a")
		require.NoError(t, err)
		require.False(t, changed)
		require.Equal(t, output, again)
		_, err = current.Decrypt(output, "form-b")
		require.Error(t, err)
		if writer != current {
			_, err = newOnly.Decrypt(input, "form-a")
			require.Error(t, err, "retired keys are never guessed")
		}
	}
}

func TestKeyringAuthenticatesHeaderAndRejectsWrongKeyTruncationAndTampering(t *testing.T) {
	t.Parallel()
	// Even accidentally aliasing the same key material must not permit relabeling.
	key := testEncryptionKey(3)
	ring, err := NewKeyring("old", map[string]string{"old": key, "new": key}, key)
	require.NoError(t, err)
	encrypted, err := ring.Encrypt(SecretConfig{SigningSecret: "canary"}, "form")
	require.NoError(t, err)
	for length := range len(encrypted) {
		_, err := ring.Decrypt(encrypted[:length], "form")
		require.Error(t, err)
	}
	for index := range len(encrypted) {
		tampered := bytes.Clone(encrypted)
		tampered[index] ^= 1
		_, err := ring.Decrypt(tampered, "form")
		require.Error(t, err)
	}
	relabelled := bytes.Replace(encrypted, []byte("old"), []byte("new"), 1)
	_, err = ring.Decrypt(relabelled, "form")
	require.Error(t, err)
	wrong, err := NewKeyring("old", map[string]string{"old": testEncryptionKey(4)}, key)
	require.NoError(t, err)
	_, err = wrong.Decrypt(encrypted, "form")
	require.Error(t, err, "legacy key cannot rescue a tagged authentication failure")
	_, _, err = wrong.Reencrypt(encrypted, "form")
	require.Error(t, err, "already-current key ID is not proof of authentication")
}

func TestKeyringRejectsInvalidConfiguration(t *testing.T) {
	t.Parallel()
	key := testEncryptionKey(5)
	for _, id := range []string{"", "key.with.dots", "canary\nsecret", "é", strings.Repeat("a", MaxEncryptionKeyIDBytes+1)} {
		_, err := NewKeyring(id, map[string]string{id: key}, "")
		require.Error(t, err)
		require.NotContains(t, err.Error(), "canary")
	}
	for _, keys := range []map[string]string{nil, {"other": key}, {"key": "secret"}, {"key": key, "invalid id": key}} {
		_, err := NewKeyring("key", keys, "")
		require.Error(t, err)
	}
	_, err := NewKeyring("key", map[string]string{"key": key}, "invalid legacy secret")
	require.Error(t, err)
	legacy, err := NewCipher(key)
	require.NoError(t, err)
	_, _, err = legacy.Reencrypt(nil, "form")
	require.Error(t, err)
	var disabled *Cipher
	_, _, err = disabled.Reencrypt(nil, "form")
	require.Error(t, err)
}
