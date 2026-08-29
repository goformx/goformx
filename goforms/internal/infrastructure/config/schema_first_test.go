package config

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testLoader() *ViperConfig {
	loader := NewViperConfig()
	loader.viper.Set("database.password", "test-password")
	return loader
}

func TestFirstPartyConfigurationFailsClosedWithoutSnapshot(t *testing.T) {
	loader := testLoader()
	loader.viper.Set("security.first_party.enabled", true)
	_, err := loader.LoadSchemaFirstAPI()
	require.ErrorContains(t, err, "JWKS snapshot")

	loader.viper.Set("security.first_party.jwks_snapshot", `{"keys":[{"kid":"local"}]}`)
	cfg, err := loader.LoadSchemaFirstAPI()
	require.NoError(t, err)
	require.True(t, cfg.Security.FirstParty.Enabled)
	require.Equal(t, "https://goformx.com", cfg.Security.FirstParty.Issuer)
	require.Equal(t, "https://api.goformx.com", cfg.Security.FirstParty.Audience)
	require.Equal(t, 30*time.Second, cfg.Security.FirstParty.RefreshInterval)
}

func TestSchemaFirstConfiguration(t *testing.T) {
	loader := testLoader()
	cfg, err := loader.LoadSchemaFirstAPI()
	require.NoError(t, err)
	require.Equal(t, 1000, cfg.Security.RateLimit.SubmissionsPerDay)

	loader.viper.Set("security.rate_limit.public_submission_rps", 0)
	_, err = loader.LoadSchemaFirstAPI()
	require.ErrorContains(t, err, "public submission rate limit RPS")
}

func TestWebhookConfigurationRequiresValidKey(t *testing.T) {
	loader := testLoader()
	loader.viper.Set("webhook.enabled", true)
	loader.viper.Set("webhook.encryption_key", "invalid")
	_, err := loader.LoadSchemaFirstAPI()
	require.ErrorContains(t, err, "webhook encryption key")

	key := sha256.Sum256([]byte("schema-first webhook test key"))
	loader.viper.Set("webhook.encryption_key", base64.RawStdEncoding.EncodeToString(key[:]))
	cfg, err := loader.LoadSchemaFirstAPI()
	require.NoError(t, err)
	require.True(t, cfg.Webhook.Enabled)
}
