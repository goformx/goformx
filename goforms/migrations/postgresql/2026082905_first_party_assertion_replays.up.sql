CREATE TABLE first_party_assertion_replays (
    issuer TEXT NOT NULL,
    assertion_id UUID NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    subject_id UUID NOT NULL,
    organization_id UUID NOT NULL,
    key_id TEXT NOT NULL,
    PRIMARY KEY (issuer, assertion_id)
);

CREATE INDEX idx_first_party_assertion_replays_expires_at
    ON first_party_assertion_replays (expires_at);
