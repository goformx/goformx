# Schema-first testing strategy

`task verify` is the sole local and CI release contract. It starts an isolated PostgreSQL 17.11 database on loopback port 55432, applies every migration from zero, proves the latest migration can roll down and back up, and always removes the disposable database when verification exits.

The command then runs these gates in order:

1. OpenAPI lint and semantic contract tests.
2. Generated-code and Go module drift checks.
3. Go vet and pinned golangci-lint analysis.
4. The complete race-enabled Go suite, including real PostgreSQL repository and migration behavior.
5. A no-regression statement-coverage gate for the supported form model, schema validation, service-token middleware, and HTTP API packages.
6. Reachable-vulnerability analysis with pinned govulncheck.

## Behavioral coverage floors

The initial combined floor is 37%, matching the measured schema-first baseline when the gate was introduced. Package baselines are recorded so new work targets weak behavior rather than inflating coverage through trivial assertions.

| Package boundary | Baseline | Behaviors that must remain covered |
| --- | ---: | --- |
| `domain/form/model` | 41.8% | lifecycle transitions, immutable versions, exact-version validation, idempotent submission state |
| `application/validation` | 34.4% | canonical dialect, schema compilation, field pointers, invalid payloads |
| `application/middleware/serviceauth` | 95.8% | missing, invalid, expired, revoked, under-scoped, and cross-owner tokens |
| `application/handlers/web` | 34.6% | v1 control/data-plane contract, CORS, validation, replay, and error envelopes |

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
