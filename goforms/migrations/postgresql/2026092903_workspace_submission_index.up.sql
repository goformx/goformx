-- Supports a bounded newest-first cursor across organization-owned forms.
-- No submission ownership is copied or denormalized; the read still joins forms.
CREATE INDEX form_submissions_workspace_time_id_idx
    ON form_submissions (submitted_at DESC, uuid DESC);
