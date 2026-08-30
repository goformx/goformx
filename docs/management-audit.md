# Credential and webhook mutation audit

GoFormX commits service-token issuance, revocation, operator rotation, and webhook
configuration/replay changes with an
append-only `management_audit` record in the **same PostgreSQL transaction**.
There is no best-effort logging fallback. HTTP audit-write failure returns
`503 management_audit_unavailable` without revealing an issued token or committing
the credential mutation. Other connection/commit failures can have an uncertain
outcome: inspect token metadata before retrying, and revoke any orphaned issuance
whose one-time secret was not received.

## Identity and contents

- First-party calls record the verified subject, organization, assertion ID and
  signed request ID. A browser/body/header cannot supply the actor.
- Service-token calls record the authenticated non-secret lookup ID as subject
  and credential ID, plus the resolved organization and request correlation ID.
- Operator CLI calls record `database_operator` and the connection's authenticated
  `current_user`, represented as `db:` plus base64url-encoded role bytes. This is
  database-role attribution, **not a claim to identify the human using that role**.
  Operators need individual authenticated roles or an external access trail when
  individual human attribution is required. No caller-provided actor flag exists.

The fixed record contains an audit UUID, actor identifiers, organization, event,
target token ID, optional replacement token ID, canonical granted scopes, expiry
when applicable, and event time. It has no arbitrary metadata bag. Token values,
hashes, names, request bodies, headers and payload digests are excluded. Opaque
user/organization identifiers still require access and retention controls.

Events are `service_token.created`, `service_token.revoked`, and
`service_token.rotated`. Rotation records the old target, replacement ID, inherited
scopes and new expiry as one atomic event. Concurrent/repeated revocation of an
already-revoked owned token preserves its original timestamp and does not create
another mutation event. A denied, invalid, or foreign-owned request does not create
a successful-change record. Authentication failures/access attempts belong to a
separate operational security log; last-use updates are not credential mutations.

## Operational use and rollback

Authorized operators can query the table by `organization_id`, `target_id` and
`occurred_at`, and correlate the `request_id` with the response/operational log.
Generated trace IDs now remain stable throughout one request. Request correlation
is not authorization. This slice does **not** expose an audit-list HTTP/UI endpoint.

Webhook records add the form UUID and, for live-endpoint configuration changes,
the resulting `enabled` flag. Target IDs identify endpoints, or deliveries for
replay. Events are `webhook.created`, `webhook.updated` (full replacement),
`webhook.paused`, `webhook.resumed`, `webhook.signing_secret_rotated`,
`webhook.deleted`, and `webhook.delivery_replayed`. They never contain the
destination URL/origin, headers, signing keys, ciphertext, or submission data.
Ownership checks, configuration/replay changes and audit insertion share one
transaction. The owned form is locked before the endpoint to serialize initial
creation and concurrent updates. Repeating an already-current enabled value is a
successful no-op: no timestamp change and no new mutation audit. PUT replacement
and explicit signing-secret rotation each write a new encrypted configuration and
produce a record. Audit failure returns `503 management_audit_unavailable` and
rolls back the entire webhook change. Network/commit uncertainty still requires
inspecting metadata rather than assuming success or failure.

Apply migrations through `2026083004` before this binary (`2026083003` introduced
token audits; `2026083004` adds webhook events and typed fields). Audit records have no
cascading foreign keys, so credential/account cleanup cannot erase history.
Database triggers reject update, delete and truncate. A routine down migration
refuses to drop a populated audit table; the webhook migration refuses to remove
its schema if any webhook history exists. These are application/operator mistake
guards, not tamper-proof protection against a database owner who can alter DDL.

Keep the audited binary for ordinary rollback. A pre-audit binary must not serve
credential mutations after rollback, because it does not implement this boundary.
A token-audit-only binary must likewise not resume webhook mutation traffic after
the webhook audit boundary has been enabled. Pause those routes during an older
binary rollback; retained audit history must not be deleted to permit rollback.
Retain the audit table with consistent backups, restrict direct SQL access, and
establish retention, capacity and restore evidence under #125 before public launch.
No production migration or deployment is implied by merged code.

The operator CLI retains its established invocation interface and is now included
as `/app/bin/goformx-token` in the production image and in the normal backend
binary artifacts. Supply real
`DATABASE_URL` credentials through vault-backed process environment rather than
command-line arguments, shell history or tracing. Database errors are replaced
with fixed diagnostics. The intended one-time successful token output remains
sensitive and must go directly to secret custody.

## Verification

The normal `task verify` suite covers both authenticated HTTP credential classes
against PostgreSQL, failed-audit rollback with no secret reveal, real revocation,
foreign-organization denial, duplicate/concurrent revocation, missing actor
rejection, immutable retained records and populated-down refusal. CLI tests prove
issue/rotate/revoke atomicity and database-role attribution. A retained-red trace
regression demonstrates the former repeated-ID-generation bug; the fixed handler
keeps audit, response and logging correlation aligned. Webhook tests exercise
real HTTP and PostgreSQL for both credentials, missing/mismatched actors,
audit-failure rollback for creation/replacement/pause/rotation/deletion/replay,
concurrent pause, encrypted-field preservation, retained delivery snapshots,
replay after deletion, secret-free records and populated-webhook-down refusal.
