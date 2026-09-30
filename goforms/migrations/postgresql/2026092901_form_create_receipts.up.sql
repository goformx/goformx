-- A durable receipt prevents an uncertain create retry from making another form.
-- Retain the response snapshot even if the form is later edited or soft deleted.
CREATE TABLE form_create_receipts (
    organization_id VARCHAR(36) NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    request_digest TEXT NOT NULL,
    form_id VARCHAR(36) NOT NULL,
    form_snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, idempotency_key)
);
