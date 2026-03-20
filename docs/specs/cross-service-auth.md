# Cross-Service Authentication Spec

**Owns:** HMAC assertion auth between Waaseyaa frontend and Go API backend

---

## Overview

GoFormX uses a two-service architecture where the Waaseyaa frontend authenticates users and asserts their identity to the Go backend via HMAC-signed headers. The Go API never handles user authentication directly — it trusts assertions from the frontend after cryptographic verification.

---

## Authentication Flow

```
1. User authenticates via Waaseyaa (session-based, AuthManager)
2. Frontend reads $_SESSION['waaseyaa_uid'] to get authenticated user ID
3. GoFormsClient signs each request with HMAC headers:
   - X-User-Id: authenticated user's UUID
   - X-Timestamp: current UTC timestamp (ISO 8601)
   - X-Signature: HMAC-SHA256(user_id:timestamp, shared_secret)
4. Go assertion middleware verifies:
   - Signature matches recomputed HMAC
   - Timestamp within 60-second skew tolerance
   - Extracts user ID for downstream ownership checks
```

---

## Shared Secret

- **Config key (Waaseyaa):** `goforms_shared_secret` in `config/waaseyaa.php`
- **Config key (Go):** `GOFORMS_SHARED_SECRET` env var
- **Critical invariant:** Both values MUST match. Mismatched secrets cause silent 401s on every API call.

---

## GoFormsClient (Waaseyaa → Go)

**Location:** `goformx-web/src/Service/GoFormsClient.php`

- Implements `GoFormsClientInterface`
- Constructor: `(string $baseUrl, string $sharedSecret)`
- All requests include HMAC assertion headers
- Methods: `get()`, `post()`, `put()`, `delete()` — each takes path, userId, planTier
- Throws `\RuntimeException` on HTTP errors (caught by controllers)

---

## Go Assertion Middleware

**Location:** `goforms/internal/application/middleware/assertion/`

- Verifies `X-Signature` against recomputed HMAC-SHA256
- Rejects requests with expired timestamps (>60s skew)
- Extracts user ID and attaches to request context
- Used by all `/api/*` routes (authenticated endpoints)

---

## Public Endpoints (No Auth)

These Go endpoints require NO assertion headers:

- `GET /forms/:id/schema` — returns form JSON schema
- `POST /forms/:id/submit` — accepts public form submissions
- `GET /forms/:id/embed` — returns embeddable form HTML

Rate limiting is applied at the Go level for public endpoints.

---

## Security Considerations

- HMAC secret must be cryptographically random (minimum 32 bytes)
- Timestamp skew tolerance prevents replay attacks beyond 60 seconds
- The frontend MUST NOT expose the shared secret to the browser
- Go API should log failed assertion attempts for monitoring
- Plan tier is passed as a header but NOT cryptographically verified — it's advisory for rate limiting, not access control

---

## Failure Modes

| Symptom | Cause | Fix |
|---------|-------|-----|
| All API calls return 401 | Mismatched `GOFORMS_SHARED_SECRET` | Verify both `.env` files have identical values |
| Intermittent 401s | Clock drift between services | Ensure NTP sync; check 60s skew tolerance |
| Forms load but submissions fail | Public endpoints work, auth endpoints don't | Check assertion middleware is only on `/api/*` routes |
| "Connection refused" from GoFormsClient | Go API not running | Verify `GOFORMS_API_URL` and container health |
