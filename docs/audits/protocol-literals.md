# Protocol literal audit

This audit classifies stable strings used by the supported schema-first runtime. It deliberately does not create a global constants package: each immutable value belongs to the narrowest package that defines its meaning.

## Immutable v1 protocol values

| Value family | Owning definition | Intentionally literal elsewhere |
| --- | --- | --- |
| JSON Schema Draft 2020-12 dialect | `model.JSONSchemaDraft202012URI` | Canonical schema documents, OpenAPI examples, and black-box contract fixtures |
| Public form-key prefix (`gfpk_`) | `model.PublicKeyPrefix` | Black-box fixtures that verify the wire contract |
| Service-token prefix (`gfst_`) and scopes | `auth.ServiceTokenPrefix` and typed `auth.Scope` values | CLI examples and contract fixtures |
| v1 control/data-plane route roots | `constants.PathV1Forms` and `constants.PathV1PublicForms` | OpenAPI paths and black-box HTTP fixtures |
| GoFormX schema/idempotency headers and schema media type | `constants.Header*` and `constants.ContentTypeJSONSchema` | OpenAPI declarations and black-box fixtures |
| Form lifecycle, schema-version, and submission states | Typed values in `domain/form/model` | Migration constraints/defaults and persistence integration fixtures |

The former `PathAPIV1`/`PathAPIv1` alias pair was reconciled to `PathAPIv1`. Production v1 route registration and `Location` generation now use the owning route constants.

## Deployment policy: configuration, not constants

Allowed origins, hosts, external URLs, database connection details, credentials, rate limits, request/body limits, timeouts, log levels, and environment-specific paths are operator policy. They must remain configuration with validation and safe defaults. A protocol constant must not make one of these values impossible to vary between environments.

## Persistence values

Form lifecycle, schema-version state, and submission state are typed domain strings so the database and JSON wire formats remain readable while production Go code cannot silently invent states. SQL migrations repeat these values intentionally because a migration is an independently executable persistence contract.

## Independent contracts and fixtures

OpenAPI, canonical JSON Schema files, example payloads, migrations, and black-box tests should retain important wire literals. Importing production constants into these artifacts would make drift tests self-fulfilling: changing the implementation would also change the purported independent expectation.

## Values reviewed and consciously retained

- Standard HTTP methods, headers, status codes, media types, and RFC3339 formatting use Go/Echo standard-library definitions where available.
- Stable error codes are currently local to the v1 HTTP boundary. They should become typed only when another production package must branch on them; sharing them with black-box tests is not a reason.
- Legacy `/api` paths, browser sessions, CSRF names, and legacy roles remain isolated debt scheduled for removal under issues #73, #74, and #83. Consolidating those literals would invest in an unsupported runtime.
- Human-readable log and error messages are not protocol identifiers unless the OpenAPI contract explicitly promotes them to stable codes.

## Regression policy

Compiler ownership, typed domain values, contract linting, semantic contract tests, and PostgreSQL integration tests are the regression checks. No source-text literal counter is added: it would flag deliberate contract fixtures and reward indirection rather than correctness.
