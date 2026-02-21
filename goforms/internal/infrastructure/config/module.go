// Package config provides configuration module for Fx dependency injection
package config

import (
	"go.uber.org/fx"
)

// Module provides the configuration module for Fx
var Module = fx.Module("config",
	// Use Viper configuration provider instead of LoadFromEnv
	NewViperConfigProvider(),
	fx.Provide(NewAppConfig),
	fx.Provide(NewDatabaseConfig),
	fx.Provide(NewSecurityConfig),
	fx.Provide(NewSessionConfig),
)

// Individual config providers for fine-grained dependency injection

// NewAppConfig provides app configuration
func NewAppConfig(cfg *Config) AppConfig {
	return cfg.App
}

// NewDatabaseConfig provides database configuration
func NewDatabaseConfig(cfg *Config) DatabaseConfig {
	return cfg.Database
}

// NewSecurityConfig provides security configuration
func NewSecurityConfig(cfg *Config) SecurityConfig {
	return cfg.Security
}

// NewSessionConfig provides session configuration
func NewSessionConfig(cfg *Config) SessionConfig {
	return cfg.Session
}
