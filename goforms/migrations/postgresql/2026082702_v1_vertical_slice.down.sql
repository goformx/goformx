ALTER TABLE form_submissions DROP CONSTRAINT IF EXISTS form_submissions_status_check;
DROP INDEX IF EXISTS forms_owner_name_unique;
ALTER TABLE forms DROP COLUMN IF EXISTS name;
