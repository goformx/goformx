# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GoFormX is a forms management platform organized as a monorepo with two services:

- **`goforms/`** — Go API backend (Echo, GORM, Uber FX). Owns the entire forms domain: CRUD, schema storage, submissions, public embed/submit. API-only, no UI.
- **`goformx-laravel/`** — Laravel 12 + Vue 3 + Inertia v2 frontend. Handles identity (Fortify auth, 2FA), user dashboard, form builder UI (Form.io), and settings.

Each service has its own CLAUDE.md with detailed development instructions. Read those when working within a specific service.

## Development Commands

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

### GoFormX-Laravel (PHP/Vue Frontend) — runs on :8000

```bash
cd goformx-laravel
composer install && npm install
composer run dev          # Starts Laravel server + queue + Pail logs + Vite concurrently
php artisan test --compact                          # Run all Pest tests
php artisan test --compact --filter=TestName         # Single test
vendor/bin/pint --dirty --format agent               # PHP formatting (Pint)
npm run lint              # ESLint
npm run format            # Prettier
npm run build             # Production frontend build
```

## Cross-Service Architecture

```
Browser → Laravel (:8000) → GoFormsClient → GoForms API (:8090) → PostgreSQL
              │                                    │
         Users/Sessions DB                    Forms/Submissions DB
```

### Assertion-Based Authentication

Laravel authenticates users (Fortify) and signs every request to Go with HMAC headers:

- `X-User-Id` — authenticated user's ID
- `X-Timestamp` — current UTC timestamp
- `X-Signature` — HMAC-SHA256 of `user_id:timestamp` using shared secret

Go middleware (`internal/application/middleware/assertion/`) verifies the signature and extracts the user ID for ownership checks. Timestamp skew tolerance: 60 seconds.

**Critical shared config**: `GOFORMS_SHARED_SECRET` must match in both `.env` files.

### GoFormsClient (Laravel → Go)

Located at `app/Services/GoFormsClient.php`. Config: `GOFORMS_API_URL` (default `http://localhost:8090`), `GOFORMS_SHARED_SECRET`.

Usage: `GoFormsClient::fromConfig()->withUser(auth()->user())->listForms()`.

### Database Separation

- **Laravel DB** (SQLite in dev): users, sessions, password resets
- **Go DB** (PostgreSQL 17): forms, form_submissions, users (shadow-synced from Laravel assertions)

Laravel never touches form tables; Go never touches auth tables.

## API Surface (Go)

**Authenticated** (assertion headers required, called by Laravel only):
- `GET/POST /api/forms`, `GET/PUT/DELETE /api/forms/:id`
- `GET /api/forms/:id/submissions`, `GET /api/forms/:id/submissions/:sid`

**Public** (no auth, rate limited):
- `GET /forms/:id/schema`, `POST /forms/:id/submit`, `GET /forms/:id/embed`

## Go Architecture (Clean Architecture + Uber FX)

```
internal/
├── domain/           # Entities, service interfaces, repository interfaces
│   ├── form/         # Form + FormSubmission models, service, repository
│   ├── user/         # User model, service, syncer
│   └── common/       # Shared errors, events, interfaces
├── application/      # HTTP handlers, middleware, validation, response builders
│   ├── handlers/web/ # REST handlers (implement web.Handler interface)
│   ├── middleware/    # Assertion, security, CORS, access control
│   └── validation/   # Form schema validation
└── infrastructure/   # GORM repos, Viper config, Zap logging, Echo server, event bus
```

Dependencies point inward: Infrastructure → Application → Domain. FX wires everything in `main.go` using module groups.

Handlers implement `web.Handler` (Register/Start/Stop) and are collected via FX groups (`group:"handlers"`).

## Laravel Architecture

- **Controllers** are thin: auth user → call GoFormsClient → Inertia render or redirect
- **FormController** catches `RequestException` from Go and maps status codes (422→validation, 404→not found, 5xx→flash message)
- **Frontend**: Vue 3 pages in `resources/js/pages/`, layouts in `resources/js/layouts/`, shadcn-vue components
- **Routes**: Wayfinder generates type-safe TypeScript route functions from Laravel routes (import from `@/actions/` or `@/routes/`)
- **Form builder**: Form.io integration (`@goformx/formio` wrapper) in `Forms/Edit.vue`

## Key Conventions

### Go
- Use `any` not `interface{}` (revive linter)
- No magic numbers — extract to named constants (mnd linter)
- Max 150 character line length (lll linter)
- Error wrapping: `fmt.Errorf("context: %w", err)`
- Structured logging only (Zap) — never `fmt.Printf` or `log.Printf`
- 40+ golangci-lint v2 linters enabled (`.golangci.yml`)

### Laravel/PHP
- PHP 8.4 constructor property promotion
- Explicit return types on all methods
- Form Request classes for validation (never inline)
- Use `config()` not `env()` outside config files
- Pest for tests, Pint for formatting
- Run `vendor/bin/pint --dirty --format agent` before finalizing changes
