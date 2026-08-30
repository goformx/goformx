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

## Remaining delivery contract — not implemented by filtered reads

The following remain required before closing #122:

- Detail rendering uses the exact accepted schema, retaining numeric precision.
  Delivery state and trace metadata remain distinct from acceptance state.
- A root schema annotation will explicitly identify sensitive payload locations
  with bounded JSON Pointers. The accepted immutable version owns that policy;
  current-version edits cannot reinterpret an older payload. Malformed policy
  must fail closed. No automatic field-name classifier is promised, and a JSON
  Schema annotation such as `writeOnly` alone is not an access-control policy.
- Go applies the same redaction to management reads and exports, without changing
  stored data. UI masking alone is insufficient. Secrets must not be embedded in
  schema defaults or examples. Webhook delivery policy must be documented
  separately rather than silently changing integration payloads.
- JSON/CSV exports require explicit authorization, a hard row and byte budget,
  lossless JSON values, CSV formula-injection defenses, and an audit record that
  excludes payload values and secret-bearing selectors. Incomplete exports must
  be explicit, not silently truncated. Audit persistence failures fail closed.
- The control plane will authorize owner/admin submission operations on fresh
  membership resolution; ordinary members retain form/schema-only access.
- Empty/loading/error/populated UI, exact-version display, bounded exports,
  telemetry/history privacy, and adversarial cross-tenant/form/membership tests
  must pass before claiming the complete submissions workflow.

These remaining decisions and acceptance tests are not satisfied by a filter API
or by the earlier numeric correction in #144. No deployment is implied.
