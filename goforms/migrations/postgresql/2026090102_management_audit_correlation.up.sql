ALTER TABLE management_audit
    ADD COLUMN correlation_id VARCHAR(128),
    ADD CONSTRAINT management_audit_correlation_id_check CHECK (
        correlation_id IS NULL OR (credential_class = 'service_token'
            AND correlation_id ~ '^[A-Za-z0-9_-][A-Za-z0-9._:-]{0,127}$'));

CREATE INDEX management_audit_correlation
    ON management_audit (organization_id, correlation_id, occurred_at DESC, audit_id)
    WHERE correlation_id IS NOT NULL;
