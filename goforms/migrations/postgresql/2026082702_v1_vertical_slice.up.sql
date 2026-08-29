ALTER TABLE forms ADD COLUMN IF NOT EXISTS name VARCHAR(63);
UPDATE forms
SET name = 'form-' || left(replace(uuid, '-', ''), 12)
WHERE name IS NULL OR btrim(name) = '';
ALTER TABLE forms ALTER COLUMN name SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS forms_owner_name_unique
    ON forms (user_id, name) WHERE deleted_at IS NULL;

ALTER TABLE form_submissions DROP CONSTRAINT IF EXISTS form_submissions_status_check;
UPDATE form_submissions SET status = 'accepted' WHERE status <> 'accepted';
ALTER TABLE form_submissions ADD CONSTRAINT form_submissions_status_check
    CHECK (status = 'accepted');
