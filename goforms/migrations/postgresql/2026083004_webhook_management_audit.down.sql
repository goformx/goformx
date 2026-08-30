-- Never discard webhook history or permit an older writer to resume mutations.
LOCK TABLE management_audit IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM management_audit WHERE event LIKE 'webhook.%') THEN
        RAISE EXCEPTION 'cannot remove webhook audit schema with retained webhook history';
    END IF;
END $$;
DROP INDEX management_audit_form;
ALTER TABLE management_audit
    DROP CONSTRAINT management_audit_event_check,
    DROP CONSTRAINT management_audit_payload_check,
    DROP COLUMN form_id,
    DROP COLUMN enabled,
    ADD CONSTRAINT management_audit_event_check CHECK (event IN ('service_token.created', 'service_token.revoked', 'service_token.rotated')),
    ADD CONSTRAINT management_audit_check1 CHECK (
        (event = 'service_token.revoked' AND scopes = '[]'::jsonb AND expires_at IS NULL)
        OR (event <> 'service_token.revoked' AND jsonb_array_length(scopes) BETWEEN 1 AND 8 AND expires_at IS NOT NULL));
