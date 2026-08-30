# Submission operations (#122)

The Go API owns submission reads, filters, redaction, and exports. PHP is an
authorized presentation client, not another data store or policy implementation.
The same business contract serves external scoped clients and the human UI.

## Filtered reads (development contract 1.2.0)

`GET /v1/forms/{formId}/submissions` requires `submissions:read` and an owned,
non-deleted form. `limit` (1–100, default 25) and the opaque `cursor` paginate by
`submittedAt DESC, id DESC`. Retain filters on each page; this is not a snapshot
of future inserts. Changing a selector never changes the authenticated tenant.

- `receivedFrom` is inclusive; `receivedBefore` is exclusive. Both require an
  explicit RFC 3339 offset, calendar years 1–9999, and at most microsecond
  precision, matching PostgreSQL storage. Empty/invalid or reversed ranges fail.
- `status=accepted` filters immutable acceptance state. Processing, delivered,
  and dead-letter states belong to webhook deliveries, not accepted payloads.
- `schemaVersion` selects the exact accepted version, not the current version.
  It must be an integer between 1 and 2147483647.
- Arbitrary payload search is unsupported. Unknown and repeated parameters fail
  with a bounded generic error rather than silently ignoring a filter. Queries
  exceeding 4096 bytes or containing malformed encoding also fail.

Composite indexes cover form/time/ID and form/version/time/ID. Detail and list
must agree about deleted-form visibility. Historical immutable records remain
unchanged. Times returned by submission resources preserve sub-second precision.
The new indexes use ordinary PostgreSQL index creation, which can block writes;
assess table size and build time during the #125 migration/rollback preflight.
Do not infer production rollout from merged source or a contract version bump.

## Exact-version privacy projection (development contract 1.2.0)

Submission detail now includes the exact accepted `schema`. Public acceptance
and idempotent replay responses, management lists, and detail responses apply
that version's root `x-goformx-sensitive` policy. New schema versions cannot
reinterpret older submissions. These responses are display projections, not
payloads that can be validated or resubmitted unchanged.

For example, `"x-goformx-sensitive": ["/email", "/credentials", "/items/0"]`
removes `email` and the entire `credentials` object and replaces the first array
element with `null`, preserving array indexes. The empty pointer `""` redacts
the whole payload to `{}`. Pointers address payload data, not schema properties;
`~1` escapes `/` and `~0` escapes `~`. Mark the whole array to hide all entries.
There are no wildcards, URI fragments, implicit field-name heuristics, or reveal
flags. `writeOnly` alone does not enforce redaction.

Policies allow up to 128 unique pointers of at most 256 Unicode characters each.
Array selectors use canonical non-negative decimal indexes, without leading
zeros, up to 2147483647. Missing fields, out-of-range indexes, and null intermediate
values are ignored. Other incompatible traversal fails closed; the public API
rejects it before persistence. Malformed historical policies or missing payloads
fail reads rather than returning unredacted data. No policy means no redaction.
Only the root annotation has meaning; nested annotations do not define policy.

Every projection includes sorted `redactedPaths`: the declared policy, including
optional absent fields, rather than a value-dependent list revealing which
secrets exist. Object members are removed rather than replaced with strings that
could be mistaken for user values. Numbers remain lossless, including numbers
beyond JavaScript's safe integer range. Schema snapshots and accepted payloads
are never mutated by projection.

Public submission bodies must contain an object-valued `data`; missing/null data
is rejected. Parse failures use a generic message, since decoder errors can echo
payload keys. For a nonempty sensitive policy, validation failures return one
generic `/data` diagnostic: both error messages and instance paths can contain
sensitive object keys. Management responses and public submission responses use
`Cache-Control: no-store`; this does not control downstream client telemetry.

This is read-time minimization, **not encryption or removal from storage**.
Existing configured webhooks still receive the accepted raw payload. Anyone
configuring a webhook must treat its destination as a recipient of sensitive
data. Schema definitions, defaults, examples, annotation paths, and metadata are
not redacted and must never contain actual secrets. Historical submissions with
no policy are not retroactively protected; do not rewrite immutable history to
pretend otherwise. Review that exposure before rollout.

## Runtime logging boundary (#149, #150)

The runtime ORM adapter emits event severity, elapsed milliseconds, and a bounded
error category (for example `conflict`, `invalid_data`, or `database_error`). It
does not evaluate the SQL-rendering callback or forward SQL, parameters, literal
values, arbitrary diagnostic arguments, or driver error messages. The old
`parameterized` configuration field is retained for parsing compatibility but
cannot enable value logging. Log levels, configured slow-query thresholds, and
ignored not-found queries remain supported; the default slow threshold is 200 ms.
Raw SQL and row-count diagnostics are deliberately not included in this adapter.

HTTP repository failures retain their request ID but not the raw error; form
repository error events likewise omit driver text. Field sanitization is
stateless: it no longer memoizes original request values in an unbounded shared
map. This removes both indefinite in-process value retention and a reproduced
concurrent-map-write crash without adding another cache or locking policy.

Regression evidence includes real production logging-factory output at debug
level, real GORM query construction, real PostgreSQL driver errors, concurrent
sanitization, and the HTTP/PostgreSQL submission flow with a connection-local
fault-injection callback. The latter verifies that a failed insert does not
persist a submission or echo the synthetic payload/error into ORM or HTTP logs.
These tests do not prove that proxy/access logs, database-server logging, crash
dumps, or external collectors are correctly configured. Audit those during #125;
this code change does not erase historical logs or establish a production leak.

## Bounded audited exports (development contract 1.2.0)

`POST /v1/forms/{formId}/submissions/export` requires `submissions:read`, with
the same two distinct credential classes as reads. The JSON body requires
`format: "json"` or `format: "csv"` and optionally the existing `receivedFrom`,
`receivedBefore`, `status`, and `schemaVersion` filters. Filter values never
belong in a URL. Query strings, unknown/duplicate/null/empty fields, cursor or
limit overrides, invalid ranges, and bodies larger than 4096 bytes fail with a
generic 400 response. Integral numeric spellings such as `1.0` and `1e0` select
version 1; near-integers are not rounded.

The database reads one statement snapshot ordered by acceptance time and ID
descending, joining each row to its exact accepted schema's root privacy policy.
It does not join to the current form version or repeatedly fetch full schemas.
The row count and source size are evaluated **before PostgreSQL sends payloads**.
Source size includes payload/policy text, request/status metadata and a fixed
per-row allowance. The extra sentinel row detects overflow, not a partial page.

| Budget | Limit |
| --- | ---: |
| Accepted rows | 1,000 |
| Candidate source data | 8 MiB |
| Encoded download | 8 MiB |
| Application processing deadline | 10 seconds |
| CSV columns, including metadata | 256 |
| Nested CSV object levels | 32 |
| Concurrent exports per API instance | 1 |

OpenAPI exposes these as `x-goformx-export-limits`, checked against domain
constants. Source/row/output/depth/column overflow returns 413 with no download;
narrow the filters and retry. A timeout returns 504. A second concurrent export
receives 429 with `Retry-After: 1`, without entering a wait queue; authorization
still runs before this admission check. Deployment traffic and memory budgets
still require capacity validation: decoded JSON uses more memory than its wire
representation, and this admission limit is per instance, not cluster-wide.
One very large historical row can
require a separately reviewed administrative recovery path instead of this API.

JSON contains the normal redacted submission resources plus
`meta.exportId`, `meta.preparedAt`, and `meta.rowCount`. Exact JSON numbers are
preserved. CSV contains ID, form ID, accepted schema version, request ID, status,
acceptance time, and redacted-path metadata, followed by sorted `data:/pointer`
columns. Objects are flattened using escaped JSON Pointers; arrays and empty
objects remain JSON text in a cell. Column selection follows the redacted
projection, never the raw payload. Missing and empty string cells are not
distinguishable in CSV; use JSON for that distinction and machine round trips.

Every CSV cell is double-quoted, internal quotes doubled, and apostrophe-prefixed
as text, including headers and numeric values. This protects the initial artifact
against formula-leading text and avoids presenting large numbers as spreadsheet
numbers. A strict CSV reader will see the apostrophe. Spreadsheet applications
and re-save/import workflows can change these protections; this is not universal
round-trip safety. See [OWASP's CSV injection guidance](https://owasp.org/www-community/attacks/CSV_Injection).
Do not remove the text prefix and then treat untrusted cells as formulas.

Before any attachment headers or bytes are released, a mandatory repository
operation rechecks form ownership/deletion and persists an `export.prepared`
audit record. Audit failure returns 503, not an unaudited download. Records
include export/organization/form IDs, actor ID, credential class and non-secret
lookup ID, request ID, format, row/byte counts and time. They do not include
payloads, schema, filters, bearer credentials, or body digests. `prepared` means
the artifact was prepared and audited; it does not claim the client received it.
Successful responses declare the complete artifact size in `Content-Length`.
Download proxies must compare this with received bytes before releasing a file;
a capped or interrupted HTTP read must not become a successful partial export.
The audit ID is also returned in `X-GoFormX-Export-ID`; filenames use only that
generated UUID, never a user-provided form title.

Migration `2026083002` adds the audit table without foreign-key cascades. Audit
rows survive form/token deletion, and ordinary UPDATE, DELETE, and TRUNCATE are
rejected by triggers. This is not tamper-proof against a privileged database
administrator. Down migration succeeds only while the table is empty; once an
export is recorded, roll back the application without dropping audit history.
Include the new table in backup/restore and retention decisions before #125.

## Remaining delivery contract — required before closing #122

The following remain required before closing #122:

- UI detail rendering uses the exact accepted schema, retaining numeric precision.
  Delivery state and trace metadata remain distinct from acceptance state.
- The control-plane export action must use this Go API directly, with no browser
  credentials, client-side re-redaction, or alternate unaudited export path.
- The control plane will authorize owner/admin submission operations on fresh
  membership resolution; ordinary members retain form/schema-only access.
- Empty/loading/error/populated UI, exact-version display, bounded exports,
  telemetry/history privacy, and adversarial cross-tenant/form/membership tests
  must pass before claiming the complete submissions workflow.
- Deployment privacy verification must include proxy/access logs, PostgreSQL
  server logging, and external collectors; application regressions alone cannot
  prove those independently configured boundaries.

These remaining decisions and acceptance tests are not satisfied by a filter API
or by the earlier numeric correction in #144. No deployment is implied.
