-- Preserve deployed migration history while replacing the final anonymous
-- multi-column audit constraint name with an operationally stable name.
ALTER TABLE management_audit
    RENAME CONSTRAINT management_audit_check TO management_audit_relationship_check;
