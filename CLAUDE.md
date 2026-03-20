# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GoFormX is a forms management platform organized as a monorepo with three services:

- **`goforms/`** — Go API backend (Echo, GORM, Uber FX). Owns the entire forms domain: CRUD, schema storage, submissions, public embed/submit. API-only, no UI.
- **`goformx-web/`** — **Waaseyaa + Vue 3 + Inertia v3 frontend** (primary). Handles identity (waaseyaa/auth), user dashboard, form builder UI (Form.io), billing (waaseyaa/billing), and settings.
- **`goformx-laravel/`** — **Deprecated.** Laravel 12 + Vue 3 + Inertia v2 frontend. Kept during confidence period for rollback capability. Do not develop new features here.
- **`goformx-formio/`** — Git submodule (`goformx/formio`). Form.io template library providing Tailwind-based templates to replace Form.io's default Bootstrap templates.

Each service has its own CLAUDE.md with detailed development instructions. Read those when working within a specific service.

## Orchestration Table

When working on a file pattern, consult the associated skill and spec for context:

| File Pattern | Skill | Spec |
|-------------|-------|------|
| `goformx-web/src/Service/GoFormsClient*` | `debug-integration` | `docs/specs/cross-service-auth.md` |
| `goformx-web/src/Service/UserRepository*` | `laravel-to-waaseyaa` | `docs/specs/user-persistence.md` |
| `goformx-web/src/Controller/Form*` | `feature-dev` | `docs/specs/form-lifecycle.md` |
| `goformx-web/src/Controller/Auth*` | `feature-dev` | `docs/specs/cross-service-auth.md` |
| `goformx-web/src/Controller/Billing*` | `feature-dev` | `docs/specs/form-lifecycle.md` |
| `goformx-web/src/AppServiceProvider.php` | `claudriel` | All specs (routes span all domains) |
| `goformx-web/frontend/**` | `frontend-design` | `docs/specs/form-lifecycle.md` |
| `goforms/internal/application/middleware/assertion/**` | `debug-integration` | `docs/specs/cross-service-auth.md` |
| `goforms/internal/domain/form/**` | `feature-dev` | `docs/specs/form-lifecycle.md` |
| `goforms/internal/application/handlers/**` | `feature-dev` | `docs/specs/form-lifecycle.md` |
| `.claude/rules/**` | `updating-codified-context` | — |
| `docs/specs/**` | `updating-codified-context` | — |

## Form.io Template Integration

The `@goformx/formio` package provides custom Tailwind-styled templates for the Form.io builder and renderer. **Important development notes:**

### Local Development Setup

During development, link the local package to get template changes immediately:

```bash
cd goformx-formio && npm link
cd goformx-web/frontend && npm link @goformx/formio
```

### Template Registration

Templates are registered in `useFormBuilder.ts`:
```typescript
import { Formio } from '@formio/js';
import goforms from '@goformx/formio';
Formio.use(goforms);  // Registers 'goforms' framework templates
```

### CSS Requirements

The goforms templates use Tailwind CSS variables and utilities. Styling Form.io elements uses a two-pronged approach:

1. **CSS overrides** (`formio-overrides.css` in `layer(formio)`): Structural/non-color properties (padding, border-radius, font-size) and dialog styling
2. **JavaScript CSSOM** (`useFormBuilder.ts`): Color properties (border, background-color, color) on sidebar buttons, drop zones, and submit buttons — needed because Bootstrap uses `!important` on those same properties within the formio layer

See `docs/solutions/2026-02-27-formio-builder-sidebar-visibility.md` for detailed troubleshooting.

## Development Environment

### GoForms (Go Backend) — runs on :8090

```bash
cd goforms
task install              # Install Go tools (mockgen, air, golangci-lint, migrate)
task generate             # Generate mocks
task dev:backend          # Hot reload dev server (air)
task build                # Build binary
task test:backend         # Unit tests
task test:backend:cover   # Unit tests with coverage
task test:integration     # Integration tests
task lint:backend         # go fmt + go vet + golangci-lint
task migrate:up           # Apply database migrations
task migrate:down         # Rollback one migration
# Single test:
go test -v -run TestFunctionName ./path/to/package/...
```

### GoFormX-Web (Waaseyaa Frontend)

```bash
cd goformx-web
vendor/bin/phpunit          # Run tests
bin/migrate.php             # Run MariaDB migrations
# Frontend dev:
cd frontend && npm run dev  # Vite dev server on :5173
```

## Cross-Service Architecture

```
Browser → Waaseyaa (goformx-web) → GoFormsClient → GoForms API (:8090) → PostgreSQL
                │                                          │
          Users/Sessions DB (MariaDB)              Forms/Submissions DB (PostgreSQL)
```

### Assertion-Based Authentication

Waaseyaa authenticates users (session-based, AuthManager) and signs every request to Go with HMAC headers:

- `X-User-Id` — authenticated user's ID
- `X-Timestamp` — current UTC timestamp
- `X-Signature` — HMAC-SHA256 of `user_id:timestamp` using shared secret

Go middleware (`internal/application/middleware/assertion/`) verifies the signature and extracts the user ID for ownership checks. Timestamp skew tolerance: 60 seconds.

**Critical shared config**: `GOFORMS_SHARED_SECRET` must match in both `.env` files.

See `docs/specs/cross-service-auth.md` for full protocol details.

### GoFormsClient (Waaseyaa → Go)

Located at `goformx-web/src/Service/GoFormsClient.php`. Config: `goforms_api_url` (default `http://localhost:8090`), `goforms_shared_secret` in `config/waaseyaa.php`.

### Database Separation

- **Waaseyaa DB** (MariaDB): users, sessions, subscriptions, password resets
- **Go DB** (PostgreSQL 17): forms, form_submissions, users (shadow-synced from assertions)
- **Waaseyaa Entity Storage** (SQLite): framework entity storage (auto-migrated on boot)

Waaseyaa never touches form tables; Go never touches auth tables.

## API Surface (Go)

**Authenticated** (assertion headers required, called by Waaseyaa only):
- `GET/POST /api/forms`, `GET/PUT/DELETE /api/forms/:id`
- `GET /api/forms/:id/submissions`, `GET /api/forms/:id/submissions/:sid`

**Public** (no auth, rate limited):
- `GET /forms/:id/schema`, `POST /forms/:id/submit`, `GET /forms/:id/embed`

## Go Architecture (Clean Architecture + Uber FX, Go 1.25)

```
internal/
├── domain/           # Business entities, service interfaces, repository interfaces
│   ├── entities/     # Core entity structs (user.go)
│   ├── form/         # Form + FormSubmission models, service, repository
│   ├── user/         # User model, service, syncer
│   └── common/       # Shared errors, events, interfaces
├── application/      # HTTP layer
│   ├── constants/    # Application constants
│   ├── handlers/web/ # REST handlers (implement web.Handler interface)
│   ├── middleware/    # Assertion, security, CORS, access control
│   ├── response/     # Response builders
│   └── validation/   # Form schema validation
└── infrastructure/   # GORM repos, Viper config, Zap logging, Echo server, event bus
```

Dependencies point inward: Infrastructure → Application → Domain. FX wires everything in `main.go` using module groups.

Handlers implement `web.Handler` (Register/Start/Stop) and are collected via FX groups (`group:"handlers"`).

## Waaseyaa Frontend Architecture

- **Service providers** register DI, routes, middleware via `ServiceProvider` base class
- **Controllers** are thin: auth check → call GoFormsClient or UserRepository → Inertia render or redirect
- **Routes** defined in `AppServiceProvider::routes()` via `WaaseyaaRouter`
- **Frontend**: Vue 3 + Inertia v3 pages in `frontend/src/pages/`, shadcn-vue + reka-ui components
- **Form builder**: Form.io integration via `@goformx/formio` in `Forms/Edit.vue`
- **SSR pages**: Twig templates in `templates/` for public/auth pages (home, login, register, etc.)
- **Auth**: `Waaseyaa\Auth\AuthManager` for session-based auth, `TwoFactorManager` for 2FA
- **Billing**: `Waaseyaa\Billing\BillingManager` + Stripe integration

## Key Conventions

### Go
- Use `any` not `interface{}` (revive linter)
- No magic numbers — extract to named constants (mnd linter)
- Max 150 character line length (lll linter)
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Structured logging only (Zap) — never `fmt.Printf` or `log.Printf`
- 40+ golangci-lint v2 linters enabled (`.golangci.yml`)

### Waaseyaa/PHP
- PHP 8.4 constructor property promotion
- Explicit return types on all methods
- No Illuminate/Laravel imports — this is a Waaseyaa app (see `.claude/rules/waaseyaa-invariants.md`)
- Entity persistence via `SqlEntityStorage` + `EntityRepository` (see `docs/specs/user-persistence.md`)
- Use `getenv()` / `env()` helper — NEVER `$_ENV`
- PHPUnit for tests

### Frontend/CSS
- Tailwind CSS v4 with `@tailwindcss/vite` plugin (config via CSS `@theme`, not tailwind.config.js)
- shadcn-vue + reka-ui components
- CSS variables for theming: `--primary`, `--foreground`, `--background`, etc.
- Form.io Bootstrap CSS isolated in `layer(formio)` — being migrated to Tailwind via goformx-formio templates

## Server Access

- `deployer@coforge.xyz` — app deployment, file management (no sudo)
- `jones@northcloud.one` — sudo operations (Caddy reload, file ownership, PHP-FPM)
- Caddy reload: `ssh jones@northcloud.one "sudo caddy reload --config /etc/caddy/Caddyfile"`

## Gotchas

- **Never use `$_ENV` in app code** — Waaseyaa's `EnvLoader` only populates `putenv()`/`getenv()`. Use `getenv()` or the `env()` helper. `$_ENV` silently returns `null` and falls through to wrong defaults.
- **Caddy log ownership** — Log dirs and files must be `caddy:caddy`. Caddy reload fails with permission denied if deployer owns them.
- **SQLite write access** — Both the `.sqlite` file AND its parent directory need `www-data` group write for WAL/journal files.
- **Ansible Caddy pattern** — Each app deploys its own `Caddyfile` to `/home/deployer/<app>/Caddyfile`. Main `/etc/caddy/Caddyfile` imports them via glob. New services need a Caddyfile or they have no reverse proxy.
- **Route priority** — Public routes in `registerPublicRoutes()` shadow auth routes for the same path. If both need `/forms/{id}`, handle auth check in the public route handler (#62).
- **Vite base URL** — `vite.config.ts` must set `base: '/build/'` so dynamic imports resolve to `/build/assets/` not `/assets/`.
- **UserRepository uses raw PDO** — Migration debt. See `docs/specs/user-persistence.md` for the migration plan to Waaseyaa entity storage.

## Codified Context

This repo uses a three-tier codified context system:

| Tier | Location | Purpose |
|------|----------|---------|
| **Constitution** | `CLAUDE.md` (this file) | Architecture, conventions, orchestration |
| **Rules** | `.claude/rules/*.md` | Silent invariants (always active, never cited) |
| **Specs** | `docs/specs/*.md` | Domain contracts for each subsystem |

When modifying a subsystem, update its spec in the same PR.

## Troubleshooting Resources

Solution documents for past issues are in `docs/solutions/`. Check these before debugging similar problems.
