# Schema-first runtime boundaries

## Supported production graph

The production binary is built exclusively from goforms/cmd/api. That composition root performs explicit construction in this order:

    config -> logger -> PostgreSQL connection
           -> form repository + token repository
           -> v1 HTTP handler -> Echo router -> net/http server

There is no dependency-injection container in this graph. The router installs one visible global middleware chain: request ID, panic recovery, security headers, and optional configured rate limiting. Route-specific middleware is limited to public-form CORS, scoped service-token authorization, and per-form public-submission admission control.

## Dependency direction

| Boundary | Current package | May depend on |
| --- | --- | --- |
| Composition root | cmd/api | every supported boundary needed to construct the process |
| Form and submission model | internal/domain/form/model | standard library and domain values |
| Service-token model | internal/domain/auth | standard library |
| Schema validation | internal/application/validation | form model and JSON Schema library |
| HTTP API | internal/application/handlers/web v1 files | narrow form persistence and auth interfaces, schema validation, domain models |
| Service-token authorization | internal/application/middleware/serviceauth | auth domain and its narrow repository interface |
| PostgreSQL adapters | internal/infrastructure/repository/form and token | domain interfaces/models and database adapter |
| OpenAPI and schema contracts | contracts and contracts/schema | no runtime implementation |
| Migrations | migrations/postgresql | PostgreSQL only |

The v1 handler owns a deliberately narrow V1Repository interface. It cannot call legacy user, plan, pagination, browser-session, or mutable-schema operations. Its RequestLogger interface keeps it independent of the concrete logging implementation. Go internal package visibility prevents consumers outside the module from importing implementation details.

## Public-write and read boundaries

Canonical schema compilation rejects remote references and schemas outside the configured depth, node, and pattern budgets. Successful compilations are cached by a digest of the schema, so repeated submissions validate against a bounded compiled-schema cache.

The public submission route applies a bounded, per-form token bucket before decoding or validating a request. PostgreSQL then serializes admission for each form and enforces a rolling 24-hour submission quota in the same transaction as the idempotent insert. A replay of an accepted idempotency key remains readable after the quota is exhausted.

Submission reads require the owning service token, return the same not-found response for missing and foreign forms, and use a bounded opaque cursor ordered by submission timestamp and ID. These controls and their accepted residual risks are recorded in `docs/security/threat-model.md` and `docs/security/abuse-case-gate.md`.

## Legacy isolation

The remaining Fx modules, legacy form handler, browser sessions, CSRF, HMAC assertions, plan enforcement, shadow users, and duplicate middleware packages are reference code only. They are not imported by cmd/api and therefore cannot be mounted in the supported production process.

The cmd/api route test enforces this cutover by rejecting any mounted /api/ route and by proving that legacy assertion headers cannot authenticate a v1 request. Issue #83 owns physical deletion of the isolated packages after the production cleanup branch lands.

## Change rules

- Business invariants belong in domain models and must test without HTTP, PostgreSQL, global logging, or configuration.
- HTTP handlers depend on narrow behavioral interfaces, never concrete stores.
- PostgreSQL implements those interfaces and does not define domain behavior.
- New middleware must be added to the single composition-root chain or to the one route group whose trust boundary requires it.
- A legacy package must not be imported from cmd/api. Reintroducing sessions, CSRF, assertion headers, shadow users, or plan headers into v1 requires a new ADR and explicit roadmap change.
