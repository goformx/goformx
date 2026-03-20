# CLAUDE.md — goformx-web

## Overview

GoFormX web frontend built on Waaseyaa framework + Vue 3 + Inertia v3. Handles identity, dashboard, form builder UI, billing, and settings.

## Development

```bash
vendor/bin/phpunit          # Run tests (37 tests)
```

## Environment

- Uses `getenv()` / `env()` helper — NEVER `$_ENV` (EnvLoader only populates putenv)
- Config: `config/waaseyaa.php` — all env access via `env()` helper using `getenv()`
- MariaDB for users/sessions, SQLite for Waaseyaa entity storage

## Production Deployment

- Deploy path: `/home/deployer/goformx-web/` (releases + shared symlink pattern)
- `.env` symlinked from `shared/.env` into each release
- SQLite at `shared/storage/goformx.sqlite` (needs www-data group write)
- Migrations: `bin/migrate.php` (MariaDB schema), Waaseyaa auto-migrates SQLite on boot

## Architecture

- `src/AppServiceProvider.php` — routes, DI singletons, auth handlers
- `src/Service/UserRepository.php` — PDO-based MariaDB user queries
- `config/waaseyaa.php` — all app configuration
- `public/index.php` — entry point, boots HttpKernel
- `templates/` — Twig templates for SSR pages
- `frontend/` — Vue 3 + Inertia components
