---
name: cross-service-test
description: Run Go + Laravel test suites across the GoFormX monorepo
user-invocable: true
disable-model-invocation: true
arguments:
  - name: scope
    description: "Optional: --go-only or --laravel-only to run a single suite"
---

# Cross-Service Test Runner

Run the full GoFormX test suite spanning both Go and Laravel services.

## Usage

```
/cross-service-test              # Run all tests (Go + Laravel)
/cross-service-test --go-only    # Go tests only
/cross-service-test --laravel-only  # Laravel tests only
```

## Execution

### All tests (default)

Run from the monorepo root:

```bash
task test
```

This runs Go and Laravel test suites in parallel via Taskfile.

### Go only (`--go-only`)

```bash
cd goforms && task test:backend
```

### Laravel only (`--laravel-only`)

```bash
cd goformx-laravel && ddev artisan test --compact
```

## Before Running

1. Ensure DDEV is running: `ddev status` (start with `ddev start` if needed)
2. For Go tests, ensure mocks are generated: `cd goforms && task generate`

## Interpreting Results

- Report total pass/fail counts for each service
- If both services pass, confirm with a summary
- If either fails, show the failing test names and relevant output

## Troubleshooting Common Failures

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| `connection refused` on port 8090 | Go sidecar not running | `ddev start` (sidecar auto-starts) |
| `mock not found` or interface mismatch | Stale mocks | `cd goforms && task generate` |
| `DDEV not running` | DDEV containers stopped | `ddev start` |
| `table not found` in Go tests | Migrations not applied | `cd goforms && task migrate:up` |
| `Class not found` in Laravel | Autoload stale | `ddev composer dump-autoload` |
| Assertion auth failures in integration | Shared secret mismatch | Compare `GOFORMS_SHARED_SECRET` in both `.env` files |
