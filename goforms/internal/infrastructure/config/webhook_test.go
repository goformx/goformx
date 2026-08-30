package config

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWebhookKeyringConfigRejectsAmbiguityAndDoesNotExposeSecrets(t *testing.T) {
	t.Parallel()
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	for _, invalid := range []string{
		`null`, `[]`, `{}`, `{"key":null}`, `{"key":1}`, `{"key":"canary","key":"canary"}`,
		`{"key":"` + key + `"} {}`, `{"key":"canary"}`, strings.Repeat("canary", 2000),
	} {
		_, err := (WebhookConfig{ActiveEncryptionKeyID: "key", EncryptionKeyring: invalid}).Cipher()
		require.Error(t, err)
		require.NotContains(t, err.Error(), "canary")
	}
	configuration := WebhookConfig{ActiveEncryptionKeyID: "key", EncryptionKeyring: `{"key":"` + key + `"}`, EncryptionKey: key}
	_, err := configuration.Cipher()
	require.NoError(t, err)
	encoded, err := json.Marshal(configuration)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), key)
}

func TestWebhookKeyringEnvironmentReachesRuntimeConfiguration(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	t.Setenv("WEBHOOK_ENABLED", "true")
	t.Setenv("DB_PASSWORD", "synthetic-test-password")
	t.Setenv("WEBHOOK_ENCRYPTION_KEYRING", `{"current":"`+key+`"}`)
	t.Setenv("WEBHOOK_ACTIVE_ENCRYPTION_KEY_ID", "current")
	configuration, err := NewViperConfig().LoadSchemaFirstAPI()
	require.NoError(t, err)
	require.Equal(t, "current", configuration.Webhook.ActiveEncryptionKeyID)
	_, err = configuration.Webhook.Cipher()
	require.NoError(t, err)
}
