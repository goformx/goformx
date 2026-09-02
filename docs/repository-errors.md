# Repository error contract

Persistence exposes stable error categories through `errors.Is`; HTTP handlers
must not classify errors by their text. The supported mapping is:

| Repository/domain category | HTTP result | Source |
| --- | --- | --- |
| `ErrNotFound` | `404 not_found` | Explicit tenant-scoped absence only |
| `ErrInvalidInput` | `400 invalid_request` | Identifier validation, GORM invalid data, PostgreSQL `22P02` |
| `ErrConflict` | `409 conflict` | GORM duplicate key, PostgreSQL `23505`, or an explicit repository conflict |
| `model.ErrPreconditionFailed` | `412 precondition_failed` | Stale conditional form mutation |
| `ErrDatabaseError` or unknown | `500 internal_error` | Every unclassified persistence/driver failure |

Tenant-scoped lookup misses remain indistinguishable from foreign-tenant
resources. PostgreSQL foreign-key/check failures, serialization failures, and
other unexpected SQL states intentionally stay internal errors: treating those
as caller mistakes could hide a persistence defect. Logs may carry only the
bounded category and request ID; driver text and rejected values are not HTTP
response data.
