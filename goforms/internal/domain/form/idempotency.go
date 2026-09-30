package form

import "errors"

// ErrFormIdempotencyConflict means a create key was already committed with
// different accepted form inputs in the same organization.
var ErrFormIdempotencyConflict = errors.New("form idempotency key conflict")
