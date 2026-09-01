# Durable signed webhooks

GoFormX webhooks are an optional part of the supported schema-first runtime. When enabled, accepting a submission and creating its delivery record occur in one PostgreSQL transaction. The event stores an encrypted snapshot of the destination configuration, so later endpoint changes cannot alter an already-accepted delivery.

## Enable the worker

Set `WEBHOOK_ENABLED=true` and configure encryption through the deployment vault. Existing installations may retain `WEBHOOK_ENCRYPTION_KEY` (exactly 32 random bytes, base64 encoded). New installations should use `WEBHOOK_ACTIVE_ENCRYPTION_KEY_ID` and `WEBHOOK_ENCRYPTION_KEYRING`, a JSON object mapping non-secret key IDs to base64-encoded 32-byte keys. Generate each key once with `openssl rand -base64 32`; do not paste real keys into commands, tickets, logs, or this document. Losing a required key makes encrypted configuration unrecoverable. Follow the maintenance procedure below to rotate existing data.

The API process owns a durable dispatcher loop. Pending records survive process or network failure. A stale processing lease is reclaimed after WEBHOOK_LOCK_TIMEOUT; delivery attempts stop at WEBHOOK_MAX_ATTEMPTS and enter dead_letter.

## Configure and observe

PUT /v1/forms/{formId}/webhook requires webhooks:write and accepts:

    {
      "url": "https://example.com/goformx",
      "headers": {"Authorization": "Bearer write-only-value"},
      "signingSecret": "at-least-32-characters-and-kept-by-the-receiver",
      "enabled": true
    }

The URL must use HTTPS port 443 and cannot contain credentials, a query, or a fragment. Every resolved address must be public both when the endpoint is configured and when a connection is opened. Proxies and redirects are disabled. Loopback, private, link-local, carrier-grade NAT, multicast, documentation, benchmarking, and reserved ranges are rejected.

The complete destination URL, headers, and signing secret are encrypted together with AES-256-GCM and bound to the form ID as authenticated data. Only the destination origin is returned or stored as plaintext; a potentially secret URL path never appears in API responses or delivery logs. Full replacement with PUT requires supplying the complete desired secret configuration again; PATCH supports secret-preserving lifecycle changes as described below.

Use GET /v1/forms/{formId}/deliveries?limit=25 with submissions:read to inspect recent status, attempt count, next attempt, HTTP status, and non-sensitive error category. Use POST /v1/forms/{formId}/deliveries/{deliveryId}/replay with webhooks:write to requeue a dead-letter delivery. Replay keeps the same delivery ID and event payload so receivers can remain idempotent.

## Verify a request

Each request includes:

- X-GoFormX-Delivery-ID: stable across every attempt and manual replay;
- X-GoFormX-Timestamp: Unix seconds for this attempt;
- X-GoFormX-Signature: v1= followed by the lowercase HMAC-SHA256 digest.

The signed bytes are:

    delivery_id + "." + timestamp + "." + exact_request_body

The receiver must read the raw body before JSON parsing, reject timestamps outside a five-minute window, calculate the HMAC using the delivery ID and timestamp headers exactly as shown above, compare signatures in constant time, and confirm the delivery ID matches the event ID in the body. Record the delivery ID in the same transaction that applies the side effect. A repeated delivery ID should return a successful 2xx response without repeating the side effect.

Use the compiled and behavior-tested TypeScript reference in
[`webhook-receiver.mts`](../goforms/contracts/examples/webhook-receiver.mts). It
rejects missing headers, unsupported signature versions, non-canonical timestamps,
stale or future attempts, wrong keys, changed raw bytes, invalid JSON, and a body
whose event ID differs from the delivery header. The same committed fixtures are
checked by the production Go signer and the TypeScript receiver during
`task verify`; copying only the prose or reconstructing JSON is unsupported.
Apply a request-body limit before buffering the raw bytes, and return one generic
failure response rather than exposing the verifier's diagnostic code to callers.
Return a permanent 4xx for a request that fails verification. Return a retryable
5xx when receiver-owned infrastructure fails instead: no signing secret is
available, the replay store is down, or the side-effect transaction cannot commit.
The reference verifier reports an empty key set as `secret_unavailable` so the
handler can return 503 rather than permanently dead-lettering a valid delivery.

Replay protection is a receiver-side business transaction, not another signature
check. Store `X-GoFormX-Delivery-ID` under a unique constraint in the same database
transaction that applies the event's side effect. If the insert conflicts, commit
nothing new and return 2xx. Do not insert a permanent "processed" marker in one
transaction and perform the effect later: a crash between them would acknowledge a
delivery whose effect never happened. A changed timestamp on a retry is expected;
the stable delivery ID, not the signature or timestamp, is the idempotency key.

Any 2xx response completes delivery. Network errors, 408, 429, and 5xx responses retry with capped exponential backoff. Other 4xx responses and configuration/authentication failures enter the dead letter immediately. Response bodies are discarded and never logged.

## Rotate storage encryption keys (maintenance only)

This rotates **encryption at rest**, not the receiver's signing secret. It leaves destination, headers, signing secret, delivery IDs, attempt counters, leases, schedules, and statuses unchanged. PostgreSQL update triggers may advance `updated_at`. Neither public submission nor management API contracts change.

The versioned ciphertext is `goformx.webhook.v1:<key-id>:` followed by a 12-byte random nonce and AES-256-GCM ciphertext/tag. The entire header, a zero separator, and the form ID are authenticated data. IDs allow 1–64 ASCII letters, digits, `_` or `-`; the keyring supports at most eight keys. A tagged ciphertext selects exactly its declared key. Unknown IDs, wrong keys, modified headers, or wrong forms fail closed; no trial of other keys follows. Original untagged ciphertext is read only with the explicitly supplied `WEBHOOK_ENCRYPTION_KEY`. Legacy-only configuration continues writing the original format for staged binary rollout.

The privileged `goformx-webhook-keys` binary is included under `/app/bin/` in the explicit [maintenance image](container-packaging.md), not the API image; source builds use the ordinary `go build ./...` contract. It accepts only `rotate` or `verify`. Supply `DATABASE_URL` and the three webhook encryption variables through a vault-backed process environment, **not command-line arguments**. The maintenance command reads environment only; it does not load the API's YAML configuration. Do not use shell tracing, dump the environment, or enable database statement/parameter logging. Outputs contain only row counts or fixed, secret-free error messages.

1. Inventory every API/worker replica and record the current binary digest and vault key versions. Take a consistent PostgreSQL backup and retain every key needed by that backup in the vault. Prove restoration in an isolated environment before changing production.
2. Deploy the keyring-capable binary while retaining the original encryption configuration. Do not start writing tagged data until all processes can read it. Schedule a maintenance window: stop **all API and worker processes**, and wait for in-flight requests/deliveries to finish. Disabling only `WEBHOOK_ENABLED` does not stop submission enqueueing and is not maintenance mode.
3. Prepare the new vault keyring with distinct IDs for old and new key material and select the new active ID. During the first migration, retain the original key in `WEBHOOK_ENCRYPTION_KEY` for untagged reads. Configure the same ring for the maintenance command and every process that will resume. Never reuse an existing ID for different key material.
4. Run `/app/bin/goformx-webhook-keys rotate` using the pinned maintenance image in a one-off maintenance container with the vault environment. It takes write-blocking locks on both webhook tables (five-second lock-acquisition timeout), authenticates every row, and rewrites endpoints and **all retained delivery snapshots**, including disabled endpoints, processing, delivered and replayable dead-letter rows. Reads use batches of 100, but **one transaction covers the entire operation**. There is no partial batch commit or secret-bearing checkpoint. Allow disk/WAL headroom and measure maintenance duration on a production-sized restored database before production use.
5. Run `/app/bin/goformx-webhook-keys verify` with **only the new key** and no legacy key. Success proves every retained ciphertext authenticates under the selected active ID. Verification also locks out writes; it does not rewrite rows. A success response is bounded JSON with endpoint/delivery counts and `reencrypted: 0`.
6. Resume every API/worker using the verified new keyring, check readiness and perform a synthetic signed delivery/retry. An old process can write old ciphertext after locks are released, so verification is not permission to leave an old writer running. Remove old keys from the online environment only after all writers are confirmed updated. Keep old key versions in the vault for as long as any retained backup requires them.

### Failure, interruption and rollback

- Authentication failure, failed update, signal cancellation or disconnect before commit leaves the transaction uncommitted; PostgreSQL rolls it back. Fix configuration or recover damaged ciphertext from the matching backup, then rerun. Never skip an unreadable row. If a connection disappears, wait for its PostgreSQL session/locks to clear before retrying.
- A commit acknowledgement or output failure is ambiguous from the operator's perspective. Do not assume rollback. Run `verify` with the intended active keyring while still in maintenance. Rerunning `rotate` is safe: it authenticates already-current rows and leaves their ciphertext unchanged.
- To reverse a completed rotation, keep the keyring-capable binary, configure both keys with the **old ID active**, run `rotate`, and verify using only the old tagged key. This restores old-key encryption, not the legacy binary format. Do not downgrade to a pre-keyring binary against tagged ciphertext.
- To restore a pre-keyring backup, restore its matching legacy vault key and compatible binary/configuration, then rerun migration if desired. A database backup alone is insufficient. Tests exercise PostgreSQL logical ciphertext backup/restore, reverse rotation, interruption and wrong-key rollback; infrastructure-level full backup/vault restoration remains a production release gate.

The operation intentionally has no online/HTTP rotation endpoint. Maintenance credentials grant database-wide access and belong only to the operator, never a tenant or the control-plane browser.

## Dashboard-safe lifecycle changes

`PATCH /v1/forms/{formId}/webhook` requires `webhooks:write` and accepts exactly
one field: `{"enabled":false}` to pause, `{"enabled":true}` to resume, or
`{"signingSecret":"<new 32–256 character secret>"}` to rotate the receiver signing
secret. Nulls, empty updates and combined changes are rejected. The destination,
custom headers and omitted secret fields remain unchanged; responses contain
metadata only. Full destination/header replacement continues to use PUT with
the complete write-only configuration. Both management credential classes use
the same organization boundary. Do not put a management credential in a browser.

**Pause means stop future enqueueing**, not cancel accepted deliveries. Already
accepted snapshots remain dispatchable after pause, rotation or deletion; a
dead-letter replay uses its original delivery ID, payload and signing secret.
Resuming does not backfill submissions accepted while paused. The dashboard must
make these semantics visible rather than claiming that pause stops all traffic.

Install the new signing secret at the receiver before rotating GoFormX, and
accept both old and new keys during the overlap. Retain the old receiver key
until outstanding deliveries and intentionally retained dead letters no longer
need replay. Do not retire it merely because new submissions use the new key.
This is distinct from storage encryption-key rotation (#113), which does not
change the receiver's signing secret. No secret-readback endpoint exists.

Pass current then previous signing secrets to the reference verifier during that
bounded overlap. Historical accepted deliveries keep their original encrypted
signing-secret snapshot, so a manual replay after rotation still verifies only
with the old receiver key. Removing the previous key is therefore also a decision
to abandon any retained delivery that still depends on it; verify delivery and
dead-letter retention before removal.

Every successful configuration change and replay has an atomic, secret-free
[management audit](management-audit.md). Audit failure means no change was
committed. Other network/commit failures can be ambiguous; inspect metadata and
delivery status before retrying. A repeated pause/resume value is a no-op, while
replay only succeeds for a currently dead-letter delivery.
