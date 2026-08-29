// Package constants contains the public schema-first HTTP contract constants.
package constants

const (
	PathV1Forms       = "/v1/forms"
	PathV1PublicForms = "/v1/public/forms"

	HeaderIdempotencyKey = "Idempotency-Key"
	HeaderSchemaVersion  = "X-GoFormX-Schema-Version"
	HeaderReplay         = "Idempotency-Replayed"
	HeaderTraceID        = "X-Trace-Id"

	ContentTypeJSONSchema = "application/schema+json"
)
