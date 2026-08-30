# GoFormX contributor contract

## Supported direction

GoFormX is being rebuilt as an AI-first, schema-driven Go service. Work toward the roadmap in `goformx/goformx#84`.

- `goforms/` is the only supported runtime.
- JSON Schema Draft 2020-12 is the canonical form definition.
- OpenAPI is the machine-readable HTTP contract.
- PostgreSQL is the supported database.
- Dashboard UI, billing, browser sessions, framework migrations, and renderer-specific schemas are not v1 requirements. Their retired implementations remain available in Git history, not in the working tree.

## Required workflow

Run commands from the repository root:

- `task bootstrap` downloads modules and regenerates committed artifacts.
- `task verify` is the local and CI quality contract.
- `task test`, `task lint`, and `task vuln` run focused gates.

Tool versions must be pinned. Do not introduce `@latest`, hidden global prerequisites, or a second build path. Generated mocks are committed; update them in the same change as their interfaces.

## Architecture boundaries

Keep business rules independent of HTTP, logging, configuration, and persistence. New code should live behind explicit packages for form, schema, submission, auth, Postgres, HTTP API, migrations, and OpenAPI. Prefer explicit constructors and one middleware chain.

Public browsers use unguessable form keys and never receive reusable credentials. External control-plane callers use scoped, revocable service tokens. The Waaseyaa control plane acts for a resolved user and organization through the single-use first-party assertion profile in `docs/adr/0002-first-party-assertions.md`; the two credential classes must not fall back to one another. Submissions must record the immutable schema version used for validation.

## Tests and changes

Tests must assert observable behavior. Logging TODOs or documenting expectations in a passing test is not a test.

- Unit-test lifecycle, validation, authorization, and idempotency rules.
- Exercise repositories and migrations against real PostgreSQL.
- Keep HTTP behavior aligned with OpenAPI contract tests.
- Treat the personal-site contact flow in issue #57 as the release gate.

When a change alters the API, schema rules, migrations, trust boundaries, or operational behavior, update the matching contract or decision document in the same pull request. Preserve unrelated user changes and never deploy or alter DNS from verification commands.

## Code Review Rules

- Flag changes that mix service-token and first-party-assertion authorization or let a resolved user act across organization boundaries. Require evidence from the actual request path, not a constructor signature alone.
- Check API/schema/migration changes against the matching OpenAPI or decision contract and observable tests. Do not describe static inspection as executed verification.
- Review automation must remain manually requested, subscription-only, and unable to modify repository code. Failed, incomplete, or stale reviewer output is not approval; do not weaken limits or add paid fallbacks to make a review pass.
