BEGIN;
LOCK TABLE forms, sites IN ACCESS EXCLUSIVE MODE;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM sites) THEN
        RAISE EXCEPTION 'sites rollback refused: site identities exist; preserve or migrate them before rollback';
    END IF;
END $$;
DROP INDEX forms_organization_site_idx;
ALTER TABLE forms DROP CONSTRAINT forms_site_same_organization_fk;
ALTER TABLE forms DROP COLUMN site_id;
DROP TABLE sites;
COMMIT;
