// Package config provides validation utilities for Viper-based configuration
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// File permission constants
const (
	permUserWrite = 0o200
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
	Value   any
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s (value: %v)", e.Field, e.Message, e.Value)
}

// ValidationResult contains the results of configuration validation
type ValidationResult struct {
	IsValid bool
	Errors  []ValidationError
}

// AddError adds a validation error to the result
func (r *ValidationResult) AddError(field, message string, value any) {
	r.IsValid = false
	r.Errors = append(r.Errors, ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	})
}

// ValidateConfig validates the complete configuration with detailed error reporting
func ValidateConfig(cfg *Config) ValidationResult {
	result := ValidationResult{IsValid: true}

	// Validate each configuration section
	validateAppConfig(cfg.App, &result)
	validateDatabaseConfig(cfg.Database, &result)
	validateSecurityConfig(cfg.Security, &result)
	validateSessionConfig(cfg.Session, &result)

	// Validate cross-section dependencies
	validateCrossSectionDependencies(cfg, &result)

	return result
}

// validateCrossSectionDependencies validates dependencies between configuration sections
func validateCrossSectionDependencies(cfg *Config, result *ValidationResult) {
	// Check if TLS is enabled but secure cookies are disabled
	if cfg.Security.TLS.Enabled && !cfg.Security.SecureCookie {
		result.AddError("security.secure_cookie",
			"secure cookies should be enabled when TLS is enabled",
			cfg.Security.SecureCookie)
	}
}

// Helper functions

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

// isWritableDirectory checks if a directory is writable
func isWritableDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir() && (info.Mode()&permUserWrite) != 0
}

// ValidateEnvironmentVariables validates that required environment variables are set
func ValidateEnvironmentVariables() ValidationResult {
	result := ValidationResult{IsValid: true}

	// Check for required environment variables
	requiredVars := []string{
		"APP_NAME",
		"DB_HOST",
		"DB_NAME",
		"DB_USERNAME",
		"DB_PASSWORD",
	}

	for _, envVar := range requiredVars {
		if os.Getenv(envVar) == "" {
			result.AddError(envVar, "required environment variable is not set", "")
		}
	}

	// Validate environment variable formats
	if port := os.Getenv("APP_PORT"); port != "" {
		if _, err := strconv.Atoi(port); err != nil {
			result.AddError("APP_PORT", "must be a valid integer", port)
		}
	}

	if timeout := os.Getenv("APP_READ_TIMEOUT"); timeout != "" {
		if _, err := time.ParseDuration(timeout); err != nil {
			result.AddError("APP_READ_TIMEOUT", "must be a valid duration", timeout)
		}
	}

	return result
}
