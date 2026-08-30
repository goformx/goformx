-- Roll the application back without dropping audit history. Schema rollback is
-- permitted only before the first prepared export; operators cannot accidentally
-- erase an audit trail by running a routine down migration.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM submission_export_audit) THEN
        RAISE EXCEPTION 'cannot roll back a populated submission export audit';
    END IF;
END;
$$;
DROP TABLE submission_export_audit;
DROP FUNCTION prevent_submission_export_audit_mutation();
