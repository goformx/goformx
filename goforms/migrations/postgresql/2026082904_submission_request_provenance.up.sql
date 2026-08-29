ALTER TABLE form_submissions ADD COLUMN IF NOT EXISTS request_id TEXT;
UPDATE form_submissions
SET request_id = 'legacy_' || replace(uuid, '-', '')
WHERE request_id IS NULL OR btrim(request_id) = '';
ALTER TABLE form_submissions ALTER COLUMN request_id SET NOT NULL;
ALTER TABLE form_submissions ADD CONSTRAINT form_submissions_request_id_check
    CHECK (char_length(request_id) BETWEEN 1 AND 128);
