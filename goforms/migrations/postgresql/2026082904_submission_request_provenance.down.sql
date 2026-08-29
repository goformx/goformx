ALTER TABLE form_submissions DROP CONSTRAINT IF EXISTS form_submissions_request_id_check;
ALTER TABLE form_submissions DROP COLUMN IF EXISTS request_id;
