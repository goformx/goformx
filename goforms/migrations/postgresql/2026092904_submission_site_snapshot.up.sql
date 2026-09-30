-- Null is intentionally retained for pre-migration history: historical site
-- identity cannot be reconstructed reliably from a form's current association.
ALTER TABLE form_submissions ADD COLUMN site_id_at_acceptance UUID NULL;
ALTER TABLE form_submissions ADD CONSTRAINT form_submissions_site_at_acceptance_fk
    FOREIGN KEY (site_id_at_acceptance) REFERENCES sites (uuid) ON DELETE RESTRICT;
CREATE INDEX form_submissions_site_time_id_idx
    ON form_submissions (site_id_at_acceptance, submitted_at DESC, uuid DESC)
    WHERE site_id_at_acceptance IS NOT NULL;
