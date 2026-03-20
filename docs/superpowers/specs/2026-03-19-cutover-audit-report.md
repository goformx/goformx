# GoFormX Cutover Audit Report

**Date:** 2026-03-19
**Status:** FAIL — 7 P0 blockers, 5 P1 issues, 3 P2 hygiene items

---

## A. Executive Summary

The goformx-web scaffold is structurally complete: 28 Vue pages render, login/register work, HMAC auth passes to the Go API, Form.io builder initializes correctly, and the Go shadow user syncer works as designed.

However, **the application cannot go to production.** Two categories of blockers exist:

1. **Database schema mismatch (P0):** The goformx-web migrations use `uid`/`mail` column names while the existing Laravel database uses `id`/`email`. The `CREATE TABLE IF NOT EXISTS` pattern means the migrations silently do nothing on the existing database, and the application code then queries columns that don't exist. This is a silent runtime crash for every authenticated operation.

2. **Stub route handlers (P0):** 7 route handlers are stubs that redirect without performing any operation — password reset, email verification, profile update, password change, Stripe checkout, Stripe portal, and webhook processing. A real StripeClient implementation doesn't exist.

Both categories must be fixed before cutover.

---

## B. Full Blocker List

### P0 — Production-Blocking

| # | Area | Finding |
|---|------|---------|
| 1 | DB Migration | `users` table uses `uid` PK; Laravel has `id`. Runtime column-not-found. |
| 2 | DB Migration | `users` table uses `mail`; Laravel has `email`. Query failure on every user lookup. |
| 3 | Auth | `forgot-password.post` — no token generated, no email sent. Stub. |
| 4 | Auth | `reset-password.post` — no password reset. Immediate redirect. |
| 5 | Auth | `verify-email.verify` — no email verification. Immediate redirect. |
| 6 | Billing | `billing.checkout` / `billing.portal` — stubs returning redirect. |
| 7 | Billing | No `StripeClient` implementation exists. `StripeClientInterface` is unbound. |

### P1 — Correctness / Security

| # | Area | Finding |
|---|------|---------|
| 8 | Auth | `settings.profile.update` — stub, no DB write. |
| 9 | Auth | `settings.password.update` — stub, no password change. |
| 10 | Auth | `settings.profile.destroy` — clears session but never deletes user from DB. |
| 11 | Security | No CSRF validation on Inertia XHR — `application/json` exempt in CsrfMiddleware. |
| 12 | Security | No rate limiting on login — `RateLimiter` imported but never called. |

### P2 — Hygiene

| # | Area | Finding |
|---|------|---------|
| 13 | Auth | 2FA recovery code consumed but never removed from DB (reuse possible). |
| 14 | Auth | `logout` doesn't clear `two_factor_uid` session key. |
| 15 | DB Migration | `sessions` table missing `ip_address`/`user_agent`; `plan_override` width mismatch. |

### Working Correctly

| Area | Status |
|------|--------|
| Login (POST /login) | Authenticates against MariaDB, creates session |
| Registration (POST /register) | Creates user, sets session |
| Logout (POST /logout) | Clears session UID |
| 2FA TOTP verification | Verifies codes with ±1 window |
| HMAC signature format | Matches Go API exactly |
| Go shadow user syncer | Auto-creates shadow user on first assertion |
| Form.io builder | Loads @goformx/formio templates, saves schema to Go API |
| Form CRUD (GET/PUT/DELETE) | Proxies to Go API correctly |
| Inertia v3 rendering | Pages render with correct page data |

---

## C. GitHub Issues

### Issue 1: Fix database schema to match existing Laravel tables
**Labels:** `migration`, `P0`
**Milestone:** Laravel to Waaseyaa Migration

Rewrite `goformx-web/migrations/` to work against the existing Laravel MariaDB schema:
- Use `id` instead of `uid` as the users PK column
- Use `email` instead of `mail`
- Update `UserRepository.php` to query `id` and `email` columns
- Update `AppServiceProvider.php` session reads from `waaseyaa_uid` → user's `id` column
- Fix FK references in subscriptions table: `REFERENCES users(id)`
- Add missing columns: `meter_id`, `meter_event_name` to subscription_items
- Add `ip_address`, `user_agent` to sessions
- Align `plan_override` to `VARCHAR(20)`

### Issue 2: Wire auth POST handlers (password reset, email verify, profile, password)
**Labels:** `migration`, `P0`
**Milestone:** Laravel to Waaseyaa Migration

Complete the stub route handlers:
- `forgot-password.post`: Generate token via `PasswordResetManager`, send email via `MailerInterface`
- `reset-password.post`: Validate token, call `UserRepository::updatePassword()`
- `verify-email.verify`: Validate signed URL via `EmailVerifier`, call `UserRepository::verifyEmail()`
- `settings.profile.update`: Read name/email from request, add `UserRepository::updateProfile()`, update DB
- `settings.password.update`: Verify current password, call `UserRepository::updatePassword()`
- `settings.profile.destroy`: Add `UserRepository::delete()`, delete user from DB, clear session

### Issue 3: Create StripeClient and wire billing routes
**Labels:** `migration`, `P0`
**Milestone:** Laravel to Waaseyaa Migration

- Create `src/Service/StripeClient.php` implementing `StripeClientInterface` using `\Stripe\StripeClient`
- Register `StripeClientInterface` binding in `AppServiceProvider::register()`
- Wire `billing.checkout` route to `BillingController::checkout()`
- Wire `billing.portal` route to `BillingController::portal()`
- Wire `stripe.webhook` route to `WebhookHandler` with Stripe signature verification

### Issue 4: Wire email sending for auth flows
**Labels:** `migration`, `P1`
**Milestone:** Laravel to Waaseyaa Migration

- Register `MailerInterface` in `AppServiceProvider` with SMTP transport (PostMark in production, Mailpit in dev)
- Send verification email on registration with signed URL from `EmailVerifier`
- Send password reset email on forgot-password with token from `PasswordResetManager`
- Add mail config to `config/waaseyaa.php` (SMTP host, port, from address)

### Issue 5: Fix CSRF protection for Inertia XHR and add login rate limiting
**Labels:** `migration`, `security`, `P1`
**Milestone:** Laravel to Waaseyaa Migration

- CsrfMiddleware exempts `application/json` content type, which includes all Inertia mutations
- Options: (a) Check `X-CSRF-Token` header even for JSON when `X-Inertia` is present, or (b) rely on `X-Inertia` custom header as CSRF mitigation (document this decision)
- Wire `RateLimiter` into `login.post` handler: 5 attempts/min per email+IP
- Fix `AuthManager::logout()` to also unset `two_factor_uid` and regenerate session ID

### Issue 6: Fix 2FA recovery code consumption
**Labels:** `migration`, `P2`
**Milestone:** Laravel to Waaseyaa Migration

- Add `UserRepository::updateRecoveryCodes()` method
- After successful recovery code verification, remove the used code from the stored array and update DB
- Prevents recovery code reuse

---

## D. PR Plans

**PR 1 (Issue 1): Database schema alignment**
- Rewrite all 5 migration SQL files
- Update `UserRepository.php` (find/create/update methods to use `id`/`email`)
- Update `AppServiceProvider.php` session key references
- Test: run migrations on empty DB + verify against production schema dump

**PR 2 (Issue 2): Auth handler wiring**
- Add `updateProfile()`, `delete()`, `updateRecoveryCodes()` to UserRepository
- Complete 6 route handler closures in AppServiceProvider
- Add unit tests for each handler path

**PR 3 (Issue 3): Stripe billing wiring**
- Create `StripeClient.php` (~50 LOC wrapping 3 SDK calls)
- Update AppServiceProvider bindings + 3 route closures
- Add integration test with FakeStripeClient

**PR 4 (Issue 4): Email wiring**
- Add mail config, register MailerInterface
- Wire into forgot-password and register handlers
- Test with Mailpit in Docker

**PR 5 (Issue 5): Security fixes**
- CSRF decision + implementation
- Rate limiter wiring in login handler
- Session hygiene in logout

**PR 6 (Issue 6): 2FA recovery code fix**
- Add updateRecoveryCodes to UserRepository
- Wire into 2FA challenge handler

**Execution order:** PR1 → PR2 → PR3 → PR4 → PR5 → PR6 (PR1 unblocks everything; PR2-4 can parallelize after PR1)

---

## E. Database Migration Safety Report

**Current state:** The `CREATE TABLE IF NOT EXISTS` pattern means the goformx-web migrations are NO-OPs on the existing production database. No data loss risk from running them. But the application queries non-existent columns (`uid`, `mail`), causing runtime errors.

**Required approach:**
1. Do NOT use `CREATE TABLE IF NOT EXISTS` for tables that already exist
2. Write `ALTER TABLE` migrations that add the new columns (`_data` JSON, missing meter columns) without touching existing columns
3. The application code must use the existing column names (`id`, `email`) not the Waaseyaa entity conventions (`uid`, `mail`)
4. Test against a production schema dump before cutover

**Data loss risk:** ZERO if migrations are additive-only (no ALTER COLUMN, no DROP, no RENAME). The existing data is untouched.

---

## F. Email + Billing + Auth Validation Plan

After PRs 1-5 are merged, validate each flow end-to-end with Playwright:

1. **Registration → Email verification:** Register, check Mailpit for email, click link, verify `email_verified_at` is set
2. **Login → Dashboard:** Login with credentials, verify session, verify dashboard renders user data
3. **Password reset:** Forgot password → check Mailpit → click link → set new password → login with new password
4. **2FA setup:** Enable 2FA in settings → scan QR → verify code → logout → login → 2FA challenge → verify code
5. **Profile update:** Change name/email → verify DB updated
6. **Password change:** Change password → logout → login with new password
7. **Stripe checkout:** Click upgrade → verify redirect to Stripe Checkout (test mode)
8. **Stripe portal:** Click manage → verify redirect to Stripe portal
9. **Stripe webhook:** Send test webhook via `stripe listen --forward-to` → verify subscription synced

---

## G. Form.io Integration Validation Plan

Form.io builder is confirmed working at the code level. Runtime validation:

1. `npm link @goformx/formio` in the Vite container
2. Login → Create form → Verify builder loads with goforms Tailwind templates
3. Add fields (text, email, textarea) → Verify sidebar and drag-drop work
4. Save form → Verify PUT to Go API succeeds (check Go logs for 200)
5. Preview form → Verify form renders with saved schema
6. Public fill → Navigate to `/forms/{id}` → Submit → Verify Go API receives submission

---

## H. Cutover Plan (with Rollback)

**Pre-cutover (day before):**
1. Full MariaDB dump: `mysqldump --single-transaction --all-databases`
2. Full Laravel app backup: `tar czf /tmp/goformx-laravel-backup.tar.gz /home/deployer/goformx/`
3. Verify backups are restorable
4. Run goformx-web test suite
5. Run migrations against staging copy of production DB — verify no errors

**Cutover sequence (15-minute window):**
1. `php artisan down` on Laravel app (maintenance mode)
2. Run additive ALTER TABLE migrations on production MariaDB
3. Deploy goformx-web via GitHub Actions (push to main)
4. Verify deployment completed (check Actions run)
5. Reload Caddy to serve goformx-web
6. Smoke test: home, login, dashboard, forms, billing, settings (Playwright)
7. Verify Go API logs show successful HMAC auth from new app
8. Remove maintenance mode marker

**Rollback (instant, <1 minute):**
1. Reload Caddy to serve Laravel app (swap Caddyfile back)
2. Laravel is fully intact — no destructive schema changes
3. Additive columns (`_data`) are harmless to Laravel
4. Investigate, fix, re-attempt

**Confidence period:** 7 days. Laravel deployment preserved. Daily checks of error logs, Go API logs, Stripe webhook logs.

---

## I. 48-Hour Post-Cutover Monitoring Plan

**Hour 0-1:** Watch Go API logs live for assertion failures. Check error rates.

**Hour 1-4:** Monitor:
- PHP error log (`/home/deployer/goformx-web/log/php-error.log`)
- Caddy access log for 5xx responses
- Go API logs for 401/500 responses
- Stripe dashboard for failed webhooks

**Hour 4-24:** Check:
- All 14 verification checklist items from Issue #49
- Stripe webhook delivery success rate
- New user registrations creating Go shadow users
- Email delivery (PostMark dashboard)

**Hour 24-48:** Check:
- User-reported issues
- Session stability (no random logouts)
- Form builder saves reliably
- Billing page shows correct tier for subscribed users

**Alert triggers for immediate rollback:**
- Any 500 error rate > 1% of requests
- HMAC assertion failures in Go API logs
- Stripe webhook signature verification failures
- Users unable to login

---

## J. Founder Notes

1. **Fix the column names first.** Everything else is blocked by the `uid`/`mail` vs `id`/`email` mismatch. This is a 30-minute fix but it touches every layer.

2. **The Stripe integration is the longest pole.** Creating `StripeClient`, wiring billing routes, and testing with real Stripe test keys will take a focused session. Consider doing this with Stripe CLI's `listen` command running.

3. **The CSRF decision needs your input.** Inertia's XHR requests are exempt from CSRF because they send `application/json`. The `X-Inertia` custom header provides some protection (browsers won't send it cross-origin without CORS). You need to decide: (a) accept the custom-header mitigation and document it, or (b) add explicit `X-CSRF-Token` header checking. Option (a) is what Laravel's Inertia does in practice.

4. **The migration is NOT a rewrite.** The Go API, Form.io templates, Vue components, and HMAC protocol are all unchanged. The risk surface is the PHP middleware layer — and most of that is thin closures wiring existing packages together.

5. **Estimated remaining effort:** 2 sessions. Session 1: PR1 (schema) + PR2 (auth handlers) + PR5 (security). Session 2: PR3 (Stripe) + PR4 (email) + PR6 (2FA). Then cutover.
