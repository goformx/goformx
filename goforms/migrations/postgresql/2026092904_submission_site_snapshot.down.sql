BEGIN;
LOCK TABLE form_submissions IN ACCESS EXCLUSIVE MODE;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM form_submissions WHERE site_id_at_acceptance IS NOT NULL) THEN
        RAISE EXCEPTION 'submission site snapshot rollback refused: accepted attribution exists';
    END IF;
END $$;
DROP INDEX form_submissions_site_time_id_idx;
ALTER TABLE form_submissions DROP CONSTRAINT form_submissions_site_at_acceptance_fk;
ALTER TABLE form_submissions DROP COLUMN site_id_at_acceptance;
COMMIT;
