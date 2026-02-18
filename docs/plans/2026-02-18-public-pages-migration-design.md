# Public Pages Migration Design

**Status:** Approved  
**Date:** 2026-02-18

## Summary

Laravel serves the home/landing page and the standalone form-fill page (`/forms/{id}`). Go continues to serve the public embed runtime (`/forms/:id/embed`, `/forms/:id/schema`, `/forms/:id/validation`, `/forms/:id/submit`) and the authenticated API (`/api/forms/*`). Laravel remains the UI + identity layer; Go remains the form domain + public runtime.

---

## 1. Scope and Boundary

- **Laravel** serves the home/landing page and the standalone form-fill page (`/forms/{id}`).
- **Go** continues to serve the public embed runtime (`/forms/:id/embed`, `/forms/:id/schema`, `/forms/:id/validation`, `/forms/:id/submit`) and the authenticated API (`/api/forms/*`).
- **Laravel** remains the UI + identity layer.
- **Go** remains the form domain + public runtime.

---

## 2. Laravel Scope (Landing + Standalone Form-Fill)

### Routes

| Route | Purpose |
|-------|--------|
| `/` | Home/landing: marketing, product explanation, auth links (login/register), entry to dashboard. |
| `/forms/{id}` | Standalone form-fill: first-party, branded page; loads schema from Go, renders Form.io (Vue), submits to Go's public `POST /forms/:id/submit`. |

### Data Flow for `/forms/{id}`

1. User requests Laravel `GET /forms/{id}` (no auth required).
2. Laravel resolves `id` and optionally checks that the form exists (e.g. via Go or a lightweight existence check if available).
3. Laravel renders an Inertia page that fetches schema from **Go** `GET /forms/:id/schema` (public) and renders the form with Form.io.
4. On submit, the browser POSTs to **Go** `POST /forms/:id/submit` (public). Laravel does not proxy submissions.
5. Success and validation/error UX is handled on the same Laravel page (e.g. thank-you view, inline errors).

### Landing Page

- Port or reimplement from goforms `Home.vue` (and any backend that served it) into Laravel.
- Use Laravel's guest layout; link to Fortify login/register and dashboard.
- No Go dependency.

### Production Routing

- Laravel owns `/` and `/forms/{id}`.
- Reverse proxy must send `/forms/:id/embed`, `/forms/:id/schema`, `/forms/:id/submit` to Go; all other `/forms/*` and `/` go to Laravel so the public runtime stays in Go.

---

## 3. Go Scope (Unchanged)

| Route / group | Purpose |
|---------------|--------|
| `GET /forms/:id/embed` | Iframe target: minimal HTML, Form.io from CDN, schema from same origin, posts to `/forms/:id/submit`. |
| `GET /forms/:id/schema` | Public schema JSON (embed + Laravel standalone page). |
| `GET /forms/:id/validation` | Public validation schema. |
| `POST /forms/:id/submit` | Public form submission (embed + Laravel standalone page). |
| `GET/POST/PUT/DELETE /api/forms/*` | Authenticated API (assertion headers; Laravel only). |

- No new routes, no changes to request/response shapes, no changes to CORS, rate limiting, or public runtime behavior.
- Laravel's standalone form-fill page uses the same public schema and submit endpoints as the embed runtime; authenticated dashboard/builder uses `/api/forms/*`.

---

## 4. Error Handling and Edge Cases

### Laravel — Standalone Form-Fill (`/forms/{id}`)

| Scenario | Laravel behavior |
|----------|------------------|
| Form not found (invalid id, deleted) | Go returns 404 for schema → show "Form not found" (or 404 page). |
| Go unreachable / 5xx when fetching schema | Show "Form temporarily unavailable"; optional retry. |
| Submit: 422 validation errors | Go returns 422 with field errors; Form.io/Vue surfaces them in the form. |
| Submit: 429 rate limit | Show "Too many submissions" and optional retry. |
| Submit: 404 (e.g. form deleted) | Show "Form no longer available". |
| Submit: Go unreachable / 5xx | Show "Submission failed, please try again"; optional retry. |

### Laravel — Landing (`/`)

- No Go dependency; standard Laravel/Fortify handling.

### Go

- Existing behavior (404, 422, 429, 5xx) and response shapes unchanged.

### Edge Cases

- **CORS:** Browser on Laravel's origin calls Go for schema and submit. If Go is on a different origin, Go's CORS config must allow Laravel's origin for those public endpoints (same as for embed).
- **Draft vs published:** Align with Go's existing rules (e.g. 404 for draft schema if applicable).
- **Form id format:** Laravel `/forms/{id}` uses the same identifier as Go so schema and submit calls are consistent.

---

## 5. Routing Summary and Testing

### Routing Summary

| Owner | Method | Path | Purpose |
|-------|--------|------|--------|
| Laravel | GET | `/` | Home/landing |
| Laravel | GET | `/forms/{id}` | Standalone form-fill |
| Laravel | (auth) | `/dashboard`, `/forms`, `/forms/{id}/edit`, etc. | Authenticated UI |
| Go | GET | `/forms/:id/embed` | Iframe runtime |
| Go | GET | `/forms/:id/schema` | Public schema |
| Go | GET | `/forms/:id/validation` | Public validation |
| Go | POST | `/forms/:id/submit` | Public submit |
| Go | * | `/api/forms/*` | Authenticated API |

### Production Reverse Proxy

- **Laravel:** `/`, `/forms/{id}` (standalone only), `/dashboard`, `/login`, `/register`, and all other Laravel routes. Do **not** send `/forms/:id/embed`, `/forms/:id/schema`, `/forms/:id/validation`, or `/forms/:id/submit` to Laravel.
- **Go:** `/forms/:id/embed`, `/forms/:id/schema`, `/forms/:id/validation`, `/forms/:id/submit`, `/api/forms/*`.

### Testing

- **Laravel:** Feature tests for `GET /` (landing, 200, no Go calls) and `GET /forms/{id}` (valid id → form page; 404 from Go or missing → "Form not found"). Optionally test submit flow (success, 422, 429, 5xx) with mocked or real Go. Use existing Pest/Inertia patterns.
- **Go:** Existing tests for embed, schema, validation, submit, and `/api/forms/*` remain. Add or adjust CORS tests if Laravel's origin is added to allowed origins.
