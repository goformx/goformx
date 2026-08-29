-- GoFormX stores opaque Waaseyaa organization UUIDs, not human identities.
-- Existing personal-owner UUIDs are preserved as organization UUIDs so production
-- data can be mapped without rewriting resource identifiers.
ALTER TABLE forms DROP CONSTRAINT IF EXISTS forms_user_id_fkey;
DROP INDEX IF EXISTS idx_forms_user_id;
DROP INDEX IF EXISTS forms_owner_name_unique;
ALTER TABLE forms RENAME COLUMN user_id TO organization_id;
CREATE INDEX IF NOT EXISTS forms_organization_id_idx ON forms (organization_id);
CREATE UNIQUE INDEX IF NOT EXISTS forms_organization_name_unique
    ON forms (organization_id, name) WHERE deleted_at IS NULL;

ALTER TABLE service_tokens DROP CONSTRAINT IF EXISTS service_tokens_owner_id_fkey;
DROP INDEX IF EXISTS service_tokens_owner_id_idx;
ALTER TABLE service_tokens RENAME COLUMN owner_id TO organization_id;
CREATE INDEX IF NOT EXISTS service_tokens_organization_id_idx ON service_tokens (organization_id);
