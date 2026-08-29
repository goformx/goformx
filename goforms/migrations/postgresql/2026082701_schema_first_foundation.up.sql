-- Schema-first persistence foundation. PostgreSQL is the only supported database.
ALTER TABLE forms ADD COLUMN IF NOT EXISTS public_key TEXT;
UPDATE forms SET public_key = 'gfpk_' || replace(uuid, '-', '') WHERE public_key IS NULL;
ALTER TABLE forms ALTER COLUMN public_key SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS forms_public_key_unique ON forms (public_key);

CREATE TABLE IF NOT EXISTS service_tokens (
    token_id TEXT PRIMARY KEY,
    owner_id VARCHAR(36) NOT NULL REFERENCES users (uuid) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    scopes JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    CHECK (jsonb_typeof(scopes) = 'array' AND jsonb_array_length(scopes) > 0),
    CHECK (expires_at > created_at)
);
CREATE INDEX IF NOT EXISTS service_tokens_owner_id_idx ON service_tokens (owner_id);

ALTER TABLE forms ADD COLUMN IF NOT EXISTS current_schema_version INTEGER;
ALTER TABLE forms DROP CONSTRAINT IF EXISTS forms_status_check;
ALTER TABLE forms ADD CONSTRAINT forms_status_check CHECK (status IN ('draft', 'published', 'disabled', 'archived'));

ALTER TABLE form_schemas ALTER COLUMN schema TYPE JSONB USING schema::jsonb;
ALTER TABLE form_schemas ADD COLUMN IF NOT EXISTS state TEXT NOT NULL DEFAULT 'draft';
ALTER TABLE form_schemas ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ;
DROP TRIGGER IF EXISTS update_form_schemas_updated_at ON form_schemas;
ALTER TABLE form_schemas DROP COLUMN IF EXISTS active;
ALTER TABLE form_schemas DROP COLUMN IF EXISTS updated_at;
ALTER TABLE form_schemas DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE form_schemas ADD CONSTRAINT form_schemas_state_check CHECK (state IN ('draft', 'published', 'retired'));
CREATE UNIQUE INDEX IF NOT EXISTS form_schemas_form_version_unique ON form_schemas (form_id, version);

INSERT INTO form_schemas (uuid, form_id, schema, version, state, created_at)
SELECT gen_random_uuid()::text, uuid, schema::jsonb, 1, CASE WHEN status = 'published' THEN 'published' ELSE 'draft' END, created_at
FROM forms
ON CONFLICT (form_id, version) DO NOTHING;

UPDATE forms SET current_schema_version = 1 WHERE current_schema_version IS NULL;
ALTER TABLE forms ALTER COLUMN current_schema_version SET NOT NULL;
ALTER TABLE forms DROP COLUMN schema;

ALTER TABLE form_submissions ADD COLUMN IF NOT EXISTS schema_version INTEGER;
UPDATE form_submissions SET schema_version = 1 WHERE schema_version IS NULL;
ALTER TABLE form_submissions ALTER COLUMN schema_version SET NOT NULL;
ALTER TABLE form_submissions ADD COLUMN IF NOT EXISTS idempotency_key TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS form_submissions_idempotency_unique
    ON form_submissions (form_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
ALTER TABLE form_submissions ADD CONSTRAINT form_submissions_schema_version_fk
    FOREIGN KEY (form_id, schema_version) REFERENCES form_schemas (form_id, version);

CREATE OR REPLACE FUNCTION prevent_published_schema_mutation() RETURNS trigger AS $$
BEGIN
    IF OLD.state = 'published' AND (NEW.schema IS DISTINCT FROM OLD.schema OR NEW.version <> OLD.version OR NEW.form_id <> OLD.form_id) THEN
        RAISE EXCEPTION 'published schema versions are immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS form_schemas_immutable_published ON form_schemas;
CREATE TRIGGER form_schemas_immutable_published BEFORE UPDATE ON form_schemas
FOR EACH ROW EXECUTE FUNCTION prevent_published_schema_mutation();
