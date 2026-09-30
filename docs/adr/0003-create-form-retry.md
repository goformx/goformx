# 0003: Durable create-form retries

Status: accepted for local implementation, pending review.

`POST /v1/forms` accepts an optional `Idempotency-Key` to preserve existing
callers. New assistant and control-plane callers should generate a random key
of 16 to 128 characters per intended create and retain it across uncertain
retries. Each first-party assertion is single use, so every retry needs a fresh
assertion bound to `createForm`.

The service hashes the accepted create inputs after JSON decoding and validation.
The digest uses a versioned encoding of the request model. Adding an accepted
field, including a site association, must extend that model and the digest in
the same change. JSON object key order and whitespace do not affect the digest.
Omitted and empty allowed-origin arrays are equivalent. The key is scoped to
the authenticated organization. A reused key with changed inputs returns HTTP
409 `idempotency_conflict`.

PostgreSQL commits the form, first immutable schema version, and receipt in one
transaction under an organization and key lock. The receipt retains a snapshot
of the original creation result. A matching replay returns HTTP 201, the same
form identity, Location, ETag and body, with `X-GoFormX-Replayed: true`, even
if the form has since been edited or soft deleted. Receipts do not expire.
Different organizations may use the same key independently. Requests without
a key keep the legacy create behavior.

The runtime role receives only SELECT and INSERT on receipts; the backup role
reads them. A down migration refuses to drop a populated receipt table. A
deployment rollback must preserve this table and its records, or explicitly
stop for an operator-led recovery decision.
