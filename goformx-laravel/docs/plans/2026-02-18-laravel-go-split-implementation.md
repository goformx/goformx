# Laravel + Go Split Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor goforms to API-only (trusting Laravel-signed assertions) and build goformx-laravel as the frontend that calls Go for form operations.

**Architecture:** Laravel owns auth + UI; Go owns form domain. Laravel proves identity via HMAC-signed headers. Go exposes `/api/forms/*` (authenticated) and `/forms/:id/embed`, `/forms/:id/submit` (public). See design doc: `docs/plans/2026-02-18-goformx-laravel-go-split-design.md`.

**Tech Stack:** Laravel 12, Inertia + Vue, Fortify; Go, Echo, GORM, PostgreSQL.

---

## Phase 1: Go — Assertion middleware and API routes

### Task 1.1: Add assertion verification middleware (Go)

**Files:**
- Create: `goforms/internal/application/middleware/assertion/assertion.go`
- Modify: `goforms/internal/infrastructure/config/` (add assertion config)

**Step 1: Add config for shared secret and timestamp skew**

Add to config struct (or viper defaults):
- `security.assertion.secret` from `GOFORMS_SHARED_SECRET`
- `security.assertion.timestamp_skew_seconds` (default 60)

**Step 2: Create assertion middleware**

```go
// assertion.go - verifies X-User-Id, X-Timestamp, X-Signature
// On success: c.Set("user_id", userID)
// On failure: return 401 JSON
```

**Step 3: Add test**

`goforms/internal/application/middleware/assertion/assertion_test.go` — test valid signature, invalid signature, missing headers, stale timestamp.

**Step 4: Run tests**

```bash
cd goforms && go test ./internal/application/middleware/assertion/... -v
```

**Step 5: Commit**

```bash
git add internal/application/middleware/assertion/ config/
git commit -m "feat(go): add assertion verification middleware"
```

---

### Task 1.2: Create /api/forms route group with assertion auth (Go)

**Files:**
- Modify: `goforms/internal/application/handlers/web/form_api.go`
- Modify: `goforms/internal/application/constants/constants.go` (if needed)

**Step 1: Add authenticated route group using assertion middleware**

Create `/api/forms` group that uses assertion middleware (not session/access manager) for:
- GET `/api/forms` — list forms
- POST `/api/forms` — create form
- GET `/api/forms/:id` — get form
- PUT `/api/forms/:id` — update form
- DELETE `/api/forms/:id` — delete form
- GET `/api/forms/:id/submissions` — list submissions
- GET `/api/forms/:id/submissions/:sid` — get submission

**Step 2: Keep public routes separate**

Public routes (schema, submit) remain under `/api/v1/forms` or a separate `/forms` path without assertion. Ensure no double registration of same path.

**Step 3: Update FormAPIHandler.RegisterRoutes**

Wire assertion middleware into authenticated group; keep public routes without assertion.

**Step 4: Run tests**

```bash
go test ./internal/application/handlers/web/... -v -count=1
```

**Step 5: Commit**

```bash
git add internal/application/handlers/web/form_api.go
git commit -m "feat(go): add /api/forms with assertion auth"
```

---

## Phase 2: Go — Remove Inertia, auth, and web handlers

### Task 2.1: Remove FormWebHandler and PageHandler (Go)

**Files:**
- Modify: `goforms/internal/application/module.go` (remove FormWebHandler, PageHandler from FX)
- Modify: `goforms/internal/infrastructure/module.go` (if handlers registered there)
- Delete or deprecate: `goforms/internal/application/handlers/web/form_web.go` usage

**Step 1: Remove handler registration**

Remove FormWebHandler, PageHandler from the handlers FX group. Ensure FormAPIHandler remains.

**Step 2: Remove Inertia dependency from module**

Remove inertia.EchoHandler, presentation module wiring for Inertia pages.

**Step 3: Run build**

```bash
cd goforms && go build ./...
```

**Step 4: Commit**

```bash
git add internal/application/module.go internal/infrastructure/module.go
git commit -m "refactor(go): remove FormWebHandler and PageHandler"
```

---

### Task 2.2: Remove AuthHandler and asset server (Go)

**Files:**
- Modify: `goforms/internal/application/module.go`
- Modify: `goforms/internal/infrastructure/server/server.go`

**Step 1: Remove AuthHandler**

Auth is now Laravel’s responsibility. Remove login, register, session routes.

**Step 2: Remove asset server registration**

Laravel serves frontend assets. Remove Vite/static asset routes from Go server.

**Step 3: Run build and smoke test**

```bash
go build ./... && ./goforms  # or task dev:backend, verify API responds
```

**Step 4: Commit**

```bash
git add internal/application/module.go internal/infrastructure/server/server.go
git commit -m "refactor(go): remove AuthHandler and asset server"
```

---

## Phase 3: Laravel — Go client and config

### Task 3.1: Add GoForms config and HTTP client (Laravel)

**Files:**
- Modify: `goformx-laravel/config/services.php`
- Create: `goformx-laravel/app/Services/GoFormsClient.php`

**Step 1: Add config**

```php
// config/services.php
'goforms' => [
    'url' => env('GOFORMS_API_URL', 'http://localhost:8090'),
    'secret' => env('GOFORMS_SHARED_SECRET'),
],
```

**Step 2: Add .env.example keys**

```
GOFORMS_API_URL=http://localhost:8090
GOFORMS_SHARED_SECRET=
```

**Step 3: Create GoFormsClient**

- Constructor: base URL, secret
- Method `withUser(User $user)` returns a scoped client that signs requests
- Private method `signRequest(Request $request, User $user)` — add X-User-Id, X-Timestamp, X-Signature
- Methods: `listForms()`, `getForm($id)`, `createForm($data)`, `updateForm($id, $data)`, `deleteForm($id)`, `listSubmissions($formId)`, `getSubmission($formId, $submissionId)`

**Step 4: Write unit test**

Mock HTTP, assert headers include X-User-Id, X-Timestamp, X-Signature.

**Step 5: Run tests**

```bash
cd goformx-laravel && php artisan test --filter=GoFormsClient
```

**Step 6: Commit**

```bash
git add config/services.php app/Services/GoFormsClient.php tests/
git commit -m "feat(laravel): add GoFormsClient with signed assertions"
```

---

### Task 3.2: Add form controller and routes (Laravel)

**Files:**
- Create: `goformx-laravel/app/Http/Controllers/FormController.php`
- Modify: `goformx-laravel/routes/web.php`

**Step 1: Create FormController**

- `index()` — list forms, call GoFormsClient::listForms(), render Inertia Dashboard or Forms/Index
- `show($id)` — get form, render Forms/Edit with form prop
- `store(Request)` — create form, redirect
- `update(Request, $id)` — update form, redirect
- `destroy($id)` — delete form, redirect

Use GoFormsClient, catch HTTP exceptions, map to flash/validation.

**Step 2: Add routes**

```php
Route::middleware(['auth'])->group(function () {
    Route::get('/forms', [FormController::class, 'index']);
    Route::post('/forms', [FormController::class, 'store']);
    Route::get('/forms/{id}', [FormController::class, 'show']);
    Route::put('/forms/{id}', [FormController::class, 'update']);
    Route::delete('/forms/{id}', [FormController::class, 'destroy']);
});
```

**Step 3: Feature test**

`tests/Feature/FormControllerTest.php` — mock GoFormsClient or use a fake, assert Inertia response structure.

**Step 4: Run tests**

```bash
php artisan test --filter=Form
```

**Step 5: Commit**

```bash
git add app/Http/Controllers/FormController.php routes/web.php tests/Feature/FormControllerTest.php
git commit -m "feat(laravel): add FormController and routes"
```

---

## Phase 4: Laravel — Port form builder UI

### Task 4.1: Port form builder page and composables (Laravel)

**Files:**
- Create: `goformx-laravel/resources/js/pages/Forms/Edit.vue`
- Create: `goformx-laravel/resources/js/composables/useFormBuilder.ts`
- Port: Form.io integration, BuilderLayout, FieldsPanel, FieldSettingsPanel from goforms

**Step 1: Copy Vue components**

Copy from goforms to goformx-laravel: Form builder layout, fields panel, field settings panel. Adjust imports for Laravel path aliases.

**Step 2: Port useFormBuilder composable**

Adapt to call Laravel routes (which proxy to Go). Change API base from direct Go to Laravel endpoints.

**Step 3: Create Forms/Edit.vue**

Inertia page that receives `form` prop from FormController::show, renders builder, saves via Inertia POST to Laravel update route.

**Step 4: Add FormController::update schema handling**

Accept schema in request, call GoFormsClient::updateForm with schema. Map Go 422 to Inertia validation.

**Step 5: Manual test**

Run Laravel + Go, log in, open form edit, drag field, save. Verify schema persists in Go DB.

**Step 6: Commit**

```bash
git add resources/js/pages/Forms/ resources/js/composables/ app/Http/Controllers/FormController.php
git commit -m "feat(laravel): port form builder UI"
```

---

### Task 4.2: Add dashboard forms list and navigation (Laravel)

**Files:**
- Modify: `goformx-laravel/resources/js/pages/Dashboard/Index.vue` (or create Forms/Index.vue)
- Modify: layout/nav to include Forms link

**Step 1: Create Forms/Index page**

Grid of forms (FormCard), load from FormController::index. Add "New form" button.

**Step 2: Add dashboard or forms index route**

Wire FormController::index to Forms/Index or Dashboard with forms.

**Step 3: Add nav link**

Link to /forms in main nav.

**Step 4: Run frontend build**

```bash
npm run build
```

**Step 5: Commit**

```bash
git add resources/js/pages/Forms/Index.vue resources/js/components/ resources/js/layouts/
git commit -m "feat(laravel): add forms list and navigation"
```

---

## Phase 5: Public embed and submit (Go)

### Task 5.1: Ensure public routes are accessible (Go)

**Files:**
- Modify: `goforms/internal/application/handlers/web/form_api.go`
- Review: `goforms/internal/application/constants/constants.go`

**Step 1: Verify public route paths**

Ensure `/forms/:id/schema`, `/forms/:id/submit` (or equivalent) exist and do not require assertion.

**Step 2: Add embed route if missing**

GET `/forms/:id/embed` — return HTML/JS or schema for embedding. CORS enabled.

**Step 3: Integration test**

```bash
curl -X POST http://localhost:8090/api/v1/forms/{id}/submit -H "Content-Type: application/json" -d '{}'
# Expect 422 or 200 depending on form
```

**Step 4: Commit**

```bash
git add internal/application/handlers/web/form_api.go
git commit -m "feat(go): ensure public embed and submit routes"
```

---

## Phase 6: Error handling and polish

### Task 6.1: Map Go errors to Laravel responses

**Files:**
- Modify: `goformx-laravel/app/Services/GoFormsClient.php`
- Modify: `goformx-laravel/app/Http/Controllers/FormController.php`

**Step 1: Handle 502/503**

When Go is unreachable, throw or return meaningful exception; FormController catches and flashes "Form service temporarily unavailable".

**Step 2: Handle 422**

Map Go validation JSON to Laravel validation format; pass to Inertia.

**Step 3: Handle 404**

Redirect to forms list or 404 page.

**Step 4: Test error paths**

Mock GoClient to return 502, 422, 404; assert correct user-facing behavior.

**Step 5: Commit**

```bash
git add app/Services/GoFormsClient.php app/Http/Controllers/FormController.php
git commit -m "feat(laravel): map Go errors to user-facing responses"
```

---

## Execution Handoff

Plan complete and saved to `docs/plans/2026-02-18-laravel-go-split-implementation.md`.

**Two execution options:**

1. **Subagent-driven (this session)** — Use superpowers:subagent-driven-development; dispatch fresh subagent per task, review between tasks, fast iteration.

2. **Parallel session (separate)** — Open a new session in a worktree, use superpowers:executing-plans for batch execution with checkpoints.
