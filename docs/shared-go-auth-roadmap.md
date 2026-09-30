# Shared Go authentication roadmap

Updated 2026-09-30. Status: proposed extraction, no code moved and no shared module created.

## Purpose and ownership

Share GoFormX's narrowly defined control-plane assertion verification mechanics with **NorthCloud**, the customer news product at `northcloud.one` currently implemented in `jonesrussell/northway`. The older `jonesrussell/north-cloud` collection/procurement stack is not this initial consumer. NorthCloud's [launch roadmap](https://github.com/jonesrussell/northway/blob/main/docs/launch-roadmap.md) owns its Intersnipe deployment and staged Pi retirement; [adoption notes](https://github.com/jonesrussell/northway/blob/main/docs/shared-go-auth.md) own its adapter gates.

This is supporting engineering work, not a new GoFormX product direction or a reason to merge unrelated application PRs. Product domains and authorization remain separately owned. Follow the [GoFormX roadmap](https://github.com/goformx/goformx/issues/84).

## Exact source baseline

Inspection compared GoFormX `b5308f5c2a6df7d86506a4adca21f5b2bfab4e1d` with Northway `377d00696d6597b646e4e48a80ab948a05f3f361`. GoFormX's revision is the recorded September 30 deployed API and open PR #204 head at inspection; main was `fd9cc3f6db9b49e5597b355bfffce09f4c5fb0ab`. Do not claim the deployed source is all on main or merge the application PR as part of this documentation work. The recorded deployed PHP/web reference was `bb384f90cf995e1165cc57e121ebc4347a8913b2`; later control-plane heads are not live deployment evidence.

The comparison inspected source and tests, not fresh execution. Reconcile accepted source history, license/provenance and current heads before extracting. Any relevant donor changes outside main need explicit source disposition, not an unnoticed application merge.

## Candidate inventory

Paths below refer to the exact donor revision, not an assertion about current main.

| Candidate | Source | Decision |
| --- | --- | --- |
| Assertion verification | [goforms/internal/domain/auth/assertion.go](https://github.com/goformx/goformx/blob/b5308f5c2a6df7d86506a4adca21f5b2bfab4e1d/goforms/internal/domain/auth/assertion.go) and assertion_test.go | First extraction: signature, profile, time, operation and replay-verification mechanics |
| JWKS key provider | [goforms/internal/infrastructure/authn/jwks.go](https://github.com/goformx/goformx/blob/b5308f5c2a6df7d86506a4adca21f5b2bfab4e1d/goforms/internal/infrastructure/authn/jwks.go) and jwks_test.go | Include bounded pinned discovery, refresh ordering, rotation states and revocation tombstones |
| Replay persistence | [goforms/internal/infrastructure/repository/assertionreplay/store.go](https://github.com/goformx/goformx/blob/b5308f5c2a6df7d86506a4adca21f5b2bfab4e1d/goforms/internal/infrastructure/repository/assertionreplay/store.go) | Share only contract/conformance fixtures; PostgreSQL adapter stays in GoFormX, SQLite adapter in NorthCloud |
| HTTP principal adapter | [goforms/internal/application/middleware/serviceauth/middleware.go](https://github.com/goformx/goformx/blob/b5308f5c2a6df7d86506a4adca21f5b2bfab4e1d/goforms/internal/application/middleware/serviceauth/middleware.go) | Keep Echo middleware, credential dispatch and product scope/ownership policy local |
| External keys | [goforms/internal/domain/auth/token.go](https://github.com/goformx/goformx/blob/b5308f5c2a6df7d86506a4adca21f5b2bfab4e1d/goforms/internal/domain/auth/token.go) versus [Northway identity/keys.go](https://github.com/jonesrussell/northway/blob/377d00696d6597b646e4e48a80ab948a05f3f361/internal/identity/keys.go) | Keep lifecycle, formats, stores and schemas separate initially |
| Strict JSON | [request_json.go](https://github.com/goformx/goformx/blob/b5308f5c2a6df7d86506a4adca21f5b2bfab4e1d/goforms/internal/application/handlers/web/request_json.go) versus [Northway httpapi/json.go](https://github.com/jonesrussell/northway/blob/377d00696d6597b646e4e48a80ab948a05f3f361/internal/httpapi/json.go) | Possible later primitive after consumer tests; not shared handlers or schemas |

## Proposed module contract

Use one small standalone versioned Go module, approximately `assertion` and `jwks` packages plus conformance fixtures. Both products depend on it; it depends on neither. Module/repository name, ownership and licensing remain decisions. No generic platform, Echo/GORM/SQLite dependency, environment reader, global logger or service locator.

The library verifies cryptographic and protocol facts and reports a verified claim set. Each consumer maps that into its own principal, validates its scope registry and enforces resource ownership. Product-specific type, issuer, audience, operation IDs, trusted keys and profile version remain explicit; do not permit arbitrary algorithms or weaken the existing Ed25519 profile to appear generic.

Preserve GoFormX's existing `gofx-fpa+jwt` v2 wire contract, maximum 60-second lifetime, five-second skew, operation-bound verification and single-use replay semantics. Preserve strict claim/header decoding and ID semantics. NorthCloud adopts an explicitly reviewed distinct profile; shared code does not make either product's keys or assertions valid in the other.

Expose narrow verification-key and atomic replay-store interfaces. Durable consume must precede handler dispatch; storage failure denies authentication. Product databases own persistence, expiry cleanup and audit retention. Existing JWKS failed refreshes retain known keys; any bounded-staleness change is a separate reviewed contract decision, not incidental refactoring.

## Differences that must survive extraction

- GoFormX: Echo, PostgreSQL/GORM, Viper and Zap. NorthCloud foundation: net/http, SQLite/sqlc, explicit configuration and slog. No framework unification is required.
- GoFormX `gfst_` keys have expiry, hash-derived lookup IDs, string scopes and rotation/audit metadata. Northway `nw1_` keys have independent lookup IDs, persisted scope bits, no current automatic expiry and conditional revocation checks when recording use. Do not migrate formats or storage in this slice.
- Northway principals have private fields and deny-all zero values and are passed into tenant-scoped storage. GoFormX transports a principal through Echo. Keep adapters, local operator authority and resource checks outside the module.
- GoFormX JSON handling supports operation-selected JSON/merge-patch and a 1 MiB ceiling. Northway uses 32 KiB, depth bounds, strict Unicode/case-sensitive shape checks and different error mapping. Share a parser only after explicit compatibility decisions.
- GoFormX's in-memory per-form submission limiter is not a durable tenant query/polling budget. Error envelopes, retry semantics, retention and request identifiers remain product contracts.
- Forms, submissions, feeds, ranking, sources/rights, billing, database migrations, export/deletion and backup orchestration remain local. Small duplicated wrappers are preferable to premature abstractions.

## Phased delivery and gates

| Stage | Work | Gate |
| --- | --- | --- |
| 1. Freeze donor contract | Reconcile exact accepted source, provenance/license, profile fixtures and failure behavior | No unrelated source landing; record main versus deployed/donor differences and baseline tests |
| 2. Extract mechanics | Introduce minimal module and migrate GoFormX wrappers without wire/schema changes | Existing assertion/JWKS, serviceauth, token and real PostgreSQL replay suites pass; unchanged HTTP/status/ownership behavior |
| 3. Prove second consumer | NorthCloud profile, net/http adapter, SQLite replay store and control-plane signer fixtures | Both consumers pass shared vectors and product HTTP/isolation tests; no GoFormX runtime dependency leaks into NorthCloud |
| 4. Publish/pin | Reviewed immutable semantic-version tag, changelog and upgrade notes | Each consumer pins exact reviewed release in go.mod/go.sum; no floating branch or production local replace; independent upgrade/revert remains possible |
| 5. Evaluate next extraction | Compare remaining duplicate JSON primitives using real consumers | Extract only a demonstrated common contract; otherwise leave local |

Do not publish a module, change credentials, deploy either product or rename repositories as a side effect. For this roadmap work the owner selected local checks/review followed by direct commit/merge and push, with no PRs or dependency on GitHub Actions. Preserve existing workflows and protections; do not bypass a rejected push. Future runtime work still requires meaningful verification and explicit operational scope.

## Consumer-driven conformance and isolation tests

Carry over assertion_test.go, jwks_test.go, serviceauth/middleware_test.go and assertionreplay/store_integration_test.go behavior. Preserve Northway identity, HTTP/SQLite, principal and revocation-race tests. Tests must cover:

1. Signature/profile/type/version, issuer/audience, TTL/skew and scope failures; valid known profile succeeds. Each product rejects the other's credentials, even when both use the same library version.
2. Wrong operation rejected before replay consumption. Scope delegation cannot exceed caller authority; no agent superuser or API-key fallback after assertion rejection.
3. Concurrent consumption accepts exactly once; replay survives restart, uses issuer plus assertion identity and is not removed before its validity/skew window. Run against actual PostgreSQL and file-backed SQLite, including storage failure and cleanup boundaries.
4. Next/active/retiring/revoked key behavior, unknown-key refresh bounds, pinned HTTPS/no redirects, malformed/oversized JWKS, out-of-order refresh and persistent-in-provider revocation tombstones. Document restart/snapshot trust and stale-key limits accurately.
5. Real HTTP routes enforce two-tenant resource isolation and request scopes; NorthCloud also exercises cache, jobs and tenant-scoped SQL/FTS. A verified assertion is not itself proof of object ownership.
6. Consumer-specific principal construction, key formats, one-time secret handling, revocation and safe logs remain unchanged. PHP signer fixtures and published schemas agree with the selected wire profile.

Run race and targeted fuzz/property tests for the shared parser/verifier and concurrency paths. Record executed commands and exact versions; source inspection or passing library mocks alone does not establish production authorization.
