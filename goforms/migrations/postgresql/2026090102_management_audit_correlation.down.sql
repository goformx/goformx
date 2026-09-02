LOCK TABLE management_audit IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM management_audit WHERE correlation_id IS NOT NULL) THEN
        RAISE EXCEPTION 'cannot remove retained management audit correlation data';
    END IF;
END $$;
DROP INDEX management_audit_correlation;
ALTER TABLE management_audit
    DROP CONSTRAINT management_audit_correlation_id_check,
    DROP COLUMN correlation_id;
