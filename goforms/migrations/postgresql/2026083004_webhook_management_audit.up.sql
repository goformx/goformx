ALTER TABLE management_audit
    ADD COLUMN form_id UUID,
    ADD COLUMN enabled BOOLEAN,
    DROP CONSTRAINT management_audit_event_check,
    DROP CONSTRAINT management_audit_check1,
    ADD CONSTRAINT management_audit_event_check CHECK (event IN (
        'service_token.created', 'service_token.revoked', 'service_token.rotated',
        'webhook.created', 'webhook.updated', 'webhook.paused', 'webhook.resumed',
        'webhook.signing_secret_rotated', 'webhook.deleted', 'webhook.delivery_replayed')),
    ADD CONSTRAINT management_audit_payload_check CHECK (
        (event IN ('service_token.created', 'service_token.rotated') AND
            jsonb_array_length(scopes) BETWEEN 1 AND 8 AND expires_at IS NOT NULL AND form_id IS NULL AND enabled IS NULL)
        OR (event = 'service_token.revoked' AND scopes = '[]'::jsonb AND expires_at IS NULL AND form_id IS NULL AND enabled IS NULL)
        OR (event IN ('webhook.created', 'webhook.updated', 'webhook.paused', 'webhook.resumed',
                     'webhook.signing_secret_rotated', 'webhook.deleted', 'webhook.delivery_replayed') AND
            scopes = '[]'::jsonb AND expires_at IS NULL AND form_id IS NOT NULL AND
            target_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' AND
            ((event IN ('webhook.deleted', 'webhook.delivery_replayed') AND enabled IS NULL) OR
             (event IN ('webhook.created', 'webhook.updated', 'webhook.signing_secret_rotated') AND enabled IS NOT NULL) OR
             (event = 'webhook.paused' AND enabled IS FALSE) OR
             (event = 'webhook.resumed' AND enabled IS TRUE))));
CREATE INDEX management_audit_form ON management_audit (organization_id, form_id, occurred_at DESC, audit_id)
    WHERE form_id IS NOT NULL;
