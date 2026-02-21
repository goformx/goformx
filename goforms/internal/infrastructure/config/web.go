package config

import "time"

// FormConfig holds form-related configuration
type FormConfig struct {
	MaxFileSize      int64            `json:"max_file_size"`
	AllowedFileTypes []string         `json:"allowed_file_types"`
	MaxFields        int              `json:"max_fields"`
	MaxMemory        int64            `json:"max_memory"`
	Validation       ValidationConfig `json:"validation"`
}

// ValidationConfig holds form validation configuration
type ValidationConfig struct {
	StrictMode bool `json:"strict_mode"`
	MaxErrors  int  `json:"max_errors"`
}

// APIConfig holds API-related configuration
type APIConfig struct {
	Version    string          `json:"version"`
	Prefix     string          `json:"prefix"`
	Timeout    time.Duration   `json:"timeout"`
	MaxRetries int             `json:"max_retries"`
	RateLimit  RateLimitConfig `json:"rate_limit"`
}

// UserConfig holds user-related configuration
type UserConfig struct {
	Admin   AdminUserConfig   `json:"admin"`
	Default DefaultUserConfig `json:"default"`
}

// AdminUserConfig holds admin user configuration
type AdminUserConfig struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// DefaultUserConfig holds default user configuration
type DefaultUserConfig struct {
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}
