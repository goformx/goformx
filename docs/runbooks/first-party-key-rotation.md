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
5. Keep the retiring public key published for at least 65 seconds. Then remove it from both the live JWKS and deployment snapshot.
6. Verify the removed key returns `401`, the active key succeeds, an under-scoped active assertion returns `403`, and `gfst_` smoke tests remain unchanged.

## Emergency revocation

1. Stop Waaseyaa assertion issuance. If incident scope is uncertain, set `FIRST_PARTY_ASSERTION_ENABLED=false` and deploy immediately; this does not disable `gfst_` integrations.
2. Mark the affected `kid` as `revoked` in the live JWKS and `FIRST_PARTY_ASSERTION_JWKS_SNAPSHOT`, then deploy GoFormX. Do not merely remove it while any node may retain an older cached set.
3. Rotate Waaseyaa signing custody to a newly generated key and audit request IDs associated with the affected key window. Assertions and payloads must not appear in logs.
4. Prove an otherwise-valid assertion from the revoked key returns `401`, replay persistence is healthy, and the replacement key passes tenant-isolation and scope checks.
5. Restore `FIRST_PARTY_ASSERTION_ENABLED=true` only after the replacement public key and replay migration are present on every GoFormX node.

## Failure interpretation

- `401`: malformed, expired, replayed, wrongly addressed, wrongly signed, unknown, removed, or revoked assertion.
- `403`: valid assertion without the route's required canonical scope.
- `404`: authenticated organization does not own the requested resource.
- `503`: replay persistence or first-party authentication infrastructure failed; the operation did not run.
