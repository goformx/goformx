# Form Builder Move to Laravel — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Move all form builder and management UI from goforms to goformx-laravel (full parity: list, create, edit, preview in-place + shareable, submissions list/detail, embed page), then remove frontend and session-based API from goforms so Go is API-only.

**Architecture:** Laravel remains the only UI; it calls Go via GoFormsClient (signed assertion). New Laravel routes and Inertia pages for preview, submissions, embed. Edit page gains in-place preview. Go keeps only `/api/forms/*` (assertion) and `/forms/:id/embed|schema|submit`; strip Vue app, asset server, and `/api/v1` session routes.

**Tech Stack:** Laravel 12, Inertia v2, Vue 3, Form.io (@formio/js, @goformx/formio), Go 1.25, Echo, Uber FX. Design doc: `docs/plans/2026-02-18-form-builder-move-to-laravel-design.md`.

**Context:** Can be run in a dedicated worktree. Repos: goformx-laravel (Laravel), goforms (Go).

---

## Phase A — Laravel: New routes and controller methods

### Task A1: Add new form routes

**Files:**
- Modify: `goformx-laravel/routes/web.php`

**Step 1:** Add routes inside the `Route::middleware(['auth', 'verified'])->group` (after existing form routes):
- `GET forms/{id}/preview` → `FormController@preview` named `forms.preview`
- `GET forms/{id}/submissions` → `FormController@submissions` named `forms.submissions`
- `GET forms/{id}/submissions/{sid}` → `FormController@submission` named `forms.submissions.show`
- `GET forms/{id}/embed` → `FormController@embed` named `forms.embed`

**Step 2:** Run route list to verify.

Run: `cd goformx-laravel && php artisan route:list --name=forms`
Expected: New routes appear with correct names.

**Step 3:** Commit.

```bash
git add routes/web.php
git commit -m "feat(forms): add routes for preview, submissions, embed"
```

---

### Task A2: Add FormController preview() method

**Files:**
- Modify: `goformx-laravel/app/Http/Controllers/FormController.php`

**Step 1:** Add `preview(Request $request, string $id): Response|RedirectResponse`. Get form via `$this->goFormsClient->withUser(auth()->user())->getForm($id)`. On RequestException call `$this->handleGoError($e, $request)`; if form null throw `NotFoundHttpException`. Return `Inertia::render('Forms/Preview', ['form' => $form])`.

**Step 2:** Run tests (if any form controller tests exist).

Run: `cd goformx-laravel && php artisan test --compact --filter=Form`
Expected: Existing tests pass.

**Step 3:** Commit.

```bash
git add app/Http/Controllers/FormController.php
git commit -m "feat(forms): add FormController::preview"
```

---

### Task A3: Add FormController submissions() and submission() methods

**Files:**
- Modify: `goformx-laravel/app/Http/Controllers/FormController.php`

**Step 1:** Add `submissions(Request $request, string $id)`: get form (same as edit), then `$client->listSubmissions($id)`. Handle errors. Return `Inertia::render('Forms/Submissions', ['form' => $form, 'submissions' => $submissions])`.

**Step 2:** Add `submission(Request $request, string $id, string $sid)`: get form, then `$client->getSubmission($id, $sid)`. If submission null or not for this form, 404. Return `Inertia::render('Forms/SubmissionShow', ['form' => $form, 'submission' => $submission])`.

**Step 3:** Run tests.

Run: `cd goformx-laravel && php artisan test --compact --filter=Form`
Expected: Pass.

**Step 4:** Commit.

```bash
git add app/Http/Controllers/FormController.php
git commit -m "feat(forms): add FormController submissions list and show"
```

---

### Task A4: Add FormController embed() method

**Files:**
- Modify: `goformx-laravel/app/Http/Controllers/FormController.php`

**Step 1:** Add `embed(Request $request, string $id)`: get form (same as edit). Return `Inertia::render('Forms/Embed', ['form' => $form])`. Ensure config has a public embed base URL (e.g. `config('services.goforms.public_url')` or derive from `config('services.goforms.url')`) so the view can build embed URL and iframe snippet.

**Step 2:** If `config/services.php` does not have `goforms.public_url`, add it (optional key; fallback to `goforms.url` for same-origin).

**Step 3:** Commit.

```bash
git add app/Http/Controllers/FormController.php config/services.php
git commit -m "feat(forms): add FormController::embed and embed URL config"
```

---

## Phase B — Laravel: New Inertia pages

### Task B1: Create Forms/Preview.vue

**Files:**
- Create: `goformx-laravel/resources/js/pages/Forms/Preview.vue`

**Step 1:** Create page that accepts `form` prop (with `id`, `title`, `schema`). Use AppLayout, Head, breadcrumbs (Dashboard → Forms → form title → Preview). Mount Form.io form in read-only mode with schema from `form.schema` (same pattern as Fill.vue: Formio.createForm with schema object, no submit). Use `@formio/js` and `Formio.use(goforms)` (from @goformx/formio). Add link back to edit.

**Step 2:** Run frontend build.

Run: `cd goformx-laravel && npm run build`
Expected: Build succeeds.

**Step 3:** Commit.

```bash
git add resources/js/pages/Forms/Preview.vue
git commit -m "feat(forms): add Forms/Preview page"
```

---

### Task B2: Create Forms/Submissions.vue

**Files:**
- Create: `goformx-laravel/resources/js/pages/Forms/Submissions.vue`

**Step 1:** Accept `form`, `submissions` props. Use AppLayout, breadcrumbs (Dashboard → Forms → form title → Submissions). Table or list: submission id, submitted_at, status; link each row to `forms.submissions.show` with `id` and `sid`. Link back to form edit. Match existing UI patterns (e.g. FormCard, tables in the app).

**Step 2:** Run build.

Run: `cd goformx-laravel && npm run build`
Expected: Build succeeds.

**Step 3:** Commit.

```bash
git add resources/js/pages/Forms/Submissions.vue
git commit -m "feat(forms): add Forms/Submissions list page"
```

---

### Task B3: Create Forms/SubmissionShow.vue

**Files:**
- Create: `goformx-laravel/resources/js/pages/Forms/SubmissionShow.vue`

**Step 1:** Accept `form`, `submission` props. Breadcrumbs: Dashboard → Forms → form title → Submissions → Submission. Display submission data (e.g. key/value or JSON). Link back to submissions list.

**Step 2:** Run build.

Run: `cd goformx-laravel && npm run build`
Expected: Build succeeds.

**Step 3:** Commit.

```bash
git add resources/js/pages/Forms/SubmissionShow.vue
git commit -m "feat(forms): add Forms/SubmissionShow page"
```

---

### Task B4: Create Forms/Embed.vue

**Files:**
- Create: `goformx-laravel/resources/js/pages/Forms/Embed.vue`

**Step 1:** Accept `form` prop. Show embed URL (e.g. `{{ publicUrl }}/forms/{{ form.id }}/embed`) and iframe snippet (e.g. `<iframe src="..." ...></iframe>`). Copy-to-clipboard button. Use AppLayout and breadcrumbs. Add `publicUrl` via shared Inertia data or pass from controller (e.g. `embed_base_url` from config).

**Step 2:** Ensure controller passes `embed_base_url` or equivalent (from `config('services.goforms.public_url') ?? config('services.goforms.url')`).

**Step 3:** Run build.

Run: `cd goformx-laravel && npm run build`
Expected: Build succeeds.

**Step 4:** Commit.

```bash
git add resources/js/pages/Forms/Embed.vue app/Http/Controllers/FormController.php
git commit -m "feat(forms): add Forms/Embed page with URL and iframe snippet"
```

---

### Task B5: Add in-place preview to Forms/Edit.vue

**Files:**
- Modify: `goformx-laravel/resources/js/pages/Forms/Edit.vue`

**Step 1:** Add a tab or toggle for "Preview" (alongside existing builder). When active, render Form.io form in read-only mode in a panel using current schema (from `getSchema()` or saved form). Reuse same Form.io render pattern as Preview.vue. Ensure existing links to `forms.preview`, `forms.submissions`, and `forms.embed` use Wayfinder/named routes and point to the new routes.

**Step 2:** Run build and manual smoke test if possible.

Run: `cd goformx-laravel && npm run build`
Expected: Build succeeds.

**Step 3:** Commit.

```bash
git add resources/js/pages/Forms/Edit.vue
git commit -m "feat(forms): add in-place preview to Edit page"
```

---

## Phase C — Laravel: Tests

### Task C1: Feature tests for new form routes

**Files:**
- Create or modify: `goformx-laravel/tests/Feature/FormControllerTest.php` (or existing form test file)

**Step 1:** Write failing tests: authenticated user can access `forms.preview`, `forms.submissions`, `forms.submissions.show`, `forms.embed` for a given form (mock or fake GoFormsClient / HTTP to return form and submissions). Test 404 when form or submission missing. Use Laravel HTTP fake or a test double for GoFormsClient so tests do not call real Go.

**Step 2:** Run tests to verify they fail (or pass if already implemented).

Run: `cd goformx-laravel && php artisan test --compact --filter=Form`
Expected: Fail until controller methods and pages exist; then pass.

**Step 3:** If tests fail due to missing implementation, implementation was done in Phase A/B; run again and fix any assertion or mock issues until pass.

**Step 4:** Commit.

```bash
git add tests/Feature/FormControllerTest.php
git commit -m "test(forms): add feature tests for preview, submissions, embed"
```

---

## Phase D — Go (goforms): Remove frontend and session API

### Task D1: Remove session-based form routes from FormAPIHandler

**Files:**
- Modify: `goforms/internal/application/handlers/web/form_api.go`

**Step 1:** In `RegisterRoutes`, remove the call to `h.RegisterAuthenticatedRoutes(formsAPI)`. Remove the method `RegisterAuthenticatedRoutes` and its handlers (or leave method but unused). Ensure `RegisterLaravelRoutes` and `RegisterPublicFormsRoutes` remain. Remove `RegisterPublicRoutes(formsAPI)` if it only duplicated public schema/submit under `/api/v1`; if public submit is only under `/forms/:id/submit`, dropping `/api/v1` public routes is fine. Check that no tests or other code rely on `/api/v1/forms` GET/PUT schema.

**Step 2:** Run Go tests.

Run: `cd goforms && task test:backend` or `go test ./...`
Expected: Pass (adjust tests that hit session routes if any).

**Step 3:** Commit.

```bash
git add internal/application/handlers/web/form_api.go
git commit -m "refactor(go): remove session-based /api/v1/forms routes"
```

---

### Task D2: Remove asset server and SPA serving from Go

**Files:**
- Modify: `goforms/internal/infrastructure/web/server.go` (or wherever DevelopmentAssetServer is registered)
- Modify: `goforms/main.go` (or equivalent that wires Echo and asset server)

**Step 1:** Remove registration of DevelopmentAssetServer (and any production static server for the Vue app). Remove `registerFormioRoutes`, `registerStaticRoutes` for SPA/assets. Ensure Echo no longer serves `/`, `/assets/*`, Form.io fonts, or index.html. Keep only API and public form routes.

**Step 2:** Remove or simplify FX providers that provide the asset server. Clean `main.go` so no frontend lifecycle (Start/Stop for asset server) is invoked.

**Step 3:** Run Go tests and start server; verify `GET /api/forms` (with assertion) and `GET /forms/:id/embed` still work; verify no route for `/` or `/assets`.

Run: `cd goforms && task test:backend && task dev:backend` (briefly)
Expected: Tests pass; server starts; API and public routes respond; no SPA routes.

**Step 4:** Commit.

```bash
git add internal/infrastructure/web/server.go main.go
git commit -m "refactor(go): remove asset server and SPA serving"
```

---

### Task D3: Remove Vue app and frontend build from goforms

**Files:**
- Delete: `goforms/src/` (entire directory: pages, components, composables, assets, main.ts)
- Delete: `goforms/index.html` (if present), `goforms/vite.config.ts`
- Modify: `goforms/package.json` — remove frontend scripts and deps (vue, @inertiajs/vue3, @formio/js, @goformx/formio, vite, etc.) or remove package.json if repo becomes Go-only. If Taskfile references `task dev:frontend` or `npm run build`, remove or simplify those.

**Step 1:** Delete `src/`, `index.html`, `vite.config.ts`. Update `package.json` to remove frontend dependencies and scripts, or remove `package.json` and rely on formio repo for any npm usage.

**Step 2:** Update `goforms/Taskfile.yml` (or equivalent): remove `dev:frontend`, `build:frontend` if present; `task dev` should only start backend. Update README/CLAUDE if they describe the Vue app.

**Step 3:** Run Go tests.

Run: `cd goforms && task test:backend`
Expected: Pass.

**Step 4:** Commit.

```bash
git add -A goforms/
git commit -m "chore(go): remove Vue app and frontend build"
```

---

### Task D4: Remove auth UI handlers and session middleware for UI

**Files:**
- Modify: `goforms/internal/application/handlers/web/module.go` (or wherever handlers are registered)
- Search and remove: Any AuthHandler, PageHandler, or handler that served login/register/session pages. Remove from FX provide and from route registration.

**Step 1:** Identify and remove auth UI handlers and their routes. Remove middleware that only applied to the removed SPA (e.g. session auth for pages). Keep assertion middleware and CORS for API/embed.

**Step 2:** Run Go tests.

Run: `cd goforms && task test:backend`
Expected: Pass.

**Step 3:** Commit.

```bash
git add internal/application/handlers/web/module.go ...
git commit -m "refactor(go): remove auth UI and SPA-only handlers"
```

---

### Task D5: Go lint and final test

**Files:**
- None (verification)

**Step 1:** Run full lint and test.

Run: `cd goforms && task lint && task test:backend`
Expected: All pass.

**Step 2:** Commit any lint fixes if needed.

```bash
git add -A && git commit -m "chore(go): lint after frontend removal"
```

---

## Execution handoff

Plan complete and saved to `docs/plans/2026-02-18-form-builder-move-to-laravel-plan.md`.

**Two execution options:**

1. **Subagent-driven (this session)** — Dispatch a fresh subagent per task (or per phase), review between tasks, fast iteration.
2. **Parallel session (separate)** — Open a new session with executing-plans in a worktree, batch execution with checkpoints.

Which approach do you want?
