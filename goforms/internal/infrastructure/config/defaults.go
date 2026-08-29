package config

import (
	"time"

	domainform "github.com/goformx/goforms/internal/domain/form"
)

const (
	defaultAppPort               = 8080
	defaultDBPort                = 5432
	defaultReadTimeout           = 15 * time.Second
	defaultWriteTimeout          = 15 * time.Second
	defaultIdleTimeout           = 60 * time.Second
	defaultRequestTimeout        = 30 * time.Second
	defaultConnLifetime          = 5 * time.Minute
	defaultConnIdleTime          = 5 * time.Minute
	defaultMaxOpenConns          = 25
	defaultMaxIdleConns          = 25
	defaultRateLimitRPS          = 100
	defaultRateLimitBurst        = 200
	defaultPublicSubmissionRPS   = domainform.DefaultPublicSubmissionRPS
	defaultPublicSubmissionBurst = domainform.DefaultPublicSubmissionBurst
	defaultSubmissionsPerDay     = domainform.DefaultSubmissionsPerDay
)
