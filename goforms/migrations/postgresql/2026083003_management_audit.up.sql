CREATE TABLE management_audit (
    audit_id UUID PRIMARY KEY,
    organization_id VARCHAR(36) NOT NULL,
    subject_id VARCHAR(128) NOT NULL,
    credential_class TEXT NOT NULL CHECK (credential_class IN ('service_token', 'first_party_assertion', 'database_operator')),
    credential_id VARCHAR(128) NOT NULL,
    request_id VARCHAR(128) NOT NULL,
    event TEXT NOT NULL CHECK (event IN ('service_token.created', 'service_token.revoked', 'service_token.rotated')),
    target_id VARCHAR(128) NOT NULL,
    related_id VARCHAR(128),
    scopes JSONB NOT NULL CHECK (jsonb_typeof(scopes) = 'array'),
    expires_at TIMESTAMPTZ,
    occurred_at TIMESTAMPTZ NOT NULL,
    CHECK (subject_id ~ '^[A-Za-z0-9_-][A-Za-z0-9._:-]{0,127}$'),
    CHECK (credential_id ~ '^[A-Za-z0-9_-][A-Za-z0-9._:-]{0,127}$'),
    CHECK (request_id ~ '^[A-Za-z0-9_-][A-Za-z0-9._:-]{0,127}$'),
    CHECK (target_id ~ '^[A-Za-z0-9_-][A-Za-z0-9._:-]{0,127}$'),
    CHECK (related_id IS NULL OR related_id ~ '^[A-Za-z0-9_-][A-Za-z0-9._:-]{0,127}$'),
    CHECK ((event = 'service_token.rotated' AND related_id IS NOT NULL AND related_id <> target_id)
        OR (event <> 'service_token.rotated' AND related_id IS NULL)),
    CHECK ((event = 'service_token.revoked' AND scopes = '[]'::jsonb AND expires_at IS NULL)
        OR (event <> 'service_token.revoked' AND jsonb_array_length(scopes) BETWEEN 1 AND 8 AND expires_at IS NOT NULL))
);
CREATE INDEX management_audit_organization_time ON management_audit (organization_id, occurred_at DESC, audit_id);
CREATE INDEX management_audit_target ON management_audit (organization_id, target_id, occurred_at DESC);

-- Audit ownership and targets deliberately have no cascading foreign keys.
CREATE FUNCTION prevent_management_audit_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'management audit records are append-only';
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER management_audit_append_only BEFORE UPDATE OR DELETE ON management_audit
    FOR EACH ROW EXECUTE FUNCTION prevent_management_audit_mutation();
CREATE TRIGGER management_audit_no_truncate BEFORE TRUNCATE ON management_audit
    FOR EACH STATEMENT EXECUTE FUNCTION prevent_management_audit_mutation();
