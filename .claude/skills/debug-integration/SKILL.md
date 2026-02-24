---
name: debug-integration
description: Diagnose Laravel-to-Go integration issues (assertion auth, HMAC, signatures, CORS, connectivity)
user-invocable: true
activation-keywords:
  - assertion
  - HMAC
  - signature
  - shared secret
  - X-Signature
  - X-User-Id
  - CORS
  - GoFormsClient
  - connection refused
  - 401 unauthorized
  - 403 forbidden
---

# Integration Debugger

Systematic diagnostic checklist for Laravel-to-Go cross-service issues in GoFormX.

## When to Use

Activate when encountering:
- Assertion auth failures (401/403 from Go API)
- HMAC signature mismatches
- CORS errors in form embeds
- Connection refused between services
- User sync issues (shadow users)

## Diagnostic Checklist

Work through each step in order. Stop when you find the root cause.

### 1. Shared Secret Match

Compare the `GOFORMS_SHARED_SECRET` value in both services:

- **Laravel**: `goformx-laravel/.env` (line `GOFORMS_SHARED_SECRET=`)
- **Go**: `goforms/.env` (line `SHARED_SECRET=`)
- **Used in**: `goformx-laravel/app/Services/GoFormsClient.php` (signs requests) and `goforms/internal/application/middleware/assertion/assertion.go` (verifies)

If they don't match, that's your problem. Update one to match the other.

### 2. Timestamp Skew

The assertion middleware allows 60 seconds of clock skew.

- Check: `goforms/internal/application/middleware/assertion/assertion.go` — look for `TimestampSkew` or time comparison logic
- If DDEV containers have drifted clocks, restart: `ddev restart`
- Verify: `ddev exec date && date` — compare container vs host time

### 3. Network Connectivity

Test that Laravel can reach the Go API:

```bash
# From inside the Laravel DDEV container
ddev exec curl -s -o /dev/null -w "%{http_code}" http://goforms:8090/health
```

- Expected: `200`
- If connection refused: Go sidecar may not be running — check `ddev describe` for the goforms service
- Check `GOFORMS_API_URL` in `goformx-laravel/.env` — should be `http://goforms:8090` in DDEV

### 4. CORS Configuration

Per-form CORS is handled by:

- **Middleware**: `goforms/internal/application/handlers/web/form_cors_middleware.go`
- **Public endpoints**: `GET /forms/:id/schema`, `POST /forms/:id/submit`, `GET /forms/:id/embed`

Check:
- Are `allowed_origins` set correctly on the form?
- Is the middleware allowing credentials + wildcard origin simultaneously? (invalid per spec)
- Browser console will show the specific CORS header that's missing

### 5. User Sync (Shadow Users)

Go creates shadow user records from assertion headers on first request.

- **Syncer**: Check `goforms/internal/domain/user/` for sync logic
- If a user exists in Laravel but not in Go's `users` table, the first authenticated request creates them
- Verify: Check Go logs for user sync errors

### 6. Laravel Error Handling

`goformx-laravel/app/Http/Controllers/FormController.php` maps Go API errors:

- `422` → Validation errors (re-thrown to Inertia)
- `404` → Not found
- `5xx` → Flash error message

If errors are being swallowed, check the `catch (RequestException $e)` blocks in FormController.

### 7. GoFormsClient Config

- **Service class**: `goformx-laravel/app/Services/GoFormsClient.php`
- **Config binding**: `goformx-laravel/config/services.php` (look for `goforms` key)
- **Usage pattern**: `GoFormsClient::fromConfig()->withUser(auth()->user())->listForms()`

Verify:
- `GOFORMS_API_URL` is set and reachable
- `GOFORMS_SHARED_SECRET` is set (not empty)
- The `withUser()` call receives a valid authenticated user

## Output Format

After diagnosis, report:

```
ROOT CAUSE: [description]
LOCATION: [file path]
FIX: [specific action to take]
```
