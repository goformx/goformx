# Codified Context Addition & Entity Migration (2026-03-20)

## What Happened

Added three-tier codified context to GoFormX and migrated `UserRepository` from raw PDO to Waaseyaa's `EntityRepository` + `DatabaseInterface`.

## Issues Encountered

### 1. Storage symlink not created during deploy

**Symptom:** `mkdir(): Permission denied` in `PackageManifestCompiler.php` on production boot.

**Root cause:** `rsync` creates `storage/` as a real directory (contains `.gitkeep`). The deploy script's `ln -nfs` then creates a symlink *inside* the directory (`storage/storage → shared/storage`) instead of replacing it. The real `storage/` dir is owned by `deployer` but PHP-FPM runs as `www-data`.

**Fix:** Added `rm -rf "${RELEASE_PATH}/storage"` before the `ln -nfs` in `.github/workflows/goformx-web-deploy.yml`.

### 2. Entity type ID must match table name

**Symptom:** `SQLSTATE[42S02]: Table 'goformx.user' doesn't exist` on login.

**Root cause:** Waaseyaa's `SqlStorageDriver` uses the entity type ID as the SQL table name. We defined entity type ID as `user` but the MariaDB table is `users`.

**Fix:** Changed entity type ID from `user` to `users` in both `User.php` and `AppServiceProvider.php`.

### 3. Go API response nesting

**Symptom:** Form editor shows "Unable to load form: no form ID was provided." Form.io builder loads empty.

**Root cause:** The Go API wraps single-resource responses as `{"data": {"form": {...}}}` and list responses as `{"data": {"forms": [...]}}`. `FormController` was passing `$response['data']` (the wrapper) to Vue instead of `$response['data']['form']` (the actual form object). The Vue component looked for `props.form.id` but got `props.form.form.id`.

**Fix:** Updated `FormController` to unwrap the nested keys: `$response['data']['form']`, `$response['data']['forms']`, `$response['data']['submissions']`. Updated test mocks to match the real API response shape.

## Commits

- `31fe6c0b` — codified context + UserRepository migration
- `7e1c16bb` — entity type ID fix (`user` → `users`)
- `4408b673` — deploy symlink fix
- `70a73966` — Go API response unwrapping fix (#63)
