# Demo Page (Laravel) and Goforms Public Pages Cleanup – Design

**Status:** Approved  
**Date:** 2026-02-18

## Summary

Add a first-party Demo page in Laravel at `GET /demo` that loads a real form from Go and submits to Go so submissions are stored (data collection for later use). Use a config-driven demo form ID; no Go changes. Then remove the old public Home and Demo UI from goforms and clean up dead constants, access rules, and references.

---

## 1. Scope and Outcomes

**Goals**

- **Laravel:** Add a Demo page at `GET /demo` that loads a real form from Go and submits to Go (stored submissions).
- **Laravel:** Keep existing public pages verified: `/` (Home) and `/forms/{id}` (form-fill).
- **goforms:** Remove public UI: Home and Demo pages, their path constants, access rules, and any code that only supported them.

**Out of scope**

- How demo data is monetized later.
- Changes to Go’s public API (schema/submit remain as-is).

**Approach: config-driven demo form ID (Approach A)**

- One “demo” form created in the app (e.g. via Laravel dashboard). Its ID is set in Laravel config (`GOFORMS_DEMO_FORM_ID`). Demo page uses that ID to load schema and submit to Go.
- No Go changes, no bootstrap user, no migrations. Create form in dashboard, set ID in `.env`.

---

## 2. Laravel – Demo Page, Config, and Behavior

### Config

- In `config/services.php`, under the existing `goforms` array, add:
  - `'demo_form_id' => env('GOFORMS_DEMO_FORM_ID'),` (nullable string).
- In `.env.example`, add:
  - `GOFORMS_DEMO_FORM_ID=` with a commented note: create a form in the dashboard and paste its ID here for the `/demo` page.

### Route

- `GET /demo` → invokable controller or closure that:
  - Reads `config('services.goforms.demo_form_id')`.
  - If missing or empty: render a small Inertia page (e.g. `DemoUnconfigured.vue`) that explains the demo is not set up and links to dashboard or docs; or redirect to home with a flash. No call to Go.
  - If set: `Inertia::render('Demo', ['formId' => $demoFormId])`. Rely on existing shared props (e.g. `goFormsPublicUrl` from HandleInertiaRequests) so the front end can call Go.

### Demo page (when configured)

- New `resources/js/pages/Demo.vue` (and optionally `DemoUnconfigured.vue` if a dedicated “not configured” view is used).
- **Demo.vue:**
  - Props: `formId` (string). Use shared `goFormsPublicUrl` for schema and submit (same pattern as `Forms/Fill.vue`).
  - Layout: reuse existing guest/public layout (e.g. same header/nav pattern as Home). One main area with a card (“Try the form” / “Contact form demo”).
  - No hardcoded schema: fetch schema from `{goFormsPublicUrl}/forms/{formId}/schema`, render with Form.io, submit to `{goFormsPublicUrl}/forms/{formId}/submit`. Show success (“Thanks, we’ve received your response”) and handle validation/network errors.
  - Optional: link “Create your own form” to dashboard or `/forms` if that route exists.
- No new backend submit logic in Laravel: browser talks only to Go’s public schema/submit endpoints. Laravel only serves the page and passes `formId` and shared URL.

### Controller

- Thin invokable, e.g. `App\Http\Controllers\DemoController` with one method (e.g. `__invoke`), or a closure in `routes/web.php`. No Form Request; only reading config and rendering Inertia.

### Tests

- Feature test: `GET /demo` with `GOFORMS_DEMO_FORM_ID` set returns 200 and Inertia component `Demo` with `formId`. With `GOFORMS_DEMO_FORM_ID` unset, either 200 with `DemoUnconfigured` (or equivalent) or redirect, per chosen behavior.

---

## 3. goforms – Remove Public Home/Demo UI and Dead References

**Assumption:** Go does not register GET `/` or GET `/demo`; only path constants and access rules reference them. Removal is limited to frontend assets and those references.

### Frontend (goforms `src/`)

- **Delete**
  - `src/pages/Home.vue`
  - `src/pages/Demo.vue`
- **Keep**
  - `GuestLayout`, `Nav`, `Footer` (used by Auth and Error).
- **Update**
  - **Nav.vue:** Remove or replace the link that points to `"/"` (e.g. logo or “Home”). Options: point to `/login`, or use `#` / non-navigating link so the logo does not target a removed page.
  - **Error.vue:** Change `Link href="/"` to an existing route (e.g. `/login`) so “go home” does not hit a removed page.
- **CSS**
  - In `src/css/main.css`, remove the two imports that exist only for the demo page: `components/demo_form.css` and `pages/demo/sections.css`. Delete those two files if they exist and are unused elsewhere.

### Backend (goforms `internal/`)

- **constants.go:** Remove `PathHome` and `PathDemo` (and their values `"/"` and `"/demo"`).
- **paths.go (PathManager):** Remove `PathHome` and `PathDemo` from the `PublicPaths` slice.
- **access.go (DefaultRules):** Remove the two rules that use `constants.PathHome` and `constants.PathDemo`.
- **access_test.go:** Remove or adjust test cases that assert behavior for `PathHome` or `PathDemo` (e.g. map entries setting those paths to `access.Public` and any tests that depend on them).
- **CSRF / middleware:** If any logic explicitly checks for `path == "/"` or `path == "/demo"` (e.g. for CSRF or “public path” lists), remove or update those branches so they no longer reference the removed constants. If they use the constants, removing the constants and updating call sites is sufficient.

### Verification

- Run `task lint` and `task test` in goforms after cleanup; fix any references to removed constants or pages.
- No new routes or handlers in Go; public form API (e.g. `/forms/:id/schema`, `/forms/:id/submit`) remains unchanged.
