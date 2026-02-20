# Form Builder and Management Move to Laravel — Design

**Status:** Approved  
**Date:** 2026-02-18

## Summary

Move all form builder and management UI from goforms (Go + Vue) to goformx-laravel (Laravel + Inertia/Vue), then remove the moved functionality from goforms. Laravel gains full parity: form list, create, edit (builder + settings), in-place and shareable preview, submissions list/detail, and embed page. Go becomes API-only: assertion-authenticated `/api/forms/*` and public `/forms/:id/embed|schema|submit`.

---

## 1. Architecture and Boundaries

### Ownership

| Concern | Owner |
|--------|--------|
| Form management UI (list, create, edit, preview, submissions, embed page) | Laravel |
| Auth (Fortify), sessions, user DB | Laravel |
| Form API (CRUD, schema, submissions), public embed/submit | Go |
| Form DB | Go |

### Traffic

- **Dashboard, list, edit, preview, submissions, embed (management):** Browser → Laravel → Laravel calls Go with signed assertion headers.
- **Public fill:** Browser → Laravel `GET /forms/{id}` (Fill.vue) or direct to Go for embed.
- **Public embed / submit:** Browser → Go `/forms/:id/embed`, `/forms/:id/schema`, `/forms/:id/submit` (unchanged).

### Formio Repo

- Remains the source for the `@goformx/formio` npm package. No code changes in the formio repo for this move. Laravel continues to depend on it; goforms will no longer use it after cleanup.

---

## 2. Laravel Additions

### New Routes (auth required)

| Method | Path | Action |
|--------|------|--------|
| GET | `/forms/{id}/preview` | Shareable preview page |
| GET | `/forms/{id}/submissions` | Submissions list |
| GET | `/forms/{id}/submissions/{sid}` | Submission detail |
| GET | `/forms/{id}/embed` | Embed page (iframe snippet, URL) |

### Controllers

- **FormController** (extend existing): add `preview(string $id)`, `submissions(string $id)`, `submission(string $id, string $sid)`, `embed(string $id)`. Each uses `GoFormsClient->withUser(auth()->user())`, then `Inertia::render` with appropriate page and props. Reuse existing `handleGoError` and not-found behavior.

### Pages (Vue/Inertia)

- **Forms/Preview.vue** — Full-page read-only Form.io render; form + schema from props. Used for shareable URL and linked from Edit.
- **Forms/Submissions.vue** — List: form title + submissions (id, submitted_at, status, optional data preview). Links to submission detail. Props: `form`, `submissions`.
- **Forms/SubmissionShow.vue** — Single submission detail (data as key/value or JSON). Props: `form`, `submission`. Breadcrumb: Forms → form → Submissions → submission.
- **Forms/Embed.vue** — Embed management: embed URL, iframe snippet, copy button. Props: `form`. Embed URL is the Go public URL (e.g. `config('services.goforms.public_url')` + `/forms/{id}/embed`).

### Edit Page

- **Forms/Edit.vue** (existing): Add in-place preview (tab or toggle). When active, render current schema in read-only Form.io in a panel; schema from builder state or saved form. Keep links to shareable `GET /forms/{id}/preview`, `GET /forms/{id}/submissions`, and `GET /forms/{id}/embed`.

### Data and Errors

- All form/submission data comes from Go via `GoFormsClient`. Same error mapping as today: Go 404 → 404, 422 → validation, 5xx/unreachable → “Form service temporarily unavailable.”

---

## 3. Go (goforms) Cleanup

### Remove

1. **Frontend app** — Delete or cease using: `src/` (Vue pages, components, composables, assets, `main.ts`), SPA entry (e.g. `index.html`). Remove Vite config, frontend build scripts, and frontend npm deps (e.g. `vue`, `@inertiajs/vue3`, `@formio/js`, `@goformx/formio`). Remove or simplify `package.json` if no remaining frontend need.
2. **Asset / SPA serving** — Remove `DevelopmentAssetServer` (and any production static server) that serves the Vue app, `dist/`, and Form.io fonts/routes. Unregister from Echo so Go no longer serves HTML or frontend assets.
3. **Session-based form API** — Remove `RegisterAuthenticatedRoutes` (the `/api/v1/forms` group with session/access middleware). Only assertion-based `/api/forms/*` remains for authenticated form operations.
4. **Auth UI / session handlers** — Remove any handlers or routes that served login/register/session pages or redirects for the old goforms UI (e.g. `AuthHandler`). Remove middleware that only supported that UI.
5. **Unused deps and config** — Remove FX/DI providers or config only used for the Vue app, Inertia, or asset server. Keep CORS/CSRF for public embed/submit. Clean `main.go` so it only wires API and public routes.

### Keep

- Form domain (services, repository, model, migrations).
- **FormAPIHandler:** `RegisterLaravelRoutes` (`/api/forms` with assertion) and `RegisterPublicFormsRoutes` (`/forms/:id/embed`, schema, validation, submit). Keep public routes for embed and submit.
- Assertion middleware, CORS for embed, response builders, validation, error handling.
- Config: `GOFORMS_SHARED_SECRET`, DB, CORS, API key if used.

### Result

- Go is a single binary: HTTP server with `/api/forms/*` (assertion) and `/forms/:id/embed|schema|validation|submit`. No HTML, no static assets, no session auth.

---

## 4. Formio Repo and npm

- Formio repo: no changes. Laravel keeps `@goformx/formio` and `@formio/js` in `package.json`. goforms drops these when the frontend is removed.
- Publish `@goformx/formio` from the formio repo as today; Laravel installs from registry or workspace path. No new packages or workspace wiring required.

---

## 5. Testing and Rollout

- **Laravel:** Add/update feature tests for new routes (preview, submissions list, submission detail, embed). Mock or fake `GoFormsClient` where appropriate. Optional smoke tests against a running Go API.
- **Go:** After cleanup, keep tests for FormAPIHandler (assertion and public routes). Remove or adjust tests that depend on session auth, the Vue app, or the asset server.
- **Rollout:** Deploy Laravel with new pages first; verify list, edit, preview, submissions, embed against current Go. Then deploy stripped Go (no frontend, no `/api/v1` session routes). No DB migrations required for this move.

---

## Approach

**Approach 1 (chosen):** Implement all missing Laravel surface, verify parity, then remove goforms frontend and session routes in a single cleanup. No incremental feature-by-feature migration.
