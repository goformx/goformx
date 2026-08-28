ALTER TABLE service_tokens
    ADD COLUMN IF NOT EXISTS replaced_by_token_id TEXT REFERENCES service_tokens (token_id),
    ADD COLUMN IF NOT EXISTS revocation_reason TEXT;

CREATE INDEX IF NOT EXISTS service_tokens_replaced_by_token_id_idx
    ON service_tokens (replaced_by_token_id);
