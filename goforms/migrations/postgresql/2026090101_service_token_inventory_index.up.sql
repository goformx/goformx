DROP INDEX IF EXISTS service_tokens_organization_id_idx;
CREATE INDEX IF NOT EXISTS service_tokens_inventory_idx
    ON service_tokens (organization_id, created_at DESC, token_id DESC);
