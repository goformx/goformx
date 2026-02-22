# Connect Personal Site Forms to Production GoForms

**Date**: 2026-02-22
**Status**: Approved

## Goal

Make the two forms on https://jonesrussell.github.io/me/ submit to the production GoForms API at https://api.goformx.com:

1. **Contact form** (`/contact`) — email, message, referral fields
2. **Newsletter subscribe** (layout-level CTA) — email field

## Current State

- **GoForms API** is running in Docker on production (`goforms-goforms-1` container) at `api.goformx.com`
- **PostgreSQL** container is running but has **no application tables** — the `~/goforms/migrations/` directory on the server is empty, so migrations never ran
- **Personal site** has both forms fully implemented in code but pointing to `localhost:8091`. The newsletter CTA is hidden behind a `GoFormXPlaceholder` component

## Architecture

```
Browser (GitHub Pages)
  ├── POST https://api.goformx.com/forms/<contact-id>/submit
  └── POST https://api.goformx.com/forms/<newsletter-id>/submit
        │
        ▼
  Caddy (api.goformx.com) → reverse_proxy 127.0.0.1:8090
        │
        ▼
  GoForms Docker container (goforms-goforms-1)
    ├── CORS middleware: checks form's cors_origins against Origin header
    ├── Rate limiting: per form + origin
    ├── Schema validation: validates submission data against form schema
    └── PostgreSQL (goforms-postgres-1): forms + form_submissions tables
```

## Design

### 1. Production Database Setup

Run all PostgreSQL migrations in order by copying them to the server and executing via `docker exec psql`. Migrations to apply (in order):

1. `1970010100_create_update_updated_at_function.up.sql`
2. `1970010101_create_users_table.up.sql`
3. `1970010102_create_users_updated_at_trigger.up.sql`
4. `1983010101_create_forms_table.up.sql`
5. `1983010102_create_forms_updated_at_trigger.up.sql`
6. `1991080601_create_form_submissions.up.sql`
7. `1991080602_create_form_submissions_updated_at_trigger.up.sql`
8. `2004020401_create_form_schemas.up.sql`
9. `2004020402_create_form_schemas_updated_at_trigger.up.sql`
10. `2004020501_add_cors_to_forms.up.sql`
11. `2026022101_add_status_metadata_to_form_submissions.up.sql`

Skip `1983010103_add_status_to_forms.up.sql` — the `CREATE TABLE` in step 4 already includes the `status` column.

### 2. Form Creation via Direct SQL Insert

Insert a system user (required as FK for forms) and two forms:

**System user**: UUID generated, email `system@jonesrussell.github.io`, used solely as the owner for public-facing forms.

**Contact form schema** (JSON Schema format):
```json
{
  "type": "object",
  "properties": {
    "email": {"type": "string"},
    "message": {"type": "string"},
    "referral": {"type": "string"}
  },
  "required": ["email", "message"]
}
```

**Newsletter form schema**:
```json
{
  "type": "object",
  "properties": {
    "email": {"type": "string"}
  },
  "required": ["email"]
}
```

Both forms get:
- `status = 'published'`, `active = true`
- `cors_origins = '{"origins": ["https://jonesrussell.github.io"]}'`
- `cors_methods = '{"methods": ["GET", "POST", "OPTIONS"]}'`
- `cors_headers = '{"headers": ["Content-Type", "Accept", "Origin"]}'`

### 3. Personal Site Changes (me/ repo)

**3a. GitHub Actions deploy.yml** — Add VITE env vars to the build step:
```yaml
env:
  BASE_PATH: '/me'
  NODE_ENV: production
  VITE_GOFORMS_API_URL: 'https://api.goformx.com'
  VITE_GOFORMS_CONTACT_FORM_ID: '<contact-form-uuid>'
  VITE_GOFORMS_NEWSLETTER_FORM_ID: '<newsletter-form-uuid>'
```

**3b. Replace GoFormXPlaceholder** — In `src/routes/+layout.svelte`, swap `GoFormXPlaceholder` for `NewsletterCTA` component.

**3c. No response format changes needed** — Verified that:
- `submitForm()`: Neither form reads `submission_id` from the response; both just check success/failure
- `getSchema()`: Newsletter composable only checks truthiness of result, never accesses schema properties
- Validation errors: FormService checks `body?.data?.errors` which matches the Go API format

## Risks

- **Database migrations on production**: Running SQL directly. Mitigated by using `IF NOT EXISTS` clauses in migration files.
- **CORS configuration**: If the origin doesn't match exactly, submissions will fail with 403. Must use `https://jonesrussell.github.io` (no trailing slash, no path).
- **No rollback for forms**: If something goes wrong, forms can be deactivated via SQL (`UPDATE forms SET active = false`).
