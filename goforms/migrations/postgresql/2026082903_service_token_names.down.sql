ALTER TABLE service_tokens DROP CONSTRAINT IF EXISTS service_tokens_name_check;
ALTER TABLE service_tokens DROP COLUMN IF EXISTS name;
