# PostgreSQL permission contract

Application contract for [#159](https://github.com/goformx/goformx/issues/159).
This is not a deployment or proof of current production permissions.
Infrastructure owns provisioning, credential distribution, existing-volume
conversion and restore integration in its private issue #75. PHP never connects
to the Go database; it uses the authenticated management API.

## Principals and ownership

Use distinct identities, not one username/password selected by different images:

| Principal | Authority | Distribution |
| --- | --- | --- |
| Bootstrap administrator | Create database/roles and explicitly approved ownership conversion | Provisioning only; never API, worker, operator or backup environment |
| Object owner | NOLOGIN; owns database, application schema objects, functions and migration metadata | No password; no application membership |
| Migrator / restore operator | LOGIN, NOINHERIT; CONNECT plus owner membership with SET TRUE, INHERIT FALSE | Protected, short-lived maintenance path; explicitly SET ROLE owner before DDL/restore |
| Runtime | API + in-process worker DML below; no ownership or memberships | Only runtime credential |
| Token operator | Issue/rotate/revoke tokens and append their audit records | Token CLI only; separate from runtime and key operator |
| Storage-key operator | Read webhook ciphertexts, update ciphertext columns, acquire maintenance locks | Offline rotation CLI plus keyring; stop all API/workers first |
| Backup reader | SELECT existing tables and ACCESS SHARE locks for consistent dumps | Backup job only; not restore/owner credentials |

Every non-bootstrap role is NOSUPERUSER, NOCREATEDB, NOCREATEROLE,
NOREPLICATION and NOBYPASSRLS. Runtime, both operators and backup have **no role
memberships**, ownership or grant options. NOINHERIT alone is insufficient:
SET ROLE membership could still confer owner authority. The migrator's only
intentional membership is owner. Do not grant predefined privileged roles such
as pg_write_server_files, pg_execute_server_program or pg_read_all_data.

## SQL inventory

`goforms/deploy/postgresql/permissions.sql` is the executable reference ACL
profile for PostgreSQL 17, tested against every current up migration. S/I/U/D
mean SELECT/INSERT/UPDATE/DELETE. No runtime DELETE is implicit in table ownership.

| Table | Runtime | Other operational access | Source |
| --- | --- | --- | --- |
| forms | S/I/U (GORM updates the model) | Backup S | repository/form/store.go |
| form_schemas | S/I; U only state, published_at | Backup S | repository/form/store.go |
| form_submissions | S/I; no U/D | Backup S | repository/form/store.go, submission_export.go |
| service_tokens | S/I; U last_used_at, revoked_at, revocation_reason | Token operator S/I; U revoked_at, revocation_reason, replaced_by_token_id; backup S | repository/token/store.go; cmd/goformx-token/main.go |
| first_party_assertion_replays | S/I/D; no U | Backup S | repository/assertionreplay/store.go |
| management_audit | S/I only | Token operator S/I; backup S | repository/managementaudit; token/form repositories; token CLI |
| submission_export_audit | S/I only | Backup S | repository/form/submission_export.go |
| webhook_endpoints | S/I/D; U destination_origin, encrypted_config, enabled, updated_at | Key operator S, MAINTAIN, U encrypted_config; backup S | repository/form/webhook.go; webhookrotation/rotation.go |
| webhook_deliveries | S/I; U status, attempt_count, next_attempt_at, locked_at, updated_at, delivered_at, last_http_status, last_error_category | Key operator S, MAINTAIN, U encrypted_config; backup S | repository/form/webhook.go; webhookrotation/rotation.go |
| users (legacy), schema_migrations | None | Backup S; owner/migrator maintenance | Existing migrations; migration tool metadata |

Source paths above are relative to `goforms/internal/infrastructure/` except
`cmd/`, which is relative to `goforms/`. All identifiers are application-generated
UUID/text; current migrations create no sequences. A new sequence/table/function
requires an explicit permission-inventory and regression update, not automatic
runtime access.

### Locks and functions

- Form publication/versioning, token revocation, endpoint edits and delivery
  claiming use SELECT FOR UPDATE (including SKIP LOCKED) with their DML grants.
- Submission admission uses pg_catalog.pg_advisory_xact_lock/hashtextextended;
  retain the normal built-in function access. These locks are not row ownership.
- Trigger functions are owner-created, SECURITY INVOKER, with no runtime DDL or
  direct EXECUTE grant. Existing append-only and published-schema triggers remain
  in force. Runtime cannot disable them or set session_replication_role=replica.
- Offline key rotation uses SHARE ROW EXCLUSIVE locks. PostgreSQL 17's MAINTAIN
  privilege permits these locks while ciphertext-column UPDATE avoids granting
  arbitrary delivery metadata updates. MAINTAIN also permits maintenance work
  such as VACUUM/REINDEX: this trusted operator can affect availability and is not
  a ciphertext-only sandbox. It has no submission/token/audit read access.
- Backup SELECT permits ACCESS SHARE locks but not restore DDL. No table-level
  UPDATE/DELETE/TRUNCATE, owner, superuser or pg_read_all_data shortcut is needed.

References: [PostgreSQL 17 LOCK privileges](https://www.postgresql.org/docs/17/sql-lock.html)
and [default privileges](https://www.postgresql.org/docs/17/sql-alterdefaultprivileges.html).

## Applying the reference profile

Preconditions: an isolated GoFormX database, current migrations applied as the
dedicated owner, freshly provisioned operational roles with the attributes and
membership rules above, and no additional ACLs (including column ACLs), role
settings, SECURITY DEFINER entry points or unexpected schema owners. Execute the
profile **in one transaction as owner**, with psql `ON_ERROR_STOP=1`, supplying
identifier variables `database`, `owner`, `migrator`, `runtime`, `token_operator`,
`key_operator`, `backup`. The `:"variable"` form quotes identifiers; passwords
are not profile inputs. Keep credentials out of command arguments and logs.

The profile removes PUBLIC database/schema authority and application-function
EXECUTE; runtime cannot CREATE schemas, persistent/temporary objects or DDL.
Owner-global default ACLs remove PUBLIC function execution for future objects.
A schema-local REVOKE cannot override the default global PUBLIC EXECUTE grant.
Future tables, sequences and functions are private, including to the backup role,
until reviewed grants are added. Migrations must actually SET ROLE owner: merely
being a member does not apply owner's default privileges to login-owned objects.
Apply migration and required ACL changes atomically before resuming service.

This profile is **not** a sanitizer for arbitrary existing installations. In
particular, it does not erase inherited access, column grants, PUBLIC grants in
other schemas, pre-existing role settings/default ACLs, ownership or cluster-wide
memberships. Reapplying unchanged grants is harmless; removing an old grant needs
an explicit versioned revocation. It must not be wired into API startup or an
initdb-only hook that silently skips populated volumes.

## Verification and retained rollout gates

`task verify` runs `TestDatabasePermissionContract` with a disposable PostgreSQL
administrator URL. The test creates uniquely named database/roles, applies the
real SQL migration chain as owner through a NOINHERIT migrator, applies the same
ACL template, and authenticates runtime/operators/backup through separate logins.
It never modifies role grants or schemas in the supplied database. Cleanup drops
only its generated database and roles; never point this fixture at production.
The canonical migration tool's full-chain/down/up checks remain separate and
unchanged; the fixture's metadata table is not a substitute for testing that tool.

Evidence includes real form/schema/publication/submission/idempotency, token
creation/use/revocation, replay consumption/cleanup, webhook lifecycle/outbox/
worker/replay, export and both audit tables; actual token CLI issue/rotate/revoke;
storage-key rotation and verification; backup SELECT/locks; SQLSTATE 42501 on
forbidden operations; role attributes/membership/ownership; and future-object
denials after a fresh migrator login. No schema mocks or administrator SET ROLE
are used for runtime evidence. These repository checks do not replace the full
browser/PHP/Go launch tests tracked in #168.

Infrastructure #75 must still prove, before #125 closes:

1. Fresh provisioning **and** conversion of a populated existing volume, including
   old owner/superuser credentials, column/default ACLs, memberships, schema and
   function ownership. Verify live state instead of assuming source equals live.
2. Every API/worker/migrator/operator/snapshot/preflight/promotion consumer receives
   only its intended credential; create-once environment generation cannot skip
   conversion. No bootstrap credentials remain in runtime configuration.
3. Actual pg_dump and isolated pg_restore drill: a dump made with `--no-owner`
   / `--no-privileges` does not preserve the permission contract. Restore as owner,
   reapply version-matched ACLs, then rerun positive and negative tests through
   runtime credentials; table-presence checks alone do not prove recovery.
4. Record safe rollback, data/audit retention, future migrations, default ACLs,
   backup coverage, and separately approved production credential rotation.

## Residual authority

This limits SQL capabilities, not cross-organization access after process or
runtime-credential compromise. The API necessarily inserts token hashes and
assertion replay rows, can delete replay rows for expiry cleanup, reads submissions
across organizations and can append audit events. SQL ACLs do not validate those
rows' tenant claims or enforce an expired-only replay DELETE predicate. Form UPDATE
also remains broad because the current repository persists the full model.

Application authentication, current membership resolution, scope/tenant checks,
single-use assertions, immutable snapshots and atomic audit writes remain required.
Do not describe CLI removal, column grants, append-only audit triggers or encrypted
webhook storage as a complete post-compromise isolation boundary. Stronger isolation
would be a separately reviewed architecture decision, not hidden in this issue.
