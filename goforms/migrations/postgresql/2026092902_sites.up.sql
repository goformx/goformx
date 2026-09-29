CREATE TABLE sites (
    uuid UUID PRIMARY KEY,
    organization_id VARCHAR(36) NOT NULL,
    name VARCHAR(100) NOT NULL,
    origin VARCHAR(2048) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT sites_organization_uuid_unique UNIQUE (organization_id, uuid),
    CONSTRAINT sites_organization_origin_unique UNIQUE (organization_id, origin)
);

CREATE INDEX sites_organization_created_idx ON sites (organization_id, created_at DESC, uuid DESC);
ALTER TABLE forms ADD COLUMN site_id UUID NULL;
ALTER TABLE forms ADD CONSTRAINT forms_site_same_organization_fk
    FOREIGN KEY (organization_id, site_id) REFERENCES sites (organization_id, uuid);
CREATE INDEX forms_organization_site_idx ON forms (organization_id, site_id) WHERE deleted_at IS NULL;
