-- Restore the exact pre-migration catalog name without changing retained rows.
ALTER TABLE management_audit
    RENAME CONSTRAINT management_audit_relationship_check TO management_audit_check;
