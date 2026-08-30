CREATE INDEX form_submissions_form_time_id_idx
    ON form_submissions (form_id, submitted_at DESC, uuid DESC);
CREATE INDEX form_submissions_form_version_time_id_idx
    ON form_submissions (form_id, schema_version, submitted_at DESC, uuid DESC);
