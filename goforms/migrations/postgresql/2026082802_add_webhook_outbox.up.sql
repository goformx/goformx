CREATE TABLE webhook_endpoints (
    uuid VARCHAR(36) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    form_id VARCHAR(36) NOT NULL UNIQUE REFERENCES forms(uuid) ON DELETE CASCADE,
    destination_origin TEXT NOT NULL,
    encrypted_config BYTEA NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER webhook_endpoints_set_updated_at
    BEFORE UPDATE ON webhook_endpoints
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE webhook_deliveries (
    uuid VARCHAR(36) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    submission_id VARCHAR(36) NOT NULL UNIQUE REFERENCES form_submissions(uuid) ON DELETE CASCADE,
    form_id VARCHAR(36) NOT NULL REFERENCES forms(uuid) ON DELETE CASCADE,
    endpoint_id VARCHAR(36) NOT NULL,
    destination_origin TEXT NOT NULL,
    encrypted_config BYTEA NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'delivered', 'dead_letter')),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    last_http_status INTEGER CHECK (last_http_status BETWEEN 100 AND 599),
    last_error_category VARCHAR(40) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX webhook_deliveries_claim_idx
    ON webhook_deliveries (next_attempt_at, created_at)
    WHERE status IN ('pending', 'processing');
CREATE INDEX webhook_deliveries_form_created_idx
    ON webhook_deliveries (form_id, created_at DESC, uuid DESC);

CREATE TRIGGER webhook_deliveries_set_updated_at
    BEFORE UPDATE ON webhook_deliveries
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
