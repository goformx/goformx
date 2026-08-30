-- Application rollback must preserve the durable management history.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM management_audit) THEN
        RAISE EXCEPTION 'cannot roll back a populated management audit';
    END IF;
END;
$$;
DROP TABLE management_audit;
DROP FUNCTION prevent_management_audit_mutation();
