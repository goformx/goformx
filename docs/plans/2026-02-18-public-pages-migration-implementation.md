# Public Pages Migration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Migrate public pages from goforms to Laravel: landing page at `/` and standalone form-fill page at `/forms/{id}`. Go keeps the embed runtime and public API; Laravel serves first-party UI only.

**Architecture:** Laravel serves `/` (landing) and public `GET /forms/{id}` (form-fill). Standalone form-fill loads schema from Go `GET /forms/:id/schema` and submits to Go `POST /forms/:id/submit` from the browser; Laravel does not proxy. Authenticated form builder moves to `GET /forms/{id}/edit`. See design: `docs/plans/2026-02-18-public-pages-migration-design.md`.

**Tech Stack:** Laravel 12, Inertia + Vue, Form.io (for form-fill), Pest, Go (unchanged).

---

## Phase 1: Route and URL alignment

### Task 1.1: Move authenticated form "show" to `/forms/{id}/edit`

**Files:**
- Modify: `routes/web.php`
- Modify: `app/Http/Controllers/FormController.php` (rename `show` → `edit` or keep `show` and add route)
- Modify: any views/links that reference `forms.show` (e.g. redirect after store, Forms/Index links)

**Step 1: Add route `forms/{id}/edit` and point to FormController::edit**

In `routes/web.php`, inside the auth group, replace:
- `Route::get('forms/{id}', [FormController::class, 'show'])->name('forms.show');`
with:
- `Route::get('forms/{id}/edit', [FormController::class, 'edit'])->name('forms.edit');`

Add a temporary duplicate route for `forms/{id}` in the auth group that redirects to `forms.edit` so existing links keep working, or update all references in one go (see steps below).

**Step 2: Rename controller method `show` to `edit`**

In `FormController.php`, rename method `show` to `edit`. Update the method signature to accept `string $id` and return the same Inertia response for `Forms/Edit`.

**Step 3: Update redirect after form store**

In `FormController::store`, change `redirect()->route('forms.show', $formId)` to `redirect()->route('forms.edit', $formId)`.

**Step 4: Update Forms/Index and any other links**

Search for `forms.show` or `route('forms.show', ...)` and replace with `forms.edit` and `route('forms.edit', ...)` (e.g. in `resources/js/pages/Forms/Index.vue`, any Blade or Vue links to "edit" form).

**Step 5: Remove auth route for `forms/{id}`**

Ensure the only auth route for a single form is `forms/{id}/edit` (name `forms.edit`). Remove any remaining `forms.show` route.

**Step 6: Run tests**

Run: `php artisan test --compact --filter=Form`
Fix any failing tests (expectations for route names or redirects).

**Step 7: Commit**

```bash
git add routes/web.php app/Http/Controllers/FormController.php resources/js/pages/Forms/
git commit -m "refactor: move form builder to /forms/{id}/edit"
```

---

## Phase 2: Landing page

### Task 2.1: Ensure landing page at `/` meets design

**Files:**
- Modify or keep: `routes/web.php` (already has `Route::get('/', ...)`)
- Modify: `resources/js/pages/Welcome.vue` (or create `Home.vue` and render that)

**Step 1: Decide Welcome vs Home**

If `Welcome.vue` already provides hero, features, and auth links (login, register, dashboard), keep it and optionally align copy/styling with goforms `Home.vue`. If not, create `resources/js/pages/Home.vue` by porting content from `goforms/src/pages/Home.vue` (hero, features list, CTAs) and use Laravel guest layout patterns and `@/routes` for login/register/dashboard.

**Step 2: Point `/` to the chosen page**

In `routes/web.php`, ensure `Route::get('/', ...)` returns `Inertia::render('Welcome', ...)` or `Inertia::render('Home', ...)` with `canRegister` from Fortify.

**Step 3: Write a simple feature test for landing**

Create or extend test: `tests/Feature/LandingPageTest.php` (or in an existing test). Assert `GET /` returns 200 and that the response contains expected content (e.g. Inertia page or text like "Log in" / "Register"). No Go calls.

Example (Pest):

```php
// tests/Feature/LandingPageTest.php
use function Pest\get;

it('returns landing page', function () {
    get('/')->assertOk();
});
```

**Step 4: Run test**

Run: `php artisan test --compact tests/Feature/LandingPageTest.php`
Expected: PASS.

**Step 5: Commit**

```bash
git add routes/web.php resources/js/pages/ tests/Feature/
git commit -m "feat: landing page at /"
```

---

## Phase 3: Public standalone form-fill page

### Task 3.1: Expose Go public URL to frontend

**Files:**
- Modify: `app/Http/Middleware/HandleInertiaRequests.php` (or equivalent shared data provider)
- Check: `config/services.php` already has `goforms.url`

**Step 1: Share Go forms public base URL with Inertia**

So the standalone form-fill page can call `GET {goFormsUrl}/forms/{id}/schema` and `POST {goFormsUrl}/forms/{id}/submit`, add a shared Inertia prop (e.g. `goFormsPublicUrl` or reuse a single `goforms.url` key). In `HandleInertiaRequests::share()`, add something like:

```php
'goFormsPublicUrl' => config('services.goforms.url'),
```

Ensure this is only the origin (no trailing slash) so the frontend can build `/forms/:id/schema` and `/forms/:id/submit`.

**Step 2: Run existing tests**

Run: `php artisan test --compact`
Expected: no regressions.

**Step 3: Commit**

```bash
git add app/Http/Middleware/HandleInertiaRequests.php
git commit -m "feat: share Go forms public URL with frontend"
```

---

### Task 3.2: Add public route and controller for form-fill

**Files:**
- Create: `app/Http/Controllers/PublicFormController.php` (or `FormFillController.php`)
- Modify: `routes/web.php`

**Step 1: Write failing feature test**

Create `tests/Feature/PublicFormFillTest.php`. Use Pest. Mock HTTP to Go or use a test double so that when the app tries to verify form existence or load schema, you control 404 vs 200.

Example (adjust to your approach — e.g. no server-side schema fetch, only client):

```php
// tests/Feature/PublicFormFillTest.php
use function Pest\get;

it('returns form fill page for valid form id', function () {
    // If you do a server-side head check to Go, mock 200 for that.
    get('/forms/some-id')->assertOk();
});

it('returns 404 or form not found when Go returns 404', function () {
    // Mock Go 404 for schema or existence check.
    get('/forms/nonexistent')->assertStatus(404);
});
```

Run test: expect failure (route or controller missing).

**Step 2: Add public route**

In `routes/web.php`, add **before** the auth group (so it takes precedence for unauthenticated users):

```php
Route::get('forms/{id}', [PublicFormController::class, 'show'])->name('forms.fill');
```

Use the correct controller name and method. Ensure there is no conflicting `forms/{id}` in the auth group (you moved that to `forms/{id}/edit` in Phase 1).

**Step 3: Create PublicFormController**

Create `app/Http/Controllers/PublicFormController.php`. In `show(string $id)`:

- Option A: Call Go (e.g. `GET {goforms.url}/forms/{id}/schema`) to verify form exists; if 404 or 5xx, return 404 or 503 with a friendly view/message. Then render Inertia page with `formId: $id` and any props (e.g. `goFormsPublicUrl` is already shared).
- Option B: Do not call Go from server; render Inertia with `formId: $id` and let the client fetch schema; handle 404/5xx on the client and show "Form not found" / "Form temporarily unavailable."

Choose one and implement. Design: "optionally check that the form exists (e.g. via Go or a lightweight existence check)." So either A or B is valid; document in controller or plan which you use.

**Step 4: Run test again**

Run: `php artisan test --compact tests/Feature/PublicFormFillTest.php`
Expected: PASS (or adjust test to match behavior).

**Step 5: Commit**

```bash
git add routes/web.php app/Http/Controllers/PublicFormController.php tests/Feature/PublicFormFillTest.php
git commit -m "feat: add public form-fill route and controller"
```

---

### Task 3.3: Build Inertia + Form.io form-fill page

**Files:**
- Create: `resources/js/pages/Forms/Fill.vue` (or `PublicForm.vue`)
- Modify: `app/Http/Controllers/PublicFormController.php` (ensure it passes `formId` and any needed props)

**Step 1: Create Vue page component**

Create `resources/js/pages/Forms/Fill.vue`. It should:

- Accept props: `formId` (string), and optionally `formTitle` if you pass it from the server.
- Use shared `goFormsPublicUrl` to build:
  - Schema URL: `${goFormsPublicUrl}/forms/${formId}/schema`
  - Submit URL: `${goFormsPublicUrl}/forms/${formId}/submit`
- On mount, fetch schema from schema URL. On 404 → show "Form not found". On 5xx or network error → show "Form temporarily unavailable" (and optional retry).
- Render form with Form.io (same approach as goforms: Formio.createForm(container, schema, { submit: submitUrl }). Handle submit success (e.g. thank-you message). Handle 422 (validation) by letting Form.io show errors; handle 429 → "Too many submissions"; 404 on submit → "Form no longer available"; 5xx → "Submission failed, please try again."
- Use a simple layout (guest or minimal) so the page is first-party and branded.

Reference goforms embed HTML and `Forms/Preview.vue` for Form.io usage; avoid embedding in iframe — this is a normal page.

**Step 2: Ensure controller passes formId**

In `PublicFormController::show`, pass `formId` (and optionally title) to the Inertia page:

```php
return Inertia::render('Forms/Fill', [
    'formId' => $id,
]);
```

**Step 3: Manual or E2E check**

Run `npm run dev` and open `/forms/{valid-id}` (with Go running and a real form id). Confirm schema loads, form renders, submit works and error messages match design.

**Step 4: Commit**

```bash
git add resources/js/pages/Forms/Fill.vue app/Http/Controllers/PublicFormController.php
git commit -m "feat: add standalone form-fill page with Form.io"
```

---

### Task 3.4: Harden error handling and tests

**Files:**
- Modify: `resources/js/pages/Forms/Fill.vue` (all 404, 422, 429, 5xx cases)
- Modify: `tests/Feature/PublicFormFillTest.php`

**Step 1: Add tests for error paths**

In `PublicFormFillTest.php`, add or extend tests (using HTTP fake or test server) so that when Go returns 404 for schema, the user sees "Form not found" or 404; when Go is unreachable, user sees "Form temporarily unavailable" (or 503). Cover at least one success path and one 404 path.

**Step 2: Run tests**

Run: `php artisan test --compact tests/Feature/PublicFormFillTest.php`
Expected: PASS.

**Step 3: Run Pint**

Run: `vendor/bin/pint --dirty --format agent`

**Step 4: Commit**

```bash
git add tests/Feature/PublicFormFillTest.php resources/js/pages/Forms/Fill.vue
git commit -m "test: public form-fill error handling"
```

---

## Phase 4: Documentation and CORS

### Task 4.1: Document reverse proxy and CORS

**Files:**
- Modify: `docs/plans/2026-02-18-public-pages-migration-design.md` (optional: add "Deployment" subsection)
- Or: `README.md` / existing ops doc

**Step 1: Add deployment note**

Document that in production the reverse proxy must:

- Send to Laravel: `/`, `/forms/{id}` (only the standalone form-fill; ensure no conflict with Go paths), `/dashboard`, `/login`, `/register`, and all other Laravel routes.
- Send to Go: `/forms/:id/embed`, `/forms/:id/schema`, `/forms/:id/validation`, `/forms/:id/submit`, `/api/forms/*`.

Document that Go's CORS config must allow Laravel's origin (e.g. `APP_URL`) for the public schema and submit endpoints if the browser calls Go from Laravel's domain.

**Step 2: Commit**

```bash
git add docs/ README.md
git commit -m "docs: reverse proxy and CORS for public pages"
```

---

## Execution handoff

Plan complete and saved to `docs/plans/2026-02-18-public-pages-migration-implementation.md`.

**Two execution options:**

1. **Subagent-driven (this session)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.
2. **Parallel session (separate)** — Open a new session with the executing-plans skill and run through the plan with checkpoints.

Which approach do you want?
