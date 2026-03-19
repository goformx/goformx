# GoFormX: Laravel to Waaseyaa Migration Design

**Date:** 2026-03-19
**Status:** Approved
**GitHub Issue:** [#43](https://github.com/goformx/goformx/issues/43)
**GitHub Milestone:** [Laravel to Waaseyaa Migration](https://github.com/goformx/goformx/milestone/1)

## Motivation

1. **Dogfooding** — Prove out Waaseyaa as a real application framework with a production app
2. **Feature alignment** — Leverage Waaseyaa's entity system, AI capabilities, and plugin architecture
3. **Consolidation** — Reduce the number of frameworks maintained and deployed

## Architecture Overview

```
Browser
  ├── Public pages (SSR) ──→ Waaseyaa SSR templates (marketing, legal, docs, pricing)
  ├── Auth pages (SSR) ────→ Waaseyaa SSR templates (login, register, 2FA, verify)
  └── App pages (Inertia) ─→ Inertia v3 + Vue 3 (dashboard, form builder, submissions, billing, settings)
        │
        ├── Waaseyaa API ──→ Users, Auth, Billing, Plans (MariaDB)
        │     │
        │     └── GoFormsClient service ──→ Go API :8090 (HMAC assertion auth)
        │                                      └── Forms, Submissions (PostgreSQL)
        └── Go Public API ──→ Form schema + submit (direct, no auth)
```

### Key Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Migration approach | Full Waaseyaa stack (Approach A) | Only option that satisfies all three motivations |
| Frontend strategy | Hybrid SSR + Inertia v3/Vue 3 | SSR for public/auth, Inertia for interactive app pages |
| Entity scope | Users/billing only | Go API continues to own forms/submissions |
| Billing | Full Stripe integration from day one | Feature parity, no degraded experience |
| Database | MariaDB (keep existing) | No unnecessary database migration |
| Dev environment | Docker Compose | DDEV's Laravel/WordPress conventions don't fit Waaseyaa |
| PHP version | 8.4+ | Current target for both GoFormX and Waaseyaa |
| Cutover strategy | Big bang | Build in parallel, cut over when feature-complete |
| Inertia version | v3 (beta) | No point building against v2 when v3 is already available |

### Project Structure

```
goformx/
├── goforms/              # Go API (unchanged)
├── goformx-web/          # NEW — Waaseyaa application
│   ├── src/              # PHP: entities, services, controllers, middleware
│   ├── frontend/         # Vue 3 + Inertia v3 (dashboard, builder, settings, billing)
│   ├── templates/        # Waaseyaa SSR templates (public + auth pages)
│   ├── config/           # Waaseyaa config
│   ├── docker-compose.yml
│   └── composer.json     # requires waaseyaa/full
├── goformx-formio/       # Submodule (unchanged)
└── goformx-laravel/      # Deprecated after cutover
```

## New Waaseyaa Packages

Three new framework packages are built as part of this migration. These live in the Waaseyaa repo, not in GoFormX.

### `waaseyaa/auth` (Layer 2: Services)

Headless authentication package — provides auth logic, app provides UI and config.

**Features:**
- Login/logout with Symfony session management
- Registration with email verification (signed URLs)
- Password reset (token-based)
- TOTP two-factor authentication
- Rate limiting on auth endpoints (Symfony RateLimiter)
- Event hooks (login succeeded, registration completed, etc.)
- Configurable redirects and views

**Rate limits (matching current Laravel config):**
- Login: 5 attempts/min per email+IP
- 2FA: 5 attempts/min per session
- Password reset: 6 attempts/min

### `waaseyaa/billing` (Layer 2: Services)

Stripe billing package — replaces Laravel Cashier.

**Core service: `BillingManager`**
- `createCheckoutSession(User $user, string $priceId): CheckoutSession`
- `getPortalUrl(User $user): string`
- `syncSubscriptionFromWebhook(array $payload): void`
- `resolveUserTier(User $user): PlanTier` (enum: free, pro, business, growth, enterprise)

**Entities provided:**
- **Subscription** — stripe_id, stripe_status, stripe_price, quantity, trial_ends_at, ends_at
- **SubscriptionItem** — stripe_id, stripe_product, stripe_price, quantity, meter_id, meter_event_name

**User entity fields (added by package):**
- stripe_id, pm_type, pm_last_four, trial_ends_at

**Webhook handling:**
- Route: `POST /stripe/webhook`
- Verifies Stripe signature
- Events: `checkout.session.completed`, `customer.subscription.created/updated/deleted`, `invoice.payment_succeeded/failed`

**Plan tier resolution (same priority as current):**
1. `plan_override` on User entity (admin-set, "founding" maps to "business")
2. Active subscription price → tier mapping (configurable)
3. Default: `free`

**Founding member program:**
- `foundingMemberSlotsRemaining(): int`
- Cap configurable via `billing.founding_member_cap` (default 100)

### `waaseyaa/inertia` (Layer 6: Interfaces)

Server-side Inertia v3 protocol adapter for Waaseyaa.

**Protocol implementation:**
- Detects `X-Inertia` header on requests
- Initial visit → full HTML with page object in `<script type="application/json">`
- Subsequent visits → JSON page object response
- `303 See Other` redirects for PUT/PATCH/DELETE
- Asset version checking → `409 Conflict` on mismatch

**Page object support (v3 spec):**
- `component`, `props`, `url`, `version`
- `deferredProps`, `mergeProps`, `prependProps`, `deepMergeProps`, `matchPropsOn`
- `encryptHistory`, `clearHistory` (only included when `true`)
- `onceProps`, `scrollRegions`
- `preserveFragment`

**Features:**
- `Inertia::render('Component', ['prop' => $value])` response helper
- Shared props middleware
- `optional()` props (lazy evaluation)
- Partial reload support (`X-Inertia-Partial-Component`, `X-Inertia-Partial-Data`, `X-Inertia-Partial-Except`)
- Precognition validation support
- Vite manifest integration for asset versioning

**Client-side:** Uses official `@inertiajs/vue3@^3.0.0-beta` and `@inertiajs/vite@^3.0.0-beta` packages.

## Entity System & Data Model

### User Entity (Waaseyaa content entity)

Replaces Laravel's User model.

| Field | Type | Notes |
|---|---|---|
| uuid | UUID | Primary key, matches existing Laravel UUIDs |
| name | string | |
| email | string | Unique |
| password | string | Bcrypt hashed |
| email_verified_at | timestamp | Nullable |
| two_factor_secret | text | Nullable, encrypted |
| two_factor_recovery_codes | text | Nullable, encrypted |
| two_factor_confirmed_at | timestamp | Nullable |
| stripe_id | string | Nullable, from waaseyaa/billing |
| pm_type | string | Nullable |
| pm_last_four | string | Nullable |
| trial_ends_at | timestamp | Nullable |
| plan_override | string | Nullable, admin-set tier |
| created_at | timestamp | |
| updated_at | timestamp | |

### What stays in Go's PostgreSQL

- Forms, form submissions — no changes
- Go's shadow user records — synced from HMAC assertions as before

## Authentication & Security

### Auth Flows (provided by `waaseyaa/auth`)

**Login:** SSR form → POST → validate credentials → create Symfony session → redirect to `/dashboard` (Inertia takes over)

**Registration:** SSR form → POST → create User entity → send verification email → redirect to verify page

**2FA (TOTP):** Generate secret → show QR → verify code. Challenge page is SSR (shown before session is fully authenticated).

**Email verification:** Signed URL via Symfony UrlGenerator. SSR page.

**Password reset:** Token-based, SSR pages throughout.

### Security Middleware

Ported 1:1 from current Laravel middleware:

- **SecurityHeaders** — X-Frame-Options: DENY, X-Content-Type-Options: nosniff, Referrer-Policy: strict-origin-when-cross-origin, Permissions-Policy (camera/microphone/geolocation disabled), HSTS in production (1 year max-age)
- **CSRF** — Symfony CSRF component for SSR forms; Inertia handles CSRF for XHR requests
- **Session auth** — Symfony session cookie

### HMAC Assertion Bridge (GoFormsClient)

Reimplemented as a Waaseyaa service. Same signature format, same headers. Go API requires zero changes.

- Signature: `HMAC-SHA256(METHOD:PATH:USER_ID:TIMESTAMP:PLAN_TIER)`
- Headers: `X-User-Id`, `X-Timestamp`, `X-Signature`, `X-Plan-Tier`
- Shared secret: `GOFORMS_SHARED_SECRET` env var
- Timestamp skew tolerance: 60 seconds

## Routing & Controllers

### SSR Routes (Waaseyaa controllers → server-rendered templates)

| Route | Purpose |
|---|---|
| `GET /` | Home/landing page |
| `GET /pricing` | Pricing page |
| `GET /privacy`, `GET /terms` | Legal pages |
| `GET /docs/{slug?}` | Documentation |
| `GET /demo` | Demo page |
| `GET /robots.txt`, `GET /sitemap.xml` | SEO |
| `GET /login`, `POST /login` | Login |
| `GET /register`, `POST /register` | Registration |
| `GET /forgot-password`, `POST /forgot-password` | Password reset request |
| `GET /reset-password/{token}`, `POST /reset-password` | Password reset |
| `GET /verify-email`, `GET /verify-email/{id}/{hash}` | Email verification |
| `GET /two-factor-challenge`, `POST /two-factor-challenge` | 2FA |
| `POST /logout` | Logout |
| `GET /forms/{id}` | Public form fill page |

### Inertia Routes (authenticated, rendered by Vue via Inertia)

| Route | Controller | Purpose |
|---|---|---|
| `GET /dashboard` | DashboardController | Main dashboard |
| `GET /forms` | FormController | List user's forms |
| `POST /forms` | FormController | Create form (→ GoFormsClient) |
| `GET /forms/{id}/edit` | FormController | Form.io builder |
| `GET /forms/{id}/preview` | FormController | Form preview |
| `GET /forms/{id}/submissions` | FormController | List submissions |
| `GET /forms/{id}/submissions/{sid}` | FormController | View submission |
| `GET /forms/{id}/embed` | FormController | Embed code |
| `PUT /forms/{id}` | FormController | Update form (→ GoFormsClient) |
| `DELETE /forms/{id}` | FormController | Delete form (→ GoFormsClient) |
| `GET /settings/profile` | SettingsController | Profile edit |
| `PATCH /settings/profile` | SettingsController | Update profile |
| `DELETE /settings/profile` | SettingsController | Delete account |
| `GET /settings/password` | SettingsController | Password edit |
| `PUT /settings/password` | SettingsController | Change password |
| `GET /settings/appearance` | SettingsController | Theme settings |
| `GET /settings/two-factor` | SettingsController | 2FA management |
| `GET /billing` | BillingController | Billing dashboard + usage |
| `POST /billing/checkout` | BillingController | Stripe checkout |
| `GET /billing/portal` | BillingController | Stripe portal redirect |

### Controller Structure

Thin controllers — same pattern as current Laravel:

- **PublicController** — SSR marketing/legal pages
- **AuthController** — SSR auth flows (delegates to `waaseyaa/auth`)
- **DashboardController** — Inertia dashboard
- **FormController** — Inertia, proxies to Go via GoFormsClient (with same error mapping)
- **SettingsController** — Inertia, profile/password/2FA
- **BillingController** — Inertia, delegates to `waaseyaa/billing`

## Frontend Architecture

### SSR Pages (Waaseyaa templates)

Server-rendered HTML for non-interactive pages:
- Public: Home, pricing, docs, demo, privacy, terms, public form fill
- Auth: Login, register, forgot/reset password, verify email, 2FA challenge

Styling: Tailwind CSS v4 (same theme variables). Alpine.js for minor interactivity (mobile nav, theme switcher).

### Inertia + Vue 3 Pages

Interactive authenticated pages — nearly identical to current setup:

**What carries over unchanged:**
- shadcn-vue components (`components/ui/`)
- Form.io integration + `@goformx/formio` templates
- Tailwind CSS v4 theme variables
- Lucide icons, vue-sonner toasts, zod validation
- AppSidebar layout pattern

**What changes:**
- `@inertiajs/vue3` v2 → v3
- Axios interceptors → Inertia v3 built-in HTTP client
- Add `@inertiajs/vite` plugin (simplifies Vite config, handles page resolution + SSR)
- Minor import path adjustments

### NPM Dependencies

**Keep:**
- `vue` ^3.5, `@inertiajs/vue3` ^3.0.0-beta, `@inertiajs/vite` ^3.0.0-beta
- `@formio/js` ^5.1, `@goformx/formio` ^0.2
- `tailwindcss` ^4.1, `@tailwindcss/vite`
- `reka-ui`, `lucide-vue-next`, `zod`, `vue-sonner`, `vue-input-otp`, `@vueuse/core`
- `vite`, `@vitejs/plugin-vue`, `typescript`, `vue-tsc`
- `eslint`, `prettier`, `@playwright/test`

**Drop:**
- `axios` (replaced by Inertia v3 built-in)

## Docker Compose Development Environment

```yaml
services:
  web:
    build: ./docker/php
    # PHP 8.4 + Nginx
    volumes:
      - .:/app
    ports:
      - "8080:80"
    depends_on:
      - mariadb
    environment:
      - APP_ENV=local
      - GOFORMS_API_URL=http://goforms:8090

  mariadb:
    image: mariadb:11.8
    volumes:
      - mariadb_data:/var/lib/mysql
    environment:
      - MARIADB_DATABASE=goformx
      - MARIADB_USER=goformx
      - MARIADB_PASSWORD=goformx

  goforms:
    build: ../goforms
    ports:
      - "8091:8090"
    depends_on:
      - postgres
    environment:
      - DATABASE_URL=postgres://goforms:goforms@postgres:5432/goforms

  postgres:
    image: postgres:17
    volumes:
      - postgres_data:/var/lib/postgresql/data
    environment:
      - POSTGRES_DB=goforms
      - POSTGRES_USER=goforms
      - POSTGRES_PASSWORD=goforms

  vite:
    build: ./docker/node
    volumes:
      - ./frontend:/app
    ports:
      - "5173:5173"
    command: npm run dev
```

**Dev workflow (Taskfile):**
```bash
task dev         # docker compose up -d
task setup       # first-time install + migrate
task test        # run PHP + Go tests
task lint        # run PHP + Go linters
```

## Deployment & Production

**Server:** `deployer@goformx.com`

| Concern | Current (Laravel) | New (Waaseyaa) |
|---|---|---|
| Deploy tool | Deployer (PHP) | Shell script or GitHub Actions |
| Deploy path | `/home/deployer/goformx` | `/home/deployer/goformx-web` |
| PHP runtime | PHP 8.4 + nginx | PHP 8.4 + nginx (unchanged) |
| Go API | Docker container `goforms-goforms-1` | Unchanged |
| Shared config | `/home/deployer/goformx/shared/.env` | `/home/deployer/goformx-web/shared/.env` |

**Deploy steps:**
1. Clone/pull release to new release directory
2. Symlink shared `.env`
3. `composer install --no-dev`
4. `php bin/console migrate`
5. `npm ci && npm run build` (Vite production build)
6. Symlink `current` to new release
7. Reload PHP-FPM

**Environment variables:**
- `GOFORMS_API_URL=http://127.0.0.1:8090`
- `GOFORMS_SHARED_SECRET` (must match Go)
- `GOFORMS_PUBLIC_URL=https://api.goformx.com`
- `STRIPE_KEY`, `STRIPE_SECRET`, `STRIPE_WEBHOOK_SECRET`
- `STRIPE_*_PRICE` variables
- `DB_HOST`, `DB_DATABASE`, `DB_USERNAME`, `DB_PASSWORD` (MariaDB)
- `APP_ENV=production`, `APP_URL=https://goformx.com`

## Data Migration & Cutover Plan

### Pre-Migration Backup

**Before any migration steps, create full backups:**
1. Full MariaDB database dump (`mysqldump --all-databases`)
2. Full application files backup (`/home/deployer/goformx/` — all releases + shared)
3. Verify backups are restorable
4. Store backups in a separate location from the deployment server

### MariaDB Schema Changes

**Keep (data preserved):**
- `users` — all records, UUIDs, Stripe fields, 2FA fields, plan_override
- `subscriptions` — Stripe subscription records
- `subscription_items` — Stripe subscription items

**Adapt for Waaseyaa:**
- Add `_data` JSON blob column to `users`, `subscriptions`, `subscription_items`
- Adjust columns as needed for Waaseyaa entity storage conventions

**Drop (Laravel-specific):**
- `sessions` — Waaseyaa uses Symfony session handling
- `cache` — Waaseyaa's cache layer
- `jobs` — Waaseyaa's queue system
- `password_reset_tokens` — reimplemented in `waaseyaa/auth`
- `migrations` — Laravel migration tracking

### Cutover Sequence

1. Full database + file backup (verified restorable)
2. Put Laravel app in maintenance mode
3. Run MariaDB schema migrations
4. Deploy `goformx-web` to `/home/deployer/goformx-web`
5. Point nginx to new app
6. Verify all functionality
7. Remove old Laravel deployment (after confidence period)

### Zero User Impact

- Existing users keep their UUIDs, passwords (bcrypt), Stripe subscriptions, and 2FA setup
- No re-registration needed
- Go API completely unaffected
- User UUIDs match between MariaDB and Go's PostgreSQL (assertion auth depends on this)

## What Does NOT Change

- **Go API** (`goforms/`) — all code, endpoints, database, Docker container
- **HMAC assertion auth protocol** — same signature format, same headers
- **PostgreSQL** — forms and submissions database
- **`@goformx/formio`** — Tailwind Form.io templates submodule
- **Production Go infrastructure** — Docker container `goforms-goforms-1` at `http://127.0.0.1:8090`
