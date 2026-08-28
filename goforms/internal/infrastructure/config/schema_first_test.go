package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func schemaFirstLoader() *ViperConfig {
	loader := NewViperConfig()
	loader.viper.Set("database.password", "test-password")

	// These defaults model the legacy runtime and are deliberately invalid
	// without secrets. The schema-first API does not consume them.
	loader.viper.Set("security.csrf.enabled", true)
	loader.viper.Set("security.csrf.secret", "")
	loader.viper.Set("security.assertion.secret", "")
	loader.viper.Set("session.type", "cookie")
	loader.viper.Set("session.secret", "")

	return loader
}

func TestLoadSchemaFirstAPIIgnoresLegacyRuntimeRequirements(t *testing.T) {
	config, err := schemaFirstLoader().LoadSchemaFirstAPI()

	require.NoError(t, err)
	assert.Equal(t, "", config.Security.CSRF.Secret)
	assert.Equal(t, "", config.Security.Assertion.Secret)
	assert.Equal(t, "", config.Session.Secret)
}

func TestLoadStillValidatesLegacyRuntimeRequirements(t *testing.T) {
	_, err := schemaFirstLoader().Load()

	require.Error(t, err)
	assert.ErrorContains(t, err, "CSRF secret is required")
	assert.ErrorContains(t, err, "assertion secret is required")
	assert.ErrorContains(t, err, "session secret is required")
}

func TestLoadSchemaFirstAPIRejectsInvalidRateLimit(t *testing.T) {
	loader := schemaFirstLoader()
	loader.viper.Set("security.rate_limit.rps", 0)

	_, err := loader.LoadSchemaFirstAPI()

	require.Error(t, err)
	assert.ErrorContains(t, err, "rate limit RPS must be positive")
}
