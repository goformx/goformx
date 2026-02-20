# Monorepo Conversion Design

**Date**: 2026-02-19
**Status**: Approved

## Summary

Merge two independent repositories (`goformx/goforms` and `goformx/goformx-laravel`) into a single monorepo at `goformx/goformx` using `git subtree`, preserving full commit history. Add shared tooling at the root. Archive the original repos.

## Current State

- `goformx/goforms` — Go API backend, 1,760 commits, Echo + GORM + Uber FX
- `goformx/goformx-laravel` — Laravel 12 + Vue 3 frontend, 50 commits, Fortify + Inertia + Form.io
- Both live under `/home/fsd42/dev/goformx/` as sibling directories (no parent git repo)
- Shared secret (`GOFORMS_SHARED_SECRET`) and API URL couple the two services
- Each has independent GitHub Actions workflows, Docker configs, and dev environments

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Merge strategy | `git subtree add` | Preserves full history, no extra tooling |
| New repo | `goformx/goformx` | Clean name for the unified project |
| Go module path | Keep `github.com/goformx/goforms` | Works fine as subdirectory module; avoids rewriting all imports |
| Dev environment | Keep DDEV for Laravel | Least disruption; DDEV handles PHP/MariaDB/Nginx well |
| CI strategy | Separate workflows + path filters | Simple migration; each service triggers independently |

## Target Structure

```
goformx/                            # goformx/goformx repo
├── goforms/                        # Subtree from goformx/goforms (full history)
│   ├── main.go
│   ├── Taskfile.yml                # Service-level tasks (unchanged)
│   ├── go.mod                      # github.com/goformx/goforms (unchanged)
│   ├── .golangci.yml
│   ├── docker-compose.yml          # Go + PostgreSQL dev compose (unchanged)
│   ├── internal/
│   ├── migrations/
│   └── ...
├── goformx-laravel/                # Subtree from goformx/goformx-laravel (full history)
│   ├── artisan
│   ├── composer.json
│   ├── .ddev/                      # DDEV config (unchanged, paths still resolve)
│   │   ├── config.yaml
│   │   └── docker-compose.goforms.yaml
│   ├── resources/js/
│   └── ...
├── .github/
│   └── workflows/
│       ├── goforms-ci.yml          # From goforms ci-cd.yml + paths filter
│       ├── laravel-ci.yml          # From laravel tests.yml + paths filter
│       ├── laravel-lint.yml        # From laravel lint.yml + paths filter
│       ├── security.yml            # Weekly scans, adapted for monorepo
│       ├── claude.yml              # Merged from both repos
│       └── claude-code-review.yml  # Merged from both repos
├── Taskfile.yml                    # Root orchestrator
├── .env.example                    # Shared secrets reference
├── .gitignore                      # Combined from both repos
├── CLAUDE.md                       # Updated for monorepo
└── LICENSE
```

## Subtree Merge Procedure

1. Create empty repo `goformx/goformx` on GitHub (no README, no .gitignore)
2. Clone locally and set up:
   ```bash
   git clone git@github.com:goformx/goformx.git
   cd goformx
   git remote add goforms-origin git@github.com:goformx/goforms.git
   git remote add laravel-origin git@github.com:goformx/goformx-laravel.git
   git fetch goforms-origin
   git fetch laravel-origin
   ```
3. Subtree-add each repo:
   ```bash
   git subtree add --prefix=goforms goforms-origin main
   git subtree add --prefix=goformx-laravel laravel-origin main
   ```
4. Both repos' full histories appear on the new `main` branch, with paths prefixed to their subdirectories.
5. Only `main` branch from each repo is merged. Active feature branches must be finished or manually rebased after cutover.

## CI Workflows

All workflows live at the repo root `.github/workflows/`. Per-service `.github/` directories are removed.

### goforms-ci.yml

Adapted from `goforms/.github/workflows/ci-cd.yml`:
- Trigger: `push` and `pull_request` with `paths: ['goforms/**']`
- All `run` steps use `working-directory: ./goforms`
- Docker build context: `./goforms`
- Jobs unchanged: test, build, docker (ghcr.io push), release (GitHub release on tags)
- Existing `dorny/paths-filter` step provides additional fine-grained filtering within the workflow

### laravel-ci.yml

Adapted from `goformx-laravel/.github/workflows/tests.yml`:
- Trigger: `paths: ['goformx-laravel/**']`
- PHP 8.4 + 8.5 matrix
- `working-directory: ./goformx-laravel`
- Jobs: composer install, npm install + build, Pest tests

### laravel-lint.yml

Adapted from `goformx-laravel/.github/workflows/lint.yml`:
- Trigger: `paths: ['goformx-laravel/**']`
- `working-directory: ./goformx-laravel`
- Jobs: Pint formatting, ESLint, Prettier

### security.yml

Adapted from `goforms/.github/workflows/security.yml`:
- Weekly cron trigger (no path filter needed)
- CodeQL matrix for Go + JavaScript, scoped to subdirectories

### claude.yml / claude-code-review.yml

Merged into single versions from both repos (nearly identical configs). No path filtering — Claude reviews whatever changed.

## Root Taskfile

Thin orchestrator that delegates to each service:

```yaml
version: "3"

includes:
  go:
    taskfile: ./goforms/Taskfile.yml
    dir: ./goforms

tasks:
  dev:
    desc: Start both services concurrently
    deps: [dev:go, dev:laravel]

  dev:go:
    desc: Start Go backend with hot reload
    dir: ./goforms
    cmd: task dev:backend

  dev:laravel:
    desc: Start Laravel with Vite
    dir: ./goformx-laravel
    cmd: composer run dev

  install:
    desc: Install dependencies for both services
    cmds:
      - task: install:go
      - task: install:laravel

  install:go:
    dir: ./goforms
    cmd: task install

  install:laravel:
    dir: ./goformx-laravel
    cmds:
      - composer install
      - npm install

  test:
    desc: Run all test suites
    deps: [test:go, test:laravel]

  test:go:
    dir: ./goforms
    cmd: task test:backend

  test:laravel:
    dir: ./goformx-laravel
    cmd: php artisan test --compact

  lint:
    desc: Run all linters
    deps: [lint:go, lint:laravel]

  lint:go:
    dir: ./goforms
    cmd: task lint:backend

  lint:laravel:
    dir: ./goformx-laravel
    cmds:
      - vendor/bin/pint --dirty --format agent
      - npm run lint

  setup:
    desc: Full first-time setup
    cmds:
      - task: install
      - task: go:generate
      - task: go:migrate:up
```

All goforms tasks are also accessible via the `go:` namespace (e.g., `task go:build`, `task go:migrate:up`).

## Shared Config

### .env.example (root)

Reference file only — each service keeps its own `.env`:

```
# Shared between goforms/.env and goformx-laravel/.env
# These values MUST match in both files
GOFORMS_SHARED_SECRET=your-secret-here
```

### .gitignore (root)

Combined from both repos:
- Go: `bin/`, `dist/`, `coverage/`, `.task/`
- Laravel: `vendor/`, `node_modules/`, `.env`, `storage/*.key`
- Shared: `.env`, `.DS_Store`, IDE files

## Changes Inside Each Service

### goforms/

- **Remove**: `.github/` directory (workflows move to root)
- **Keep**: Everything else unchanged — `go.mod`, `Taskfile.yml`, `docker-compose.yml`, `.golangci.yml`, `CLAUDE.md`, all code

### goformx-laravel/

- **Remove**: `.github/` directory
- **Keep**: Everything else unchanged — `composer.json`, `.ddev/`, all code
- **DDEV paths**: `.ddev/docker-compose.goforms.yaml` references `../../goforms` which resolves correctly from `goformx/goformx-laravel/.ddev/` to `goformx/goforms/`

### Root CLAUDE.md

Update wording from "workspace with two directories" to "monorepo". Minor phrasing changes only.

## Post-Merge

1. Verify `git log --follow goforms/main.go` traces back through full history
2. Verify `git log --follow goformx-laravel/artisan` traces back through full history
3. Push to `goformx/goformx` on GitHub
4. Verify CI workflows trigger correctly with path filters
5. Verify DDEV still works from `goformx-laravel/`
6. Archive `goformx/goforms` and `goformx/goformx-laravel` on GitHub
7. Update any external references (deployment scripts, documentation) to point to new repo
