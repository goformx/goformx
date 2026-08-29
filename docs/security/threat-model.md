# GoFormX schema-first threat model

## Scope and production data flow

This model covers the supported `goforms/cmd/api` runtime, PostgreSQL persistence, service-token operator CLI, and CI/release path. Retired human-first and renderer-specific runtimes were removed under issue #83 and are not part of the dependency or scan graph.

```text
control-plane caller -- scoped service token --> Go API -- parameterized transaction --> PostgreSQL
anonymous browser/HTTP caller -- public form key --> Go API -- immutable schema validation --> PostgreSQL
operator -- database credential --> token CLI -- hash + lifecycle metadata --> PostgreSQL
GitHub Actions -- pinned build/test/release steps --> attested multi-architecture image
```

The public form key is intentionally embedded in a website and is not authentication. CORS restricts cooperating browsers to an owner's configured origins; it does not stop direct HTTP clients. Control-plane callers cross a different boundary: a hashed, expiring, revocable service token supplies an owner principal and one explicit scope for each operation.

## Assets and security objectives

The protected assets are submission payloads; immutable published schemas; service-token hashes, scopes, and lifecycle metadata; webhook secrets and delivery state; and the finite CPU, memory, database, and backup capacity of the production Pi.

GoFormX must:

- prevent a token from crossing an owner boundary and make another owner's resources indistinguishable from absent resources;
- validate submissions against the exact immutable schema version recorded with the submission;
- compile only bounded Draft 2020-12 schemas whose references stay inside the submitted document;
- bound anonymous work and durable storage per form, independently of browser-origin behavior;
- store token and webhook secrets only in non-plaintext forms and exclude them and submission payloads from logs;
- persist a submission and its future delivery intent atomically;
- send webhooks only to policy-approved public destinations, with signatures, timestamps, idempotent retries, and observable dead-letter state;
- keep CI dependencies pinned and require tests, contract checks, vulnerability checks, and provenance before release.

## Trust boundaries and controls

| Boundary | Attacker starting capability | Enforced control | Failure impact |
| --- | --- | --- | --- |
| Anonymous caller to public submission API | Knows a public form key and controls headers/body/frequency | 1 MiB body cap, per-form token bucket, rolling daily quota, schema validation, idempotency | CPU, memory, or PostgreSQL exhaustion; invalid stored data |
| Tenant to control plane | Holds a scoped token for one owner | Hash-only token verification, expiry/revocation, route scope, uniform owner-scoped absence | Cross-tenant metadata or payload access |
| Owner schema to validator | Controls an authenticated schema definition | Exact dialect, local-only references, depth/node/pattern budgets, bounded compiled-schema cache | SSRF-like network access or validation resource exhaustion |
| API to PostgreSQL | Controls application queries and transaction order | Parameters, foreign keys, uniqueness, row/advisory locks, immutable-schema trigger | Corruption, duplicate delivery, quota race, mutable history |
| Operator to token CLI | Holds database credentials and terminal access | One-time plaintext output, cryptographic random token, hash-only storage, atomic rotation | Control-plane credential disclosure |
| Worker to webhook destination | Processes trusted configuration and untrusted network state | AES-GCM secret storage, HTTPS-only destination validation, connect-time public-IP checks, disabled proxy/redirects, HMAC signature and timestamp | SSRF, secret disclosure, forged or replayed delivery |
| CI to release registry | Can run repository workflows | Pinned Actions, least-privilege jobs, CodeQL, dependency review, tests, vulnerability scan, attestation | Supply-chain compromise |

## Attacker stories and disposition

The security review validated four pre-remediation paths:

1. A tenant could publish a structurally expensive schema and repeatedly trigger fresh compilation through the anonymous endpoint.
2. Any caller with a public key could create unlimited durable rows with unique idempotency keys.
3. A same-owner `submissions:read` caller could request an entire form history in one response.
4. An authenticated tenant could distinguish another owner's existing form from a nonexistent UUID by comparing `403` and `404` responses.

The security-gate change closes these paths with local-reference and complexity policies, a bounded compiled-schema cache, per-form burst and transactional rolling quotas, cursor pagination with a hard page maximum, and uniform `404` owner isolation. Their tests are part of the ordinary `task verify` CI contract.

## Residual risks and ownership

| Residual risk or external control | Current decision | Owner / removal condition |
| --- | --- | --- |
| In-memory burst limits are process-local | Accepted for the documented single-instance Pi deployment; the PostgreSQL rolling quota remains authoritative | GoFormX maintainer; replace with shared admission before adding a second API replica in `goformx/goformx#110` |
| Public submission cannot prove a human authored the payload | Accepted for v1; budgets bound resource use without making CAPTCHA a protocol dependency | Product owner; define the evidence-based trigger in `goformx/goformx#111` |
| Backup confidentiality and restore access are infrastructure controls | Must be proven before production cutover | `jonesrussell/waaseyaa-infra#62` and `goformx/goformx#80` |
| Webhook encryption-key rotation is not automated | The stable vault-backed key is part of backup/restore; queued delivery snapshots depend on it | GoFormX maintainer; add re-encryption in `goformx/goformx#113` before routine rotation is required |
| Plaintext service token is printed once by the privileged CLI | Accepted operator boundary; terminal capture remains sensitive | Operator; rotate immediately after suspected capture |

## Severity calibration

- Critical: unauthenticated compromise of all tenant submissions, production database credentials, or release signing authority.
- High: practical cross-tenant payload access, arbitrary internal network access from an anonymous route, or durable integrity loss without operator recovery.
- Medium: shared-service resource exhaustion, tenant-planted validation amplification, or constrained internal-network access requiring a tenant token.
- Low: same-tenant availability abuse, high-entropy existence oracles, or metadata-only exposure with restrictive prerequisites.

The schema-first gate review found and fixed two medium and two low issues. The webhook diff review found and fixed one additional low integrity issue. No high or critical findings remain.
