CREATE TABLE submission_export_audit (
    export_id UUID PRIMARY KEY,
    organization_id VARCHAR(36) NOT NULL,
    form_id VARCHAR(36) NOT NULL,
    subject_id VARCHAR(128) NOT NULL,
    credential_class TEXT NOT NULL CHECK (credential_class IN ('service_token', 'first_party_assertion')),
    credential_id VARCHAR(128) NOT NULL,
    request_id VARCHAR(128) NOT NULL,
    event TEXT NOT NULL DEFAULT 'export.prepared' CHECK (event = 'export.prepared'),
    format TEXT NOT NULL CHECK (format IN ('json', 'csv')),
    row_count INTEGER NOT NULL CHECK (row_count BETWEEN 0 AND 1000),
    byte_count INTEGER NOT NULL CHECK (byte_count BETWEEN 1 AND 8388608),
    prepared_at TIMESTAMPTZ NOT NULL,
    CHECK (subject_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'),
    CHECK (credential_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'),
    CHECK (request_id ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$')
);
CREATE INDEX submission_export_audit_organization_time
    ON submission_export_audit (organization_id, prepared_at DESC, export_id);

CREATE FUNCTION prevent_submission_export_audit_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'submission export audit records are append-only';
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER submission_export_audit_append_only
    BEFORE UPDATE OR DELETE ON submission_export_audit
    FOR EACH ROW EXECUTE FUNCTION prevent_submission_export_audit_mutation();
CREATE TRIGGER submission_export_audit_no_truncate
    BEFORE TRUNCATE ON submission_export_audit
    FOR EACH STATEMENT EXECUTE FUNCTION prevent_submission_export_audit_mutation();
