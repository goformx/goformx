# Demo Page (Laravel) and Goforms Cleanup – Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a first-party Demo page in Laravel at `GET /demo` that loads a real form from Go and submits to Go (stored submissions), using a config-driven demo form ID; then remove the old public Home and Demo UI from goforms and clean up dead constants, access rules, and references.

**Architecture:** Laravel serves `GET /demo` and renders an Inertia Demo page with `formId` from config; the page fetches schema from Go and submits to Go (same pattern as Forms/Fill). Go’s public API is unchanged. In goforms, delete Home.vue and Demo.vue, update Nav/Error and CSS, and remove PathHome/PathDemo from constants, PathManager, and access rules.

**Tech Stack:** Laravel 12, Inertia + Vue, Form.io, Pest (Laravel); Go, Vue, Testify (goforms).

---

## Part A: Laravel – Demo page and config

### Task A.1: Add demo form ID config

**Files:**
- Modify: `config/services.php`
- Modify: `.env.example` (if present)

**Step 1: Add config key**

In `config/services.php`, inside the `'goforms'` array, add:

```php
'demo_form_id' => env('GOFORMS_DEMO_FORM_ID'),
```

**Step 2: Document in .env.example**

Add a line (or block) such as:

```
# Optional: form ID for the /demo page. Create a form in the dashboard and paste its ID here.
# GOFORMS_DEMO_FORM_ID=
```

**Step 3: Commit**

```bash
git add config/services.php .env.example
git commit -m "config: add GOFORMS_DEMO_FORM_ID for demo page"
```

---

### Task A.2: Demo controller and route

**Files:**
- Create: `app/Http/Controllers/DemoController.php`
- Modify: `routes/web.php`

**Step 1: Create invokable controller**

Create `app/Http/Controllers/DemoController.php` that:
- Has a single `__invoke()` method.
- Reads `config('services.goforms.demo_form_id')`.
- If empty: `return Inertia::render('DemoUnconfigured');` (we will add the page in next task).
- If set: `return Inertia::render('Demo', ['formId' => $demoFormId]);`
- Use `Inertia` and `Inertia\Response`; no request validation needed.

**Step 2: Register route**

In `routes/web.php`, add before or after the home route:

```php
Route::get('demo', App\Http\Controllers\DemoController::class)->name('demo');
```

**Step 3: Run existing tests**

Run: `php artisan test --compact`
Expected: all pass (no new test for demo yet).

**Step 4: Commit**

```bash
git add app/Http/Controllers/DemoController.php routes/web.php
git commit -m "feat: add Demo controller and route"
```

---

### Task A.3: DemoUnconfigured page

**Files:**
- Create: `resources/js/pages/DemoUnconfigured.vue`

**Step 1: Create page**

Create a minimal Inertia page that:
- Uses `Head` with title e.g. "Demo – Not configured".
- Reuses the same layout pattern as Home (header with nav: Log in / Register / Dashboard).
- Main content: short message that the demo form is not set up; link to dashboard or home. Use `Link` and `@/routes` (e.g. `home()`, `login()`, `register()`, `dashboard()`).

**Step 2: Run build**

Run: `npm run build` (or ensure dev server runs).
Expected: no errors.

**Step 3: Commit**

```bash
git add resources/js/pages/DemoUnconfigured.vue
git commit -m "feat: add DemoUnconfigured page"
```

---

### Task A.4: Demo page (when configured)

**Files:**
- Create: `resources/js/pages/Demo.vue`
- Reference: `resources/js/pages/Forms/Fill.vue` for schema fetch and submit pattern

**Step 1: Create Demo.vue**

- Props: `formId` (string). Use shared `goFormsPublicUrl` from `$page.props` (or equivalent) for base URL.
- Layout: same public layout as Home (header with nav).
- Fetch schema from `{goFormsPublicUrl}/forms/{formId}/schema`, render with Form.io (same approach as Forms/Fill.vue).
- Submit to `POST {goFormsPublicUrl}/forms/{formId}/submit`. On success, show a message (e.g. "Thanks, we've received your response"). Handle validation and network errors.
- Optional: link "Create your own form" to dashboard or forms index.
- Do not hardcode schema; always use `formId` and Go endpoints.

**Step 2: Run build**

Run: `npm run build`
Expected: no errors.

**Step 3: Commit**

```bash
git add resources/js/pages/Demo.vue
git commit -m "feat: add Demo page with Form.io and Go submit"
```

---

### Task A.5: Feature test for demo route

**Files:**
- Create or modify: `tests/Feature/DemoPageTest.php`

**Step 1: Write test – configured**

With `GOFORMS_DEMO_FORM_ID` set (e.g. in test via `Config::set` or env), `GET /demo` returns 200 and Inertia component `Demo` with `formId`.

**Step 2: Write test – unconfigured**

With `GOFORMS_DEMO_FORM_ID` unset/empty, `GET /demo` returns 200 and Inertia component `DemoUnconfigured` (or whatever the controller renders).

**Step 3: Run tests**

Run: `php artisan test --compact tests/Feature/DemoPageTest.php`
Expected: PASS.

**Step 4: Commit**

```bash
git add tests/Feature/DemoPageTest.php
git commit -m "test: feature tests for demo page"
```

---

### Task A.6: Pint and final Laravel check

**Step 1: Run Pint**

Run: `vendor/bin/pint --dirty --format agent`

**Step 2: Run full test suite**

Run: `php artisan test --compact`
Expected: all pass.

**Step 3: Commit if any formatting changes**

```bash
git add -A && git status
# If changes: git commit -m "style: pint"
```

---

## Part B: goforms – Remove public Home/Demo UI and references

### Task B.1: Delete Home and Demo pages

**Files (goforms repo):**
- Delete: `src/pages/Home.vue`
- Delete: `src/pages/Demo.vue`

**Step 1: Delete files**

Remove the two files.

**Step 2: Verify build**

Run: `task build` or `npm run build` in goforms.
Expected: build succeeds (dynamic `./pages/**/*.vue` no longer includes these).

**Step 3: Commit**

```bash
git add -A
git commit -m "chore: remove Home and Demo pages (moved to Laravel)"
```

---

### Task B.2: Update Nav and Error links

**Files (goforms repo):**
- Modify: `src/components/shared/Nav.vue`
- Modify: `src/pages/Error.vue`

**Step 1: Nav.vue**

Find the link with `href="/"` (e.g. logo or "Home"). Change to `/login` or remove navigation (e.g. `href="#"` or span) so it does not target a removed page.

**Step 2: Error.vue**

Change `Link href="/"` to `Link href="/login"` (or another existing public route in goforms).

**Step 3: Verify**

Run: `task build` (or `npm run build`).
Expected: no errors.

**Step 4: Commit**

```bash
git add src/components/shared/Nav.vue src/pages/Error.vue
git commit -m "chore: update Nav and Error links after removing Home/Demo"
```

---

### Task B.3: Remove demo-only CSS

**Files (goforms repo):**
- Modify: `src/css/main.css`
- Delete: `src/css/components/demo_form.css` (if exists)
- Delete: `src/css/pages/demo/sections.css` (if exists)

**Step 1: Remove imports**

In `src/css/main.css`, remove the two lines that import `components/demo_form.css` and `pages/demo/sections.css`.

**Step 2: Delete CSS files**

Delete those two files if they exist and are not imported elsewhere.

**Step 3: Commit**

```bash
git add src/css/main.css
git add -u src/css/components/demo_form.css src/css/pages/demo/sections.css 2>/dev/null || true
git commit -m "chore: remove demo-only CSS imports and files"
```

---

### Task B.4: Remove PathHome and PathDemo from constants and PathManager

**Files (goforms repo):**
- Modify: `internal/application/constants/constants.go`
- Modify: `internal/application/constants/paths.go`

**Step 1: constants.go**

Remove the two constant definitions for `PathHome` and `PathDemo` (and their values `"/"` and `"/demo"`).

**Step 2: paths.go**

In the `PublicPaths` slice (or equivalent in `NewPathManager`), remove `PathHome` and `PathDemo`.

**Step 3: Run tests**

Run: `task test` or `go test ./...`
Expected: fix any compile errors (other files may reference these constants; next task updates access).

**Step 4: Commit**

```bash
git add internal/application/constants/constants.go internal/application/constants/paths.go
git commit -m "chore: remove PathHome and PathDemo constants"
```

---

### Task B.5: Remove PathHome and PathDemo from access rules and tests

**Files (goforms repo):**
- Modify: `internal/application/middleware/access/access.go`
- Modify: `internal/application/middleware/access/access_test.go`

**Step 1: access.go**

In `DefaultRules()`, remove the two rules that use `constants.PathHome` and `constants.PathDemo`.

**Step 2: access_test.go**

Remove or update any test cases that reference `PathHome` or `PathDemo` (e.g. the map entries that set those paths to `access.Public`, and any tests that assert on them). Keep tests that still make sense for other public paths.

**Step 3: Run tests and lint**

Run: `task test` and `task lint`
Expected: pass.

**Step 4: Commit**

```bash
git add internal/application/middleware/access/access.go internal/application/middleware/access/access_test.go
git commit -m "chore: remove Home and Demo from access rules and tests"
```

---

### Task B.6: Check CSRF/middleware for "/" or "/demo"

**Files (goforms repo):**
- Grep for: `"/"`, `"/demo"`, `PathHome`, `PathDemo` in `internal/`

**Step 1: Search**

Run: `grep -r 'PathHome\|PathDemo\|path == "/"\|\"/demo\"' internal/ --include='*.go'` (or equivalent). Fix any remaining references (e.g. in CSRF or middleware) by removing the branch or using a different constant.

**Step 2: Run tests and lint**

Run: `task test` and `task lint`
Expected: pass.

**Step 3: Commit if any changes**

```bash
git add -A
git status
# If changes: git commit -m "chore: remove remaining Home/Demo path references"
```

---

## Execution summary

- **Part A (Laravel):** Tasks A.1–A.6 in order. Run in goformx-laravel repo.
- **Part B (goforms):** Tasks B.1–B.6 in order. Run in goforms repo. Part B can be done after Part A or in parallel in a separate session.

After saving the plan, offer execution choice per writing-plans skill.
