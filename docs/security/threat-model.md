# GoFormX schema-first threat model

## Scope and production data flow

This model covers the supported `goforms/cmd/api` runtime, PostgreSQL persistence, both management credential classes, service-token operator CLI, and CI/release path. Retired human-first and renderer-specific runtimes were removed under issue #83 and are not part of the dependency or scan graph.

```text
Waaseyaa control plane -- signed single-use assertion --> Go API -- parameterized transaction --> PostgreSQL
server integration -- scoped service token --> Go API -- parameterized transaction --> PostgreSQL
anonymous browser/HTTP caller -- public form key --> Go API -- immutable schema validation --> PostgreSQL
operator -- database credential --> token CLI -- hash + lifecycle metadata --> PostgreSQL
operator -- vault keyring and database credential --> webhook key CLI -- atomic ciphertext rotation --> PostgreSQL
organization principal -- tokens scopes --> token management API -- hash + metadata only --> PostgreSQL
GitHub Actions -- pinned build/test/release steps --> attested multi-architecture image
```

The public form key is intentionally embedded in a website and is not authentication. CORS restricts cooperating browsers to an owner's configured origins; it does not stop direct HTTP clients. Management callers cross a different boundary. An external integration presents a hashed, expiring, revocable service token. Waaseyaa presents a 60-second EdDSA assertion only after resolving the signed-in user's active organization membership. Both resolve to an owner-scoped internal principal, but each has an independent verifier and failure in one class never falls back to the other.

## Assets and security objectives

The protected assets are submission payloads; immutable published schemas; service-token hashes, assertion signing keys and replay state, scopes and credential lifecycle metadata; webhook secrets and delivery state; and the finite CPU, memory, database, and backup capacity of the production Pi.

GoFormX must:

- prevent a token from crossing an owner boundary and make another owner's resources indistinguishable from absent resources;
- verify assertion issuer, audience, algorithm, type, key state, lifetime, organization, scope, and one-time identifier before dispatch;
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
| External integration to management plane | Holds a scoped token for one owner | Hash-only token verification, expiry/revocation, route scope, uniform owner-scoped absence | Cross-tenant metadata or payload access |
| Waaseyaa to management plane | Can mint assertions for resolved memberships and holds the signing key | Fixed issuer/audience/profile, configured JWKS, short lifetime, key state, one-use replay row, route scope, uniform owner-scoped absence | Forged identity, assertion replay, or cross-tenant access |
| Owner schema to validator | Controls an authenticated schema definition | Exact dialect, local-only references, depth/node/pattern budgets, bounded compiled-schema cache | SSRF-like network access or validation resource exhaustion |
| API to PostgreSQL | Controls application queries and transaction order | Parameters, foreign keys, uniqueness, row/advisory locks, immutable-schema trigger | Corruption, duplicate delivery, quota race, mutable history |
| Operator to token CLI | Holds database credentials and terminal access | One-time plaintext output, cryptographic random token, hash-only storage, atomic rotation | Control-plane credential disclosure |
| Operator to webhook key CLI | Holds database credentials and all required vault key versions | Environment-only key input, bounded keyring parser, authenticated key-ID/form binding, table write locks, one all-or-nothing transaction, fixed diagnostics/count-only output | Lost delivery configuration or webhook secret disclosure |
| Organization principal to token management API | Holds `tokens:read` or `tokens:write` plus a bounded delegable scope set | Owner-scoped repository queries, delegation subset check, one-time no-store reveal, hash-free metadata reads, idempotent revocation | Privilege amplification or cross-tenant credential control |
| Token mutation to durable audit | Authenticated API actor or privileged database operator | Required explicit actor and matching organization, same-transaction append, no plaintext/hash/name fields, immutable retained history | Unattributed credential creation/revocation or lost forensic evidence |
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
| Webhook storage-key rotation requires maintenance and retained backup keys | The operator CLI rotates endpoints and all retained snapshots atomically; it does not stop old processes, manage the vault, or make an old database backup decryptable with a new key | GoFormX operator; follow `docs/webhooks.md`, stop all writers, verify with active-only keys, and prove production-sized full backup/vault restore before cutover (#125) |
| Plaintext service token is printed once by the privileged CLI | Accepted operator boundary; terminal capture remains sensitive | Operator; rotate immediately after suspected capture |
| Waaseyaa assertion signing key is an online control-plane secret | Public keys rotate with a 65-second overlap; compromise requires immediate revocation and service-token fallback containment | Control-plane operator; follow ADR 0002 emergency rotation and audit all affected request IDs |

## Severity calibration

- Critical: unauthenticated compromise of all tenant submissions, production database credentials, or release signing authority.
- High: practical cross-tenant payload access, arbitrary internal network access from an anonymous route, or durable integrity loss without operator recovery.
- Medium: shared-service resource exhaustion, tenant-planted validation amplification, or constrained internal-network access requiring a tenant token.
- Low: same-tenant availability abuse, high-entropy existence oracles, or metadata-only exposure with restrictive prerequisites.

The earlier schema-first gate review found and fixed two medium and two low issues; the earlier webhook diff review found and fixed one additional low integrity issue. Those are historical review results, not an assurance that later code has no high or critical findings.

## Credential-mutation audit boundary (#123)

Credential-mutation auditing is documented separately in `docs/management-audit.md`.
`internal/domain/auth/audit_actor.go` and
`internal/infrastructure/repository/managementaudit/store.go` (under `goforms/`)
define the caller/transaction boundary. The CLI attributes the authenticated DB
role rather than trusting an operator-supplied human name. Audit-write failures
abort token mutations; tests inject PostgreSQL failures through API and CLI paths.
Direct SQL by the database owner can bypass these application writes or alter
append-only triggers; restrict that authority and retain independent backups.
Webhook mutation auditing and audit browsing remain #123 work, not controls
provided by this token-only slice.

## Storage-key rotation boundary (#113)

The existing single-instance/operator-owned deployment assumptions remain unchanged. The new entry point is a privileged maintenance CLI, not a tenant API. Source anchors are `cmd/goformx-webhook-keys/main.go` (environment input and secret-free error boundary), `internal/domain/webhook/keyring.go` and `cipher.go` (explicit keys, authenticated envelope/form binding), and `internal/infrastructure/webhookrotation/rotation.go` (fixed table set, locks, bounded reads, transaction). Paths are relative to `goforms/`.

- An attacker who can alter stored ciphertext but lacks encryption keys cannot relabel its key ID, move it to a different form, or forge the GCM tag. A bad row aborts the complete rotation; it is never silently skipped. Residual risk: that attacker can still destroy data or deny rotation, so matching backups remain necessary.
- Operator error or process interruption can otherwise leave mixed keys and strand replayable dead letters. One transaction covers both tables and all statuses; no batch commits occur. Unknown keys and wrong-key authentication fail closed. Unit tests and real PostgreSQL tests cover header/form tampering, a failure after earlier batches, cancellation/reconnection, idempotent reruns and reverse rotation.
- A stale API/worker can resume with an old key after locks release, or retain a decrypted in-flight delivery. SQL locks do not prevent either behavior outside the transaction. All processes must stop before rotation and resume with the selected keyring. This is an operator-controlled availability/integrity risk, not an anonymous rotation endpoint.
- Discarding old vault keys can render retained backups unusable even after live storage verifies successfully. PostgreSQL logical backup/restore tests prove that a pre-rotation snapshot still requires its matching key; they do not prove the production backup provider or vault restore. That operational evidence remains a #125 release gate.
- Keys and decrypted configuration exist briefly in privileged process memory. No plaintext is logged or returned; driver errors are replaced with fixed messages to avoid DSN/diagnostic disclosure. Host compromise, environment capture, core dumps and database statement logging remain outside the cryptographic boundary and must be restricted by deployment policy.
