DROP INDEX IF EXISTS service_tokens_organization_id_idx;
ALTER TABLE service_tokens RENAME COLUMN organization_id TO owner_id;
CREATE INDEX IF NOT EXISTS service_tokens_owner_id_idx ON service_tokens (owner_id);

DROP INDEX IF EXISTS forms_organization_name_unique;
DROP INDEX IF EXISTS forms_organization_id_idx;
ALTER TABLE forms RENAME COLUMN organization_id TO user_id;
CREATE INDEX IF NOT EXISTS idx_forms_user_id ON forms (user_id);
CREATE UNIQUE INDEX IF NOT EXISTS forms_owner_name_unique
    ON forms (user_id, name) WHERE deleted_at IS NULL;

-- Foreign keys to the retired local users table are intentionally not restored:
-- organization UUIDs are owned by Waaseyaa and may have no local user row.
