# Schema-first testing strategy

`task verify` is the sole local and CI release contract. It starts an isolated PostgreSQL 17.11 database in a Compose project named uniquely for the run on an ephemeral loopback port, applies every migration from zero, proves the latest migration can roll down and back up, and always removes exactly that project when verification exits. The [disposable database lifecycle](#disposable-database-lifecycle) below defines the preflight, exit-status and recovery contract.

The command then runs these gates in order:

1. OpenAPI lint and semantic contract tests.
2. Generated-code and Go module drift checks.
3. Go vet and pinned golangci-lint analysis.
4. The complete race-enabled Go suite, including real PostgreSQL repository and migration behavior.
5. A no-regression statement-coverage gate for the supported form model, submission policies, schema validation, service-token middleware, HTTP API, and runtime database/logging packages.
6. Reachable-vulnerability analysis with pinned govulncheck.
7. Built-image packaging checks for API, default and maintenance targets (`task packaging`), including on pull requests. See [container packaging](container-packaging.md).

## Disposable database lifecycle

`goforms/docker/verify/with-postgres.sh` owns the database for `task verify`. Task's `defer:` is not used because Task ignores a deferred command's exit status and only mentions the failure under `--verbose`, so a leaked container or volume would have passed CI silently. The wrapper's contract is covered by `goforms/internal/tools/verifylifecycle`, which runs the real script against a recording Docker CLI double in the ordinary race-enabled suite:

- **Preflight.** `docker compose version` must succeed before anything starts. A missing Compose v2 plugin fails with an explicit message and touches nothing; the packaging gate keeps its separate Buildx preflight.
- **Scope.** Every run uses a fresh project name `goformx-verify-<epoch>-<pid>-<random>`, printed at the start of the run and exported as `GOFORMX_VERIFY_PROJECT`. Every `docker compose` call carries `-p <that name> -f goforms/docker-compose.verify.yml`, so setup and cleanup can only reach this run's containers, network and volumes. `GOFORMX_VERIFY_PROJECT` may be set explicitly, but it must keep the `goformx-verify-` prefix and Compose's name character set; anything else is rejected before Docker is invoked. A project that already has containers is refused rather than reused, so a successful run always starts from an empty database and applies migrations from version zero.
- **Port.** PostgreSQL publishes to an ephemeral `127.0.0.1` port that the wrapper discovers and passes to the gates through `GOFORMX_TEST_DATABASE_URL`. Parallel runs in different worktrees no longer collide on a fixed port.
- **Cleanup.** `docker compose -p <name> -f ... down --volumes --remove-orphans` runs after setup that only partially succeeded, after normal completion, after a failed gate, and after `SIGINT`/`SIGTERM` (the gate command is stopped first). It never prunes images, volumes or projects outside that name.
- **Exit status.** When a gate fails with status `N` and cleanup also fails, the run exits `N` and prints the cleanup failure with its own status. When the gates pass and cleanup fails with status `M`, the run exits `M`. Interrupted runs exit 130 or 143 after cleanup. `task verify:gates` exists only as the wrapper's target; it refuses to run without `GOFORMX_TEST_DATABASE_URL`.

### Recovery after a leaked run

Cleanup failures print `CLEANUP FAILED` together with the copy/paste command below, keyed to the project name printed at the start of the run. Substitute that name and run it from the repository root:

```sh
docker compose -p goformx-verify-<id> -f goforms/docker-compose.verify.yml down --volumes --remove-orphans
```

To find verification projects that were left behind, list only the scoped names:

```sh
docker compose ls -a --filter name=goformx-verify
```

Remove each one with the command above. Never run `docker system prune`, `docker volume prune`, or a `down` without `-p goformx-verify-<id>`; those reach development databases and unrelated projects on the same daemon. If Task force-kills the wrapper during its interrupt grace period before `down` completes, the same targeted command finishes the cleanup.

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
