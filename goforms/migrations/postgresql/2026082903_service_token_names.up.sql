ALTER TABLE service_tokens ADD COLUMN IF NOT EXISTS name VARCHAR(100);
UPDATE service_tokens
SET name = 'Imported token ' || left(token_id, 12)
WHERE name IS NULL OR btrim(name) = '';
ALTER TABLE service_tokens ALTER COLUMN name SET NOT NULL;
ALTER TABLE service_tokens ADD CONSTRAINT service_tokens_name_check
    CHECK (char_length(btrim(name)) BETWEEN 1 AND 100);
