# Running the Go forms sidecar with DDEV

The Laravel app talks to a **Go forms API** (goforms). In local development we run that API as a **DDEV sidecar**: custom services in the same project so the web container can reach it at `http://goforms:8090`.

## Prerequisites

- **goforms repo** as a sibling of this repo, e.g.:
  ```text
  dev/
  ├── goformx-laravel/   ← this app
  └── goforms/           ← Go API (sibling)
  ```
- DDEV (e.g. `ddev start` from the Laravel project).

## How the sidecar is defined

The sidecar is declared in **`.ddev/docker-compose.goforms.yaml`**:

- **goforms-db** – Postgres for the Go app (volume: `goformx-laravel-goforms-db-data`).
- **goforms** – Go API: build from `../../goforms`, mount repo at `/app`, run with `air`. Listens on `0.0.0.0:8090`.
- **web** – Gets `GOFORMS_API_URL=http://goforms:8090` so Laravel uses the sidecar.

DDEV merges this file with its base compose on every `ddev start` / `ddev restart`, so no extra commands are needed to “enable” the sidecar.

## Starting and stopping

- **Start (including sidecar):**
  ```bash
  cd /path/to/goformx-laravel
  ddev start
  ```
  Or after a config change:
  ```bash
  ddev restart
  ```

- **Stop everything:**
  ```bash
  ddev stop
  ```

The goforms service starts after **goforms-db** is healthy. The first time, the Go binary is built inside the container (often 30–90 seconds); the healthcheck has a 90s start period so DDEV waits.

## Checking the sidecar

- **Status and URLs:**
  ```bash
  ddev describe
  ```
  Look for services `goforms` and `goforms-db` and the note that Laravel uses `http://goforms:8090`.

- **Go API logs:**
  ```bash
  ddev logs -s goforms
  ```
  Or with Docker:
  ```bash
  docker logs ddev-goformx-laravel-goforms
  ```

- **Database (goforms-db):**
  ```bash
  ddev exec --service=goforms-db psql -U goforms -d goforms -c '\dt'
  ```

## Configuration

- **Laravel `.env`** (in this repo) must have:
  - `GOFORMS_API_URL=http://goforms:8090` (set automatically via the compose override for the web service; you can keep it in `.env` for clarity).
  - `GOFORMS_SHARED_SECRET=ddev-goforms-secret` so signed requests match the Go service.

- **Go service** receives the same secret from the compose file (`GOFORMS_SHARED_SECRET: "ddev-goforms-secret"`). Do not change one without the other or you will get 401s.

## Go forms database migrations

The sidecar Postgres starts empty. To run goforms migrations (e.g. from the goforms repo’s migrate task) against the sidecar DB from the host:

```bash
# From goformx-laravel
ddev exec --service=goforms -- sh -c 'cd /app && migrate -path migrations/postgresql -database "postgres://goforms:goforms@goforms-db:5432/goforms?sslmode=disable" up'
```

Or use the goforms repo’s Taskfile with `DB_HOST=goforms-db` and run the migrate task from inside the DDEV web container or a one-off goforms container.

## Troubleshooting

| Issue | What to check |
|-------|----------------|
| 401 on `/api/forms` | Laravel and goforms must use the same `GOFORMS_SHARED_SECRET`. In this setup both use `ddev-goforms-secret`. Run `ddev restart` after changing `.env`. The Forms index page still loads with an empty list when Go returns 401 or 404 for list forms (see `storage/logs/laravel.log` for warnings). |
| “path ... goforms not found” on build | Ensure the **goforms** repo exists as a sibling of **goformx-laravel** (paths in the compose file are relative to `.ddev/`). |
| goforms container exits or unhealthy | Run `ddev logs -s goforms` and fix Go build or DB connection errors. Ensure goforms-db is healthy first. |
| Laravel “Form service temporarily unavailable” | Sidecar may still be starting (first build). Wait and retry, or check `ddev describe` and `ddev logs -s goforms`. |
