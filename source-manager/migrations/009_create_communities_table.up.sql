CREATE TABLE communities (
    id              VARCHAR(36)  PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(255) NOT NULL,

    -- Classification
    community_type  VARCHAR(50)  NOT NULL,
    province        VARCHAR(2),
    region          VARCHAR(100),

    -- Authoritative identifiers
    inac_id         VARCHAR(20),
    statcan_csd     VARCHAR(20),

    -- Geodata
    latitude        DOUBLE PRECISION,
    longitude       DOUBLE PRECISION,

    -- Metadata
    nation          VARCHAR(255),
    treaty          VARCHAR(255),
    language_group  VARCHAR(255),
    reserve_name    VARCHAR(255),
    population      INTEGER,
    population_year INTEGER,

    -- Digital presence
    website         TEXT,
    feed_url        TEXT,

    -- Source attribution
    data_source     VARCHAR(50)  NOT NULL DEFAULT 'manual',
    source_id       VARCHAR(36)  REFERENCES sources(id),

    -- Lifecycle
    enabled         BOOLEAN      NOT NULL DEFAULT true,
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Named constraints
    CONSTRAINT uq_communities_slug UNIQUE (slug),
    CONSTRAINT uq_communities_inac_id UNIQUE (inac_id),
    CONSTRAINT uq_communities_statcan_csd UNIQUE (statcan_csd)
);

CREATE INDEX idx_communities_type ON communities(community_type);
CREATE INDEX idx_communities_province ON communities(province);
CREATE INDEX idx_communities_inac_id ON communities(inac_id) WHERE inac_id IS NOT NULL;
CREATE INDEX idx_communities_statcan_csd ON communities(statcan_csd) WHERE statcan_csd IS NOT NULL;
CREATE INDEX idx_communities_coords ON communities(latitude, longitude) WHERE latitude IS NOT NULL;

CREATE TRIGGER set_communities_updated_at
    BEFORE UPDATE ON communities
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
