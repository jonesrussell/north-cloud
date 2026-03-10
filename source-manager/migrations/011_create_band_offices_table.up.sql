CREATE TABLE band_offices (
    id              VARCHAR(36)  PRIMARY KEY,
    community_id    VARCHAR(36)  UNIQUE NOT NULL REFERENCES communities(id) ON DELETE CASCADE,

    -- Address
    address_line1   VARCHAR(255),
    address_line2   VARCHAR(255),
    city            VARCHAR(100),
    province        VARCHAR(5),
    postal_code     VARCHAR(10),

    -- Contact
    phone           VARCHAR(50),
    fax             VARCHAR(50),
    email           TEXT,
    toll_free       VARCHAR(50),

    -- Hours
    office_hours    TEXT,

    -- Provenance
    data_source     VARCHAR(50)  NOT NULL DEFAULT 'manual',
    source_url      TEXT,
    verified        BOOLEAN      NOT NULL DEFAULT false,
    verified_at     TIMESTAMP,

    -- Lifecycle
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER set_band_offices_updated_at
    BEFORE UPDATE ON band_offices
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
