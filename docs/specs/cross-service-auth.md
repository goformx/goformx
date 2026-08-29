# Cross-service authentication and trust boundaries

**Status:** Active for the schema-first v1 API. [ADR 0002](../adr/0002-first-party-assertions.md) freezes the first-party assertion profile. The former HMAC headers, shadow-user, and plan-header design remain retired.

## Planes and credentials

GoFormX exposes two deliberately different trust boundaries:

| Caller | Plane | Credential | Authority |
| --- | --- | --- | --- |
| Public browser | Data | Rotatable gfpk_ public form key | Read one published schema and submit against it from an allowed origin |
| Waaseyaa server acting for a signed-in person | Control | Single-use EdDSA first-party assertion | Resolved user, organization, and scopes signed for one request |
| Anokii or another first-party server without a human session | Control | Scoped gfst_ bearer token | Only the organization owner and scopes stored with that token |
| Trusted automation or agent | Control | Scoped gfst_ bearer token | Same as any server integration; there is no agent superuser |
| GoFormX operator | Operations | Database access through the token CLI | Issue, rotate, and revoke service tokens |

Public form keys are identifiers, not secrets. They may appear in browser JavaScript. A reusable bearer token must remain in a server-side secret store and must never use a VITE_-prefixed variable or be returned to a browser.

The Waaseyaa assertion is not reusable: it lives for at most 60 seconds and its `jti` is accepted once. It is minted only after Waaseyaa resolves the acting user's active organization membership. Its private Ed25519 signing key remains in Waaseyaa secret custody and no assertion is returned to browser code.

## Service-token lifecycle

Service tokens:

- are random, prefixed with gfst_, and returned in plaintext exactly once;
- persist only a SHA-256 hash plus a non-secret lookup ID;
- belong to one owner and contain an explicit set of supported scopes;
- have creation and expiry timestamps;
- fail closed when expired, revoked, under-scoped, invalid, or used for another owner;
- record last_used_at after successful authentication;
- can be rotated atomically, revoking the previous token with revocation_reason=rotated and a replaced_by_token_id lineage;
- can be revoked directly by non-secret token ID.

The supported scopes are `forms:read`, `forms:write`, `forms:publish`, `submissions:read`, `tokens:read`, `tokens:write`, `webhooks:read`, and `webhooks:write`. The assertion `scp` claim and service-token scope set use this same registry. Adding a scope changes both credential contracts and requires tests, fixtures, and documentation in the same change.

## First-party assertion lifecycle

Waaseyaa signs a compact JWS with `alg=EdDSA`, `typ=gofx-fpa+jwt`, a configured `kid`, exact issuer and audience, opaque user and organization UUIDs, canonical scopes, a request UUID, and a unique assertion UUID. GoFormX verifies signature, algorithm, type, key state, issuer, audience, version, 60-second lifetime, five-second clock skew, organization ownership, route scope, and one-time replay insertion before a handler runs.

Public verification keys are discovered at `https://goformx.com/.well-known/goformx-control-plane-jwks.json` and may be supplied as a deployment snapshot. GoFormX never follows a key URL embedded in a token. Rotation publishes the next key before use, overlaps public verification for at least 65 seconds, and removes the retiring key after the assertion window has drained. Revoked keys fail immediately.

## Rotation procedure

    go run ./cmd/goformx-token rotate --token-id TOKEN_ID --ttl 24h

Store the returned replacement token before restarting its caller. Rotation revokes the old credential in the same database transaction; there is no overlap window. If an integration needs an overlap window, issue a second token first and revoke the original after the caller has switched.

## Accepted transport and rejected legacy headers

The v1 management plane accepts either `Authorization: Bearer gfst_...` or the exact first-party compact JWS profile. Classification is one-way and a failed credential never falls back to the other verifier. It does not authenticate X-User-Id, X-Timestamp, X-Signature, plan-tier headers, browser sessions, or CSRF tokens. Public data-plane routes accept no reusable credential.

The former `/api/*` routes and their authentication packages have been deleted. They are absent from the v1 OpenAPI contract and production dependency graph.

## Failure behavior

| Condition | Result |
| --- | --- |
| Missing, malformed, unknown, expired, replayed, or revoked credential | 401 unauthorized |
| Authenticated credential lacks the operation scope | 403 forbidden |
| Credential organization differs from the resource owner | Uniform owner-scoped 404 |
| Token usage audit or assertion replay persistence is unavailable | 503 service unavailable; the request does not proceed |
| Public key is unknown or has no published schema | 404 not found |
