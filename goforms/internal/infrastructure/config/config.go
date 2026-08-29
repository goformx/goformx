// Package config loads configuration for the supported schema-first API.
package config

import (
	"fmt"
	"strings"
	"time"
)

type Config struct {
	App      AppConfig      `json:"app"`
	Database DatabaseConfig `json:"database"`
	Security SecurityConfig `json:"security"`
	Webhook  WebhookConfig  `json:"webhook"`
}

type SecurityConfig struct {
	RateLimit  RateLimitConfig  `json:"rate_limit"`
	FirstParty FirstPartyConfig `json:"first_party"`
}

type FirstPartyConfig struct {
	Enabled         bool          `json:"enabled"`
	Issuer          string        `json:"issuer"`
	Audience        string        `json:"audience"`
	JWKSURL         string        `json:"jwks_url"`
	JWKSSnapshot    string        `json:"jwks_snapshot"`
	RefreshInterval time.Duration `json:"refresh_interval"`
}

type RateLimitConfig struct {
	Enabled               bool    `json:"enabled"`
	RPS                   int     `json:"rps"`
	Burst                 int     `json:"burst"`
	PublicSubmissionRPS   float64 `json:"public_submission_rps"`
	PublicSubmissionBurst int     `json:"public_submission_burst"`
	SubmissionsPerDay     int     `json:"submissions_per_day"`
}

func (c *Config) validate() error {
	var errs []string
	if err := c.App.Validate(); err != nil {
		errs = append(errs, err.Error())
	}
	if err := c.Database.Validate(); err != nil {
		errs = append(errs, err.Error())
	}
	if c.Security.RateLimit.Enabled {
		if c.Security.RateLimit.RPS <= 0 {
			errs = append(errs, "rate limit RPS must be positive")
		}
		if c.Security.RateLimit.Burst <= 0 {
			errs = append(errs, "rate limit burst must be positive")
		}
		if c.Security.RateLimit.PublicSubmissionRPS <= 0 {
			errs = append(errs, "public submission rate limit RPS must be positive")
		}
		if c.Security.RateLimit.PublicSubmissionBurst <= 0 {
			errs = append(errs, "public submission rate limit burst must be positive")
		}
		if c.Security.RateLimit.SubmissionsPerDay <= 0 {
			errs = append(errs, "daily submission limit must be positive")
		}
	}
	if c.Security.FirstParty.Enabled {
		if c.Security.FirstParty.Issuer == "" || c.Security.FirstParty.Audience == "" {
			errs = append(errs, "first-party assertion issuer and audience are required")
		}
		if c.Security.FirstParty.JWKSSnapshot == "" {
			errs = append(errs, "first-party assertion JWKS snapshot is required")
		}
		if c.Security.FirstParty.RefreshInterval <= 0 {
			errs = append(errs, "first-party assertion refresh interval must be positive")
		}
	}
	if err := c.Webhook.Validate(); err != nil {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errs, "; "))
	}
	return nil
}
