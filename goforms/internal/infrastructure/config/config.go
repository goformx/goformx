// Package config provides configuration management for the GoForms application.
// It defines the configuration structures and validation logic used by the Viper-based configuration system.
package config

import (
	"errors"
	"fmt"
	"strings"
)

// Config represents the complete application configuration
type Config struct {
	App      AppConfig      `json:"app"`
	Database DatabaseConfig `json:"database"`
	Security SecurityConfig `json:"security"`
	Session  SessionConfig  `json:"session"`
}

// validateConfig validates the configuration
func (c *Config) validateConfig() error {
	var errs []string

	// Validate core config sections
	if err := c.validateCoreConfig(); err != nil {
		errs = append(errs, err.Error())
	}

	// Validate conditional config sections
	if err := c.validateConditionalConfig(); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errs, "; "))
	}

	return nil
}

// validateCoreConfig validates the core configuration sections
func (c *Config) validateCoreConfig() error {
	var errs []string

	// Validate App config
	if err := c.App.Validate(); err != nil {
		errs = append(errs, err.Error())
	}

	// Validate Database config
	if err := c.Database.Validate(); err != nil {
		errs = append(errs, err.Error())
	}

	// Validate Security config
	if err := c.Security.Validate(); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}

	return nil
}

// validateConditionalConfig validates configuration sections that depend on other settings
func (c *Config) validateConditionalConfig() error {
	return c.validateSessionConfig()
}

// validateSessionConfig validates session configuration
func (c *Config) validateSessionConfig() error {
	if c.Session.Type != "none" && c.Session.Secret == "" {
		return errors.New("session secret is required when session type is not 'none'")
	}

	return nil
}

// GetConfigSummary returns a summary of the current configuration
func (c *Config) GetConfigSummary() map[string]any {
	return map[string]any{
		"app": map[string]any{
			"name":        c.App.Name,
			"environment": c.App.Environment,
			"debug":       c.App.Debug,
			"url":         c.App.GetServerURL(),
		},
		"database": map[string]any{
			"driver": c.Database.Driver,
			"host":   c.Database.Host,
			"port":   c.Database.Port,
			"name":   c.Database.Name,
		},
		"security": map[string]any{
			"csrf_enabled":       c.Security.CSRF.Enabled,
			"cors_enabled":       c.Security.CORS.Enabled,
			"rate_limit_enabled": c.Security.RateLimit.Enabled,
			"csp_enabled":        c.Security.CSP.Enabled,
		},
		"services": map[string]any{
			"session_type": c.Session.Type,
		},
	}
}

// IsValid checks if the configuration is valid
func (c *Config) IsValid() bool {
	return c.validateConfig() == nil
}

// GetEnvironment returns the current environment
func (c *Config) GetEnvironment() string {
	return strings.ToLower(c.App.Environment)
}

// IsProduction returns true if running in production
func (c *Config) IsProduction() bool {
	return c.GetEnvironment() == "production"
}

// IsDevelopment returns true if running in development
func (c *Config) IsDevelopment() bool {
	return c.GetEnvironment() == "development"
}

// IsStaging returns true if running in staging
func (c *Config) IsStaging() bool {
	return c.GetEnvironment() == "staging"
}

// IsTest returns true if running in test
func (c *Config) IsTest() bool {
	return c.GetEnvironment() == "test"
}
