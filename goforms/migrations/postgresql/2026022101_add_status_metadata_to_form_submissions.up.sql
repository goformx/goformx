-- Add status and metadata columns to form_submissions
ALTER TABLE form_submissions
    ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS metadata JSONB;
