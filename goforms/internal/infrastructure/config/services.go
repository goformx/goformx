package config

import (
	"time"
)

// SessionConfig holds session-related configuration
type SessionConfig struct {
	Type       string        `json:"type"`
	Secret     string        `json:"secret"`
	MaxAge     time.Duration `json:"max_age"`
	Domain     string        `json:"domain"`
	Path       string        `json:"path"`
	Secure     bool          `json:"secure"`
	HTTPOnly   bool          `json:"http_only"`
	SameSite   string        `json:"same_site"`
	Store      string        `json:"store"`
	StoreFile  string        `json:"store_file"`
	CookieName string        `json:"cookie_name"`
}
