# GoFormX Laravel + Go Split Architecture Design

**Status:** Approved  
**Date:** 2026-02-18

## Summary

Laravel owns identity and UI; Go owns the entire forms domain. Laravel serves the authenticated frontend (dashboard, form builder) and proves user identity to Go via signed assertions. Go exposes form CRUD APIs and serves public embed/submit endpoints directly.

---

## 1. Architecture Overview

### System Boundaries

```
┌─────────────────────────────────────────────────────────────────┐
│                         LARAVEL (goformx-laravel)                │
│  • Auth (Fortify: login, register, 2FA, sessions)                │
│  • Inertia + Vue pages (dashboard, form builder)                 │
│  • User DB (users, sessions, password_resets, etc.)              │
│  • Outbound: signed assertions to Go for form operations         │
└───────────────────────────────┬─────────────────────────────────┘
                                │ HTTP + X-User-Id, X-Timestamp, X-Signature
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                         GO (goforms → API-only)                  │
│  • Form domain (CRUD, schema, submissions)                       │
│  • Event bus                                                     │
│  • Form DB (forms, submissions, etc.)                            │
│  • Public: embed, submit (no auth)                               │
│  • Authenticated: form operations (trusts Laravel assertions)    │
└─────────────────────────────────────────────────────────────────┘
```

### Traffic Flow

| Flow | Path | Auth |
|------|------|------|
| Dashboard, builder | Browser → Laravel → Go API | Laravel session + signed assertion |
| Public embed | Browser → Go | None |
| Public submit | Browser → Go | None (rate limited) |

### Responsibility Split

| Concern | Owner |
|---------|-------|
| Identity, sessions, auth UI | Laravel |
| Form CRUD, schema, submissions, events | Go |
| Form builder UI | Laravel (Vue/Inertia) |
| Public embed/submit | Go |

---

## 2. Signed Assertion Format

### Outbound (Laravel → Go)

Laravel sends these headers on each request to Go:

| Header | Purpose |
|--------|---------|
| `X-User-Id` | Authenticated user ID from Laravel |
| `X-Timestamp` | ISO 8601 (e.g. `2026-02-18T20:10:00Z`) |
| `X-Signature` | HMAC-SHA256 of `user_id:timestamp` using shared secret |
| `X-User-Roles` | Optional comma-separated roles |

**Payload signed:** `{user_id}:{timestamp}`  
**Secret:** Shared via `GOFORMS_SHARED_SECRET` in both apps.

### Verification (Go)

1. Reject if `X-User-Id`, `X-Timestamp`, or `X-Signature` is missing
2. Reject if timestamp is older than 60 seconds (configurable)
3. Recompute HMAC-SHA256 of `user_id:timestamp`, compare to `X-Signature`
4. If valid, attach `user_id` to request context and proceed

### Public Routes

No assertion headers required. Go treats as unauthenticated, applies public logic and rate limiting.

---

## 3. Go API Surface

### Authenticated Endpoints (require signed assertion)

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/api/forms` | List forms for user |
| POST | `/api/forms` | Create form |
| GET | `/api/forms/:id` | Get form (ownership check) |
| PUT | `/api/forms/:id` | Update form (incl. schema) |
| DELETE | `/api/forms/:id` | Delete form |
| GET | `/api/forms/:id/submissions` | List submissions |
| GET | `/api/forms/:id/submissions/:sid` | Get single submission |

### Public Endpoints (no auth)

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/forms/:id/embed` | Embed HTML/JS or schema |
| GET | `/forms/:id/schema` | Schema only (embed/preview) |
| POST | `/forms/:id/submit` | Public form submission |

### Response Format

- Success: `200`/`201` with JSON
- Validation: `422` with error details
- Not found: `404`
- Unauthorized: `401`

---

## 4. Laravel Integration

- **Go client:** `App\Services\GoFormsClient` or similar — base URL, shared secret, auto-signs all requests with current user
- **Controllers:** Thin — auth user, call client, pass data to Inertia
- **Form builder:** Port from goforms — Vue/Form.io; schema load/save via Laravel → Go
- **Config:** `config/services.php` — `goforms.url`, `goforms.secret`
- **Routes:** Laravel owns `/dashboard`, `/forms`, `/forms/:id/edit`, etc.

### User identity sync

Laravel is the **source of truth** for users (Fortify, `users` table). Go has a `users` table only because `forms.user_id` has a foreign key to `users.uuid` for referential integrity.

**Options:**

| Approach | Description |
|----------|-------------|
| **Manual** | Seed goforms `users` for each Laravel user (e.g. after migrations). Simple but doesn’t scale; easy for dev. |
| **Lazy sync (recommended)** | On the first assertion-authenticated request that needs the user (e.g. create form), Go ensures a row exists: `GetByID(ctx, userID)`; if not found, insert a minimal user (`uuid = X-User-Id`, placeholder email, non-login password). No change to Laravel; no manual step. |
| **Push sync** | Laravel calls a Go “sync user” endpoint on login/register; Go upserts the user. Keeps Go in sync on auth events; requires a new endpoint and Laravel hook. |

**Recommendation:** **Lazy sync** in Go. Laravel keeps owning auth; Go creates a shadow user on first use so the FK is satisfied and IDs stay in sync without extra work.

---

## 5. Error Handling

### Laravel → Go Failures

| Scenario | Laravel behavior |
|----------|------------------|
| Go unreachable | 502/503, "Form service temporarily unavailable" |
| Go 422 | Map to Inertia validation errors |
| Go 404 | 404 or redirect; **list forms:** render index with empty list |
| Go 401 | Log, treat as 500 (misconfiguration); **list forms:** render index with empty list |
| Go 5xx | 502, generic message, log |

### Go-Side

| Scenario | Go behavior |
|----------|-------------|
| Invalid assertion | 401 |
| Valid assertion, not owner | 403 |
| Form not found | 404 |
| Validation failure | 422 with JSON details |

### Form Builder Auto-Save

- Same pattern as goforms — debounced save, Laravel calls Go
- On failure: toast/inline error, retain dirty state, allow retry

### Public Submit

- Go validates, returns 422 with field errors; embed script surfaces them
- Rate limit: 429 with Retry-After

---

## 6. Go Refactor Scope

### Removed from Go

- Inertia/page rendering
- FormWebHandler, PageHandler
- AuthHandler (login, register, sessions)
- gonertia, Vue SSR
- Asset server for Vite/dev

### Kept in Go

- Form domain (service, repository, model)
- FormAPIHandler (evolved to authenticated API)
- Public embed/submit routes
- Event bus, form events
- Middleware (CORS, recovery, logging)
- Validation, response builders
- PostgreSQL, GORM, migrations

### New in Go

- Assertion verification middleware
- `/api/forms/*` route group
- Optional: health check with DB

### User ID in Go

- `user_id` from assertion used for ownership only; opaque string, no user table in Go

---

## 7. Local Development and Deployment

### Local

- Laravel: `composer run dev` (or `php artisan serve` + `npm run dev`)
- Go: `task dev:backend` in goforms (port 8090)
- Laravel `.env`: `GOFORMS_API_URL`, `GOFORMS_SHARED_SECRET`
- Go `.env`: same `GOFORMS_SHARED_SECRET`
- Separate DBs: Laravel (users/sessions), Go (forms)

### Production

- Laravel: standard PHP deployment
- Go: binary or container, internal URL
- Reverse proxy: `/` → Laravel; `/forms/:id/embed`, `/forms/:id/submit` → Go
- Both apps share `GOFORMS_SHARED_SECRET`

### Existing Data

- Fresh start vs migrate users/forms: TBD per deployment need
