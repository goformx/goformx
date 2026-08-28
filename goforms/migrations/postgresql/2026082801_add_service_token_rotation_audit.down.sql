DROP INDEX IF EXISTS service_tokens_replaced_by_token_id_idx;
ALTER TABLE service_tokens
    DROP COLUMN IF EXISTS revocation_reason,
    DROP COLUMN IF EXISTS replaced_by_token_id;
