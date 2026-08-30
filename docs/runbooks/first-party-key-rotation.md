# First-party assertion key rotation and emergency revocation

This runbook applies only to Waaseyaa's Ed25519 first-party assertion keys. External `gfst_` service tokens have independent storage and revocation.

## Preconditions

- Keep private keys only in Waaseyaa server-side secret custody. Never put them in GoFormX, a browser response, an image, a log, or Git.
- Keep `FIRST_PARTY_ASSERTION_ISSUER`, `FIRST_PARTY_ASSERTION_AUDIENCE`, and `FIRST_PARTY_ASSERTION_JWKS_URL` pinned to the accepted ADR values.
- Deploy `FIRST_PARTY_ASSERTION_JWKS_SNAPSHOT` with at least one public `OKP`/`Ed25519` JWK. Every key must declare `alg: EdDSA` and one state: `next`, `active`, `retiring`, or `revoked`.
- Apply the replay-store migration before enabling `FIRST_PARTY_ASSERTION_ENABLED`.
- Keep the API process running long enough for its five-minute replay cleanup loop to remove expired identities. Cleanup failures are logged and retried without weakening one-time consumption.

## Normal rotation

1. Generate a new Ed25519 key pair in Waaseyaa secret custody. Assign a new opaque `kid`; do not reuse an identifier.
2. Publish the new public JWK with state `next`, leaving the current key `active`. Update the deployable GoFormX snapshot to contain both keys.
3. Wait for at least one GoFormX refresh interval and verify a signed test request from each key reaches an organization-owned read endpoint once; replaying either assertion must return `401`.
4. Promote the new JWK to `active`, mark the previous key `retiring`, deploy the updated snapshot, and switch Waaseyaa signing to the new private key.
5. Keep the retiring public key published for at least 65 seconds. Then mark it `revoked` in live JWKS and deployment/rollback snapshots and prove it returns `401`. Retirement must leave explicit negative trust information, not merely an absent key.
6. The key may be omitted from live discovery after every verifier has observed revocation, but retain its revoked tombstone in deployment/rollback snapshots. A later stale discovery response must not restore it. Verify the retired key remains `401`, the active key succeeds, an under-scoped active assertion returns `403`, and `gfst_` smoke tests remain unchanged.

## Emergency revocation

1. Stop Waaseyaa assertion issuance. If incident scope is uncertain, set `FIRST_PARTY_ASSERTION_ENABLED=false` and deploy immediately; this does not disable `gfst_` integrations.
2. Mark the affected `kid` as `revoked` in the live JWKS and `FIRST_PARTY_ASSERTION_JWKS_SNAPSHOT`, then deploy GoFormX. Do not merely remove it while any node may retain an older cached set.
3. Rotate Waaseyaa signing custody to a newly generated key and audit request IDs associated with the affected key window. Assertions and payloads must not appear in logs.
4. Prove an otherwise-valid assertion from the revoked key returns `401`, replay persistence is healthy, and the replacement key passes tenant-isolation and scope checks.
5. Restore `FIRST_PARTY_ASSERTION_ENABLED=true` only after the replacement public key and replay migration are present on every GoFormX node.

Explicit revocation is monotonic for a running verifier: neither a stale discovery
response, omission followed by reintroduction, nor different key material under
the same `kid` can reactivate it. A response from an older overlapping refresh
cannot replace a newer accepted key set, but explicit revocations in that older
response still take effect. These protections do not persist process
memory across restart: keep revoked-key tombstones in deployment snapshots and
live JWKS throughout incident recovery, including rollback configuration. Never
reuse a revoked key ID. A stale **cold-start** snapshot is not safe rollback input.

Response ordering only distinguishes overlapping requests; it cannot identify
old content served as the response to a later request. The explicit retirement
tombstone in steps 5–6 is therefore required even for planned rotation. Removing
a key without recording revocation does not protect against publisher rollback.

## Automated data-plane drill and remaining deployment evidence

`task verify` runs `TestFirstPartyKeyRotationDrill` with real Ed25519 assertions,
HTTPS discovery, HTTP routes, and PostgreSQL token/replay stores. It exercises
active/next overlap, signer promotion, retiring-key removal, scope and tenant
denials, emergency revocation, stale discovery, first-party disablement,
revoked-snapshot cold starts during discovery outage, and replay persistence
across verifier restart. External service-token access must remain valid at every
transition. Unit regressions additionally control overlapping response order and
key omission/reintroduction to prove revocations cannot be undone.

This test generates disposable signing fixtures in memory. It does **not** rotate
Waaseyaa production custody, wait the real 65-second drain, update deployment
snapshots, restart deployed processes, or prove every production node refreshed.
Before public launch, record those operational observations from the steps above
in #120/#125. Do not interpret this automated drill as a completed deployment
rehearsal or remove revoked keys from rollback snapshots.

## Failure interpretation

- `401`: malformed, expired, replayed, wrongly addressed, wrongly signed, unknown, removed, or revoked assertion.
- `403`: valid assertion without the route's required canonical scope.
- `404`: authenticated organization does not own the requested resource.
- `503`: replay persistence or first-party authentication infrastructure failed; the operation did not run.
