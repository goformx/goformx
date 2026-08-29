# Cross-service authentication and trust boundaries

**Status:** Active for the schema-first v1 API. The former HMAC assertion, shadow-user, and plan-header design is retired and must not be used by production routes.

## Planes and credentials

GoFormX exposes two deliberately different trust boundaries:

| Caller | Plane | Credential | Authority |
| --- | --- | --- | --- |
| Public browser | Data | Rotatable gfpk_ public form key | Read one published schema and submit against it from an allowed origin |
| Anokii or Waaseyaa server | Control | Scoped gfst_ bearer token | Only the owner and scopes stored with that token |
| Trusted automation or agent | Control | Scoped gfst_ bearer token | Same as any server integration; there is no agent superuser |
| GoFormX operator | Operations | Database access through the token CLI | Issue, rotate, and revoke service tokens |

Public form keys are identifiers, not secrets. They may appear in browser JavaScript. A reusable bearer token must remain in a server-side secret store and must never use a VITE_-prefixed variable or be returned to a browser.

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

The supported scopes are forms:read, forms:write, forms:publish, and submissions:read. Adding a scope changes the authorization contract and requires tests and documentation in the same change.

## Rotation procedure

    go run ./cmd/goformx-token rotate --token-id TOKEN_ID --ttl 24h

Store the returned replacement token before restarting its caller. Rotation revokes the old credential in the same database transaction; there is no overlap window. If an integration needs an overlap window, issue a second token first and revoke the original after the caller has switched.

## Rejected legacy headers

The v1 control plane accepts only Authorization: Bearer gfst_.... It does not authenticate X-User-Id, X-Timestamp, X-Signature, plan-tier headers, browser sessions, or CSRF tokens. Public data-plane routes accept no reusable credential.

The former `/api/*` routes and their authentication packages have been deleted. They are absent from the v1 OpenAPI contract and production dependency graph.

## Failure behavior

| Condition | Result |
| --- | --- |
| Missing or unknown bearer token | 401 unauthorized |
| Invalid, expired, revoked, or under-scoped token | 403 forbidden |
| Token owner differs from the requested form owner | 403 forbidden |
| Usage-audit persistence is unavailable | 503 service unavailable; the request does not proceed |
| Public key is unknown or has no published schema | 404 not found |
