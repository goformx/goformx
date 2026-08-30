package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type ViperConfig struct {
	viper          *viper.Viper
	configFilePath string
}

func NewViperConfig() *ViperConfig {
	v := viper.New()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	setDefaults(v)

	bindings := map[string][]string{
		"app.name": {"APP_NAME"}, "app.environment": {"APP_ENVIRONMENT", "APP_ENV"},
		"app.debug": {"APP_DEBUG"}, "app.log_level": {"APP_LOG_LEVEL"},
		"app.scheme": {"APP_SCHEME"}, "app.host": {"APP_HOST"}, "app.port": {"APP_PORT"},
		"app.read_timeout": {"APP_READ_TIMEOUT"}, "app.write_timeout": {"APP_WRITE_TIMEOUT"},
		"app.idle_timeout": {"APP_IDLE_TIMEOUT"}, "app.request_timeout": {"APP_REQUEST_TIMEOUT"},
		"database.driver": {"DB_DRIVER", "DB_CONNECTION"}, "database.host": {"DB_HOST"},
		"database.port": {"DB_PORT"}, "database.name": {"DB_NAME", "DB_DATABASE"},
		"database.username": {"DB_USERNAME", "DB_USER"}, "database.password": {"DB_PASSWORD"},
		"database.ssl_mode": {"DB_SSL_MODE"}, "database.max_open_conns": {"DB_MAX_OPEN_CONNS"},
		"database.max_idle_conns":               {"DB_MAX_IDLE_CONNS"},
		"database.conn_max_lifetime":            {"DB_CONN_MAX_LIFETIME"},
		"database.conn_max_idle_time":           {"DB_CONN_MAX_IDLE_TIME"},
		"security.first_party.enabled":          {"FIRST_PARTY_ASSERTION_ENABLED"},
		"security.first_party.issuer":           {"FIRST_PARTY_ASSERTION_ISSUER"},
		"security.first_party.audience":         {"FIRST_PARTY_ASSERTION_AUDIENCE"},
		"security.first_party.jwks_url":         {"FIRST_PARTY_ASSERTION_JWKS_URL"},
		"security.first_party.jwks_snapshot":    {"FIRST_PARTY_ASSERTION_JWKS_SNAPSHOT"},
		"security.first_party.refresh_interval": {"FIRST_PARTY_ASSERTION_REFRESH_INTERVAL"},
		"webhook.enabled":                       {"WEBHOOK_ENABLED"},
		"webhook.encryption_key":                {"WEBHOOK_ENCRYPTION_KEY"},
		"webhook.encryption_keyring":            {"WEBHOOK_ENCRYPTION_KEYRING"},
		"webhook.active_encryption_key_id":      {"WEBHOOK_ACTIVE_ENCRYPTION_KEY_ID"},
		"webhook.poll_interval":                 {"WEBHOOK_POLL_INTERVAL"},
		"webhook.request_timeout":               {"WEBHOOK_REQUEST_TIMEOUT"},
		"webhook.lock_timeout":                  {"WEBHOOK_LOCK_TIMEOUT"}, "webhook.max_attempts": {"WEBHOOK_MAX_ATTEMPTS"},
		"webhook.backoff_base": {"WEBHOOK_BACKOFF_BASE"}, "webhook.backoff_max": {"WEBHOOK_BACKOFF_MAX"},
	}
	for key, names := range bindings {
		args := append([]string{key}, names...)
		_ = v.BindEnv(args...)
	}
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.AddConfigPath("/etc/goforms")
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	return &ViperConfig{viper: v}
}

func (vc *ViperConfig) GetConfigFilePath() string { return vc.configFilePath }

func (vc *ViperConfig) LoadSchemaFirstAPI() (*Config, error) {
	if err := vc.viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("read configuration: %w", err)
		}
	} else {
		vc.configFilePath = vc.viper.ConfigFileUsed()
	}
	cfg := &Config{
		App: AppConfig{
			Name: vc.viper.GetString("app.name"), Version: vc.viper.GetString("app.version"),
			Environment: vc.viper.GetString("app.environment"), Debug: vc.viper.GetBool("app.debug"),
			LogLevel: vc.viper.GetString("app.log_level"), URL: vc.viper.GetString("app.url"),
			Scheme: vc.viper.GetString("app.scheme"), Host: vc.viper.GetString("app.host"),
			Port: vc.viper.GetInt("app.port"), ReadTimeout: vc.viper.GetDuration("app.read_timeout"),
			WriteTimeout: vc.viper.GetDuration("app.write_timeout"), IdleTimeout: vc.viper.GetDuration("app.idle_timeout"),
			RequestTimeout: vc.viper.GetDuration("app.request_timeout"),
		},
		Database: DatabaseConfig{
			Driver: vc.viper.GetString("database.driver"), Host: vc.viper.GetString("database.host"),
			Port: vc.viper.GetInt("database.port"), Name: vc.viper.GetString("database.name"),
			Username: vc.viper.GetString("database.username"), Password: vc.viper.GetString("database.password"),
			SSLMode: vc.viper.GetString("database.ssl_mode"), MaxOpenConns: vc.viper.GetInt("database.max_open_conns"),
			MaxIdleConns:    vc.viper.GetInt("database.max_idle_conns"),
			ConnMaxLifetime: vc.viper.GetDuration("database.conn_max_lifetime"),
			ConnMaxIdleTime: vc.viper.GetDuration("database.conn_max_idle_time"),
		},
		Security: SecurityConfig{
			RateLimit: RateLimitConfig{
				Enabled: vc.viper.GetBool("security.rate_limit.enabled"), RPS: vc.viper.GetInt("security.rate_limit.rps"),
				Burst:                 vc.viper.GetInt("security.rate_limit.burst"),
				PublicSubmissionRPS:   vc.viper.GetFloat64("security.rate_limit.public_submission_rps"),
				PublicSubmissionBurst: vc.viper.GetInt("security.rate_limit.public_submission_burst"),
				SubmissionsPerDay:     vc.viper.GetInt("security.rate_limit.submissions_per_day"),
			},
			FirstParty: FirstPartyConfig{
				Enabled:         vc.viper.GetBool("security.first_party.enabled"),
				Issuer:          vc.viper.GetString("security.first_party.issuer"),
				Audience:        vc.viper.GetString("security.first_party.audience"),
				JWKSURL:         vc.viper.GetString("security.first_party.jwks_url"),
				JWKSSnapshot:    vc.viper.GetString("security.first_party.jwks_snapshot"),
				RefreshInterval: vc.viper.GetDuration("security.first_party.refresh_interval"),
			},
		},
		Webhook: WebhookConfig{
			Enabled: vc.viper.GetBool("webhook.enabled"), EncryptionKey: vc.viper.GetString("webhook.encryption_key"),
			EncryptionKeyring:     vc.viper.GetString("webhook.encryption_keyring"),
			ActiveEncryptionKeyID: vc.viper.GetString("webhook.active_encryption_key_id"),
			PollInterval:          vc.viper.GetDuration("webhook.poll_interval"),
			RequestTimeout:        vc.viper.GetDuration("webhook.request_timeout"), LockTimeout: vc.viper.GetDuration("webhook.lock_timeout"),
			MaxAttempts: vc.viper.GetInt("webhook.max_attempts"), BackoffBase: vc.viper.GetDuration("webhook.backoff_base"),
			BackoffMax: vc.viper.GetDuration("webhook.backoff_max"),
		},
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "GoFormX")
	v.SetDefault("app.version", "1.0.0")
	v.SetDefault("app.environment", "development")
	v.SetDefault("app.debug", true)
	v.SetDefault("app.log_level", "info")
	v.SetDefault("app.url", "http://localhost:8080")
	v.SetDefault("app.scheme", "http")
	v.SetDefault("app.host", "localhost")
	v.SetDefault("app.port", defaultAppPort)
	v.SetDefault("app.read_timeout", defaultReadTimeout)
	v.SetDefault("app.write_timeout", defaultWriteTimeout)
	v.SetDefault("app.idle_timeout", defaultIdleTimeout)
	v.SetDefault("app.request_timeout", defaultRequestTimeout)
	v.SetDefault("database.driver", "postgres")
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", defaultDBPort)
	v.SetDefault("database.name", "goforms")
	v.SetDefault("database.username", "goforms")
	v.SetDefault("database.password", "")
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.max_open_conns", defaultMaxOpenConns)
	v.SetDefault("database.max_idle_conns", defaultMaxIdleConns)
	v.SetDefault("database.conn_max_lifetime", defaultConnLifetime)
	v.SetDefault("database.conn_max_idle_time", defaultConnIdleTime)
	v.SetDefault("security.rate_limit.enabled", true)
	v.SetDefault("security.rate_limit.rps", defaultRateLimitRPS)
	v.SetDefault("security.rate_limit.burst", defaultRateLimitBurst)
	v.SetDefault("security.rate_limit.public_submission_rps", defaultPublicSubmissionRPS)
	v.SetDefault("security.rate_limit.public_submission_burst", defaultPublicSubmissionBurst)
	v.SetDefault("security.rate_limit.submissions_per_day", defaultSubmissionsPerDay)
	v.SetDefault("security.first_party.enabled", false)
	v.SetDefault("security.first_party.issuer", "https://goformx.com")
	v.SetDefault("security.first_party.audience", "https://api.goformx.com")
	v.SetDefault("security.first_party.jwks_url", "https://goformx.com/.well-known/goformx-control-plane-jwks.json")
	v.SetDefault("security.first_party.jwks_snapshot", "")
	v.SetDefault("security.first_party.refresh_interval", 30*time.Second)
	v.SetDefault("webhook.enabled", false)
	v.SetDefault("webhook.encryption_key", "")
	v.SetDefault("webhook.poll_interval", time.Second)
	v.SetDefault("webhook.request_timeout", 10*time.Second)
	v.SetDefault("webhook.lock_timeout", 30*time.Second)
	v.SetDefault("webhook.max_attempts", 8)
	v.SetDefault("webhook.backoff_base", 5*time.Second)
	v.SetDefault("webhook.backoff_max", time.Hour)
}
