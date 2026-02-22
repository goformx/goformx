# Connect Personal Site Forms to Production GoForms — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the contact form and newsletter subscribe form on https://jonesrussell.github.io/me/ submit to the production GoForms API at https://api.goformx.com.

**Architecture:** The personal site (SvelteKit, GitHub Pages) makes direct `POST` requests to the GoForms public API. GoForms validates submissions against form schemas stored in PostgreSQL. CORS is configured per-form to allow `https://jonesrussell.github.io`.

**Tech Stack:** PostgreSQL 17 (Docker), GoForms API (Go/Echo), SvelteKit 5, GitHub Actions CI/CD

---

### Task 1: Run database migrations on production

The production PostgreSQL container has no application tables. Copy migration files and execute them.

**Files:**
- Source: `goforms/migrations/postgresql/*.up.sql` (11 files)

**Step 1: Copy migration files to production server**

```bash
scp -r goforms/migrations/postgresql deployer@coforge.xyz:~/goforms/migrations/
```

**Step 2: Run migrations in order via docker exec**

Run each migration file through psql inside the postgres container. The order matters due to FK dependencies.

```bash
ssh deployer@coforge.xyz 'for f in \
  1970010100_create_update_updated_at_function.up.sql \
  1970010101_create_users_table.up.sql \
  1970010102_create_users_updated_at_trigger.up.sql \
  1983010101_create_forms_table.up.sql \
  1983010102_create_forms_updated_at_trigger.up.sql \
  1991080601_create_form_submissions.up.sql \
  1991080602_create_form_submissions_updated_at_trigger.up.sql \
  2004020401_create_form_schemas.up.sql \
  2004020402_create_form_schemas_updated_at_trigger.up.sql \
  2004020501_add_cors_to_forms.up.sql \
  2026022101_add_status_metadata_to_form_submissions.up.sql; do \
  echo "Running $f..."; \
  docker exec -i goforms-postgres-1 psql -U goforms -d goforms < ~/goforms/migrations/postgresql/$f; \
done'
```

Skip `1983010103_add_status_to_forms.up.sql` — the CREATE TABLE in `1983010101` already has the `status` column.

Expected: Each file outputs SQL results (CREATE TABLE, CREATE INDEX, CREATE TRIGGER, ALTER TABLE).

**Step 3: Verify tables were created**

```bash
ssh deployer@coforge.xyz "docker exec goforms-postgres-1 psql -U goforms -d goforms -c '\dt'"
```

Expected output: tables `users`, `forms`, `form_submissions`, `form_schemas` listed.

---

### Task 2: Insert system user and forms into production database

Create a system user (FK requirement) and two forms with schemas and CORS config.

**Step 1: Generate UUIDs locally**

```bash
SYSTEM_USER_UUID=$(uuidgen)
CONTACT_FORM_UUID=$(uuidgen)
NEWSLETTER_FORM_UUID=$(uuidgen)
echo "User: $SYSTEM_USER_UUID"
echo "Contact: $CONTACT_FORM_UUID"
echo "Newsletter: $NEWSLETTER_FORM_UUID"
```

Save these — the form UUIDs are needed in Task 4.

**Step 2: Insert system user**

```bash
ssh deployer@coforge.xyz "docker exec -i goforms-postgres-1 psql -U goforms -d goforms" << 'EOSQL'
INSERT INTO users (uuid, email, hashed_password, first_name, last_name, role, active)
VALUES (
  '<SYSTEM_USER_UUID>',
  'system@jonesrussell.github.io',
  'NOLOGIN',
  'System',
  'Forms',
  'system',
  true
);
EOSQL
```

Replace `<SYSTEM_USER_UUID>` with the UUID from Step 1.

**Step 3: Insert contact form**

```bash
ssh deployer@coforge.xyz "docker exec -i goforms-postgres-1 psql -U goforms -d goforms" << 'EOSQL'
INSERT INTO forms (uuid, user_id, title, description, schema, active, status, cors_origins, cors_methods, cors_headers)
VALUES (
  '<CONTACT_FORM_UUID>',
  '<SYSTEM_USER_UUID>',
  'Contact Form',
  'Personal website contact form',
  '{"type":"object","properties":{"email":{"type":"string"},"message":{"type":"string"},"referral":{"type":"string"}},"required":["email","message"]}',
  true,
  'published',
  '{"origins":["https://jonesrussell.github.io"]}',
  '{"methods":["GET","POST","OPTIONS"]}',
  '{"headers":["Content-Type","Accept","Origin"]}'
);
EOSQL
```

Replace `<CONTACT_FORM_UUID>` and `<SYSTEM_USER_UUID>` with UUIDs from Step 1.

**Step 4: Insert newsletter form**

```bash
ssh deployer@coforge.xyz "docker exec -i goforms-postgres-1 psql -U goforms -d goforms" << 'EOSQL'
INSERT INTO forms (uuid, user_id, title, description, schema, active, status, cors_origins, cors_methods, cors_headers)
VALUES (
  '<NEWSLETTER_FORM_UUID>',
  '<SYSTEM_USER_UUID>',
  'Newsletter Signup',
  'Personal website newsletter subscription',
  '{"type":"object","properties":{"email":{"type":"string"}},"required":["email"]}',
  true,
  'published',
  '{"origins":["https://jonesrussell.github.io"]}',
  '{"methods":["GET","POST","OPTIONS"]}',
  '{"headers":["Content-Type","Accept","Origin"]}'
);
EOSQL
```

Replace `<NEWSLETTER_FORM_UUID>` and `<SYSTEM_USER_UUID>` with UUIDs from Step 1.

**Step 5: Verify forms were inserted**

```bash
ssh deployer@coforge.xyz "docker exec goforms-postgres-1 psql -U goforms -d goforms -c \"SELECT uuid, title, status, cors_origins FROM forms;\""
```

Expected: 2 rows — Contact Form and Newsletter Signup, both with status `published` and correct CORS origins.

---

### Task 3: Smoke test the production API endpoints

Verify the public endpoints return valid responses for the new forms.

**Step 1: Test schema endpoint for contact form**

```bash
curl -s https://api.goformx.com/forms/<CONTACT_FORM_UUID>/schema | python3 -m json.tool
```

Expected: JSON response with `success: true` and form schema in `data`.

**Step 2: Test schema endpoint for newsletter form**

```bash
curl -s https://api.goformx.com/forms/<NEWSLETTER_FORM_UUID>/schema | python3 -m json.tool
```

Expected: Same structure with newsletter schema.

**Step 3: Test form submission with CORS origin header**

```bash
curl -s -X POST https://api.goformx.com/forms/<CONTACT_FORM_UUID>/submit \
  -H "Content-Type: application/json" \
  -H "Origin: https://jonesrussell.github.io" \
  -d '{"email":"test@example.com","message":"Smoke test from curl"}' \
  | python3 -m json.tool
```

Expected: `{"success": true, "message": "Form submitted successfully", "data": {"submission_id": "...", "status": "pending", ...}}`

**Step 4: Verify CORS rejection for unauthorized origin**

```bash
curl -s -X POST https://api.goformx.com/forms/<CONTACT_FORM_UUID>/submit \
  -H "Content-Type: application/json" \
  -H "Origin: https://evil.example.com" \
  -d '{"email":"test@example.com","message":"Should be rejected"}' \
  -w "\nHTTP Status: %{http_code}\n"
```

Expected: HTTP 403 Forbidden.

**Step 5: Verify submission was stored**

```bash
ssh deployer@coforge.xyz "docker exec goforms-postgres-1 psql -U goforms -d goforms -c \"SELECT uuid, form_id, status, data FROM form_submissions;\""
```

Expected: 1 row from the smoke test in Step 3. Optionally delete it:

```bash
ssh deployer@coforge.xyz "docker exec goforms-postgres-1 psql -U goforms -d goforms -c \"DELETE FROM form_submissions;\""
```

---

### Task 4: Update personal site — swap placeholder for newsletter CTA

**Files:**
- Modify: `/home/fsd42/dev/me/src/routes/+layout.svelte`

**Step 1: Replace GoFormXPlaceholder import with NewsletterCTA**

In `/home/fsd42/dev/me/src/routes/+layout.svelte`, change line 6:

```diff
-	import GoFormXPlaceholder from '$lib/components/forms/GoFormXPlaceholder.svelte';
+	import NewsletterCTA from '$lib/components/newsletter/NewsletterCTA.svelte';
```

**Step 2: Replace the component usage in the template**

In the same file, replace lines 64-69:

```diff
-		<GoFormXPlaceholder
-			title="Stay Updated"
-			description="Subscribe to my newsletter for updates on web development, tech insights, and open source projects."
-			variant="section"
-			class="newsletter-cta"
-		/>
+		<NewsletterCTA class="newsletter-cta" />
```

The `NewsletterCTA` component has its own title/description hardcoded inside `NewsletterHeader`.

**Step 3: Verify the component renders locally**

```bash
cd /home/fsd42/dev/me && npm run dev
```

Open `http://localhost:5173/me/` in a browser. Confirm the newsletter CTA appears at the bottom of the page instead of the "GoFormX (Coming Soon)" placeholder.

**Step 4: Run lint and type check**

```bash
cd /home/fsd42/dev/me && npm run check && npm run lint
```

Expected: No errors.

**Step 5: Run unit tests**

```bash
cd /home/fsd42/dev/me && npm run test:unit:run
```

Expected: All tests pass.

**Step 6: Commit**

```bash
cd /home/fsd42/dev/me
git add src/routes/+layout.svelte
git commit -m "feat: replace GoFormX placeholder with live newsletter CTA"
```

---

### Task 5: Add GoForms env vars to GitHub Actions deploy workflow

**Files:**
- Modify: `/home/fsd42/dev/me/.github/workflows/deploy.yml`

**Step 1: Add VITE env vars to the build step**

In `/home/fsd42/dev/me/.github/workflows/deploy.yml`, update the `Build` step env block (lines 51-53):

```diff
       - name: Build
         env:
           BASE_PATH: '/me'
           NODE_ENV: production
+          VITE_GOFORMS_API_URL: 'https://api.goformx.com'
+          VITE_GOFORMS_CONTACT_FORM_ID: '<CONTACT_FORM_UUID>'
+          VITE_GOFORMS_NEWSLETTER_FORM_ID: '<NEWSLETTER_FORM_UUID>'
         run: npm run build
```

Replace `<CONTACT_FORM_UUID>` and `<NEWSLETTER_FORM_UUID>` with the UUIDs from Task 2 Step 1.

Note: These are not secrets — they are public form IDs embedded in client-side JavaScript. The forms are protected by CORS origin validation and schema-based input validation on the server.

**Step 2: Verify the workflow YAML is valid**

```bash
cd /home/fsd42/dev/me && python3 -c "import yaml; yaml.safe_load(open('.github/workflows/deploy.yml'))" && echo "Valid YAML"
```

Expected: "Valid YAML"

**Step 3: Commit**

```bash
cd /home/fsd42/dev/me
git add .github/workflows/deploy.yml
git commit -m "ci: add GoForms API URL and form IDs to production build env"
```

---

### Task 6: Push and verify deployment

**Step 1: Push the personal site changes**

```bash
cd /home/fsd42/dev/me && git push origin main
```

This triggers the GitHub Actions pipeline: build → unit-tests + e2e-tests → deploy.

**Step 2: Monitor CI/CD**

```bash
cd /home/fsd42/dev/me && gh run watch
```

Expected: All jobs pass (build, unit-tests, e2e-tests, deploy).

**Step 3: Verify live site**

1. Open https://jonesrussell.github.io/me/ — newsletter CTA should appear at bottom
2. Open https://jonesrussell.github.io/me/contact — contact form should be visible
3. Open browser DevTools Network tab
4. Submit the newsletter form with a test email — should POST to `api.goformx.com` and show success
5. Submit the contact form with test data — should POST to `api.goformx.com` and show success

**Step 4: Verify submissions landed in database**

```bash
ssh deployer@coforge.xyz "docker exec goforms-postgres-1 psql -U goforms -d goforms -c \"SELECT uuid, form_id, status, data FROM form_submissions ORDER BY submitted_at DESC LIMIT 5;\""
```

Expected: Recent test submissions visible with `status = 'pending'` and correct form data.

---

### Cleanup (optional)

- Delete test submissions from the smoke tests:
  ```bash
  ssh deployer@coforge.xyz "docker exec goforms-postgres-1 psql -U goforms -d goforms -c \"DELETE FROM form_submissions WHERE data::text LIKE '%Smoke test%' OR data::text LIKE '%test@example%';\""
  ```
- Delete the `GoFormXPlaceholder.svelte` component from the personal site if it's no longer referenced anywhere.
