# Schema-first testing strategy

`task verify` is the sole local and CI release contract. It starts an isolated PostgreSQL 17.11 database on loopback port 55432, applies every migration from zero, proves the latest migration can roll down and back up, and always removes the disposable database when verification exits.

The command then runs these gates in order:

1. OpenAPI lint and semantic contract tests.
2. Generated-code and Go module drift checks.
3. Go vet and pinned golangci-lint analysis.
4. The complete race-enabled Go suite, including real PostgreSQL repository and migration behavior.
5. A no-regression statement-coverage gate for the supported form model, submission policies, schema validation, service-token middleware, HTTP API, and runtime database/logging packages.
6. Reachable-vulnerability analysis with pinned govulncheck.
7. Built-image packaging checks for API, default and maintenance targets (`task packaging`), including on pull requests. See [container packaging](container-packaging.md).

## Behavioral coverage floors

The initial combined floor is 37%, matching the measured schema-first baseline when the gate was introduced. Package baselines are recorded so new work targets weak behavior rather than inflating coverage through trivial assertions.

| Package boundary | Baseline | Behaviors that must remain covered |
| --- | ---: | --- |
| `domain/form/model` | 41.8% | lifecycle transitions, immutable versions, exact-version validation, idempotent submission state |
| `domain/submission` | Added with #122 | bounded selectors, explicit fail-closed redaction, detached projections, exact numeric values |
| `application/validation` | 34.4% | canonical dialect, schema compilation, field pointers, invalid payloads |
| `application/middleware/serviceauth` | 95.8% | missing, invalid, expired, revoked, under-scoped, and cross-owner tokens |
| `application/handlers/web` | 34.6% | v1 control/data-plane contract, CORS, validation, replay, and error envelopes |
| `infrastructure/database` | Added with #149 | typed private ORM telemetry, real driver-error canaries, log levels and slow-query behavior |
| `infrastructure/logging` | Added with #150 | stateless field sanitization, concurrent derived runtime loggers, stable secret-field masking |

The combined gate must not be lowered to land a change. Raise it as legacy handlers leave the supported package or new schema-first behavior increases exercised statements. PostgreSQL repository behavior is separately mandatory through the disposable-database tests; it is not diluted into the unit-package percentage.

Tests assert observable behavior. Retries do not mask flakes, and any future quarantine must name an owner, reason, and removal date.

## Management authorization contract

`TestManagementScopeContract` reads the canonical OpenAPI document and exercises
the actual registered HTTP routes. The management route inventory must match in
both directions; a new operation also needs an explicit successful request fixture.
For every operation, the matrix checks:

- A service token and a cryptographically verified first-party assertion, each
  carrying only the declared required scope, successfully execute the operation.
- Every other individual scope, and all other scopes combined, return 403.
- An anonymous request returns 401.
- Denied requests never dispatch a business handler or access any form, webhook,
  or token-management repository. Successful calls have exact organization-bound
  repository expectations and the operation's expected success status.

This is route/scope conformance coverage, not a replacement for real PostgreSQL
tenant-isolation, atomic assertion replay, credential lifecycle, or cross-service
browser-to-control-plane integration tests. It runs in the regular race-enabled
suite through `task verify`, without a separate opt-in gate.

## Published client contract

`npm run contract:check` regenerates the bundled OpenAPI, companion schemas, and
TypeScript types, rejects drift from committed artifacts, and compiles the
published examples against those generated types. It also runs the webhook
receiver example against signature fixtures that the production Go signer verifies,
including tampering, timestamp, retry, delivery-ID, wrong-key and rotation cases.
Inline OpenAPI examples are
validated against their declared schemas by the Go contract suite.

`TestPublishedClientCompletesManagementFlow` runs the compiled example through
real HTTP handlers and PostgreSQL, using a disposable organization and a
least-privilege service token. It covers create/version/publish, conditional
metadata update, unauthenticated public schema discovery, invalid submission,
idempotent retry, and submission list/detail provenance. The test verifies the
stored owner and single accepted row and rejects credentials on public requests.
The current development client also downloads JSON and CSV exports as text and
verifies that both export IDs correspond to durable organization/form/credential
audit records. Reading JSON as text avoids introducing JavaScript numeric
rounding into the downloadable artifact. This tests the candidate client; it does
not publish a new contract release or change the stable release's example.
Node and the compiled example are required when PostgreSQL integration is enabled;
the test does not silently skip a missing client build in canonical verification.
