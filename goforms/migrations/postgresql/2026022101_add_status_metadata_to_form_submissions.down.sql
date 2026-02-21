-- Remove status and metadata columns from form_submissions
ALTER TABLE form_submissions
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS metadata;
