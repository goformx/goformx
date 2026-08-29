-- Accepted submissions are immutable; delivery state lives in the webhook outbox.
ALTER TABLE form_submissions
    ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'accepted',
    ADD COLUMN IF NOT EXISTS metadata JSONB;
