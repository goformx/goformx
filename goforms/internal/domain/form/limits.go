// Package form contains cross-layer invariants for the schema-first form runtime.
package form

import "errors"

var ErrSubmissionLimitExceeded = errors.New("daily submission limit exceeded")

const (
	DefaultPublicSubmissionRPS   = 1.0
	DefaultPublicSubmissionBurst = 10
	DefaultSubmissionsPerDay     = 1000
)
