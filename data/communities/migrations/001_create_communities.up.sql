-- Enable PostGIS extension (idempotent)
CREATE EXTENSION IF NOT EXISTS postgis;

-- Create communities table
CREATE TABLE IF NOT EXISTS communities (
    id VARCHAR(128) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    community_type VARCHAR(50) NOT NULL,
    province VARCHAR(2) NOT NULL DEFAULT 'ON',
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    geom GEOGRAPHY(POINT, 4326),
    population INTEGER,
    governing_body VARCHAR(255),
    external_ids JSONB NOT NULL DEFAULT '{}',
    region VARCHAR(255),
    subregion VARCHAR(255),
    neighbours JSONB NOT NULL DEFAULT '[]',
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Auto-populate geom from lat/lon on insert and update
CREATE OR REPLACE FUNCTION communities_update_geom()
RETURNS TRIGGER AS $$
BEGIN
    NEW.geom := ST_SetSRID(ST_MakePoint(NEW.longitude, NEW.latitude), 4326)::geography;
    NEW.updated_at := CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER communities_geom_trigger
    BEFORE INSERT OR UPDATE ON communities
    FOR EACH ROW
    EXECUTE FUNCTION communities_update_geom();

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_communities_type ON communities(community_type);
CREATE INDEX IF NOT EXISTS idx_communities_province ON communities(province);
CREATE INDEX IF NOT EXISTS idx_communities_region ON communities(region);
CREATE INDEX IF NOT EXISTS idx_communities_name ON communities(name);
CREATE INDEX IF NOT EXISTS idx_communities_geom ON communities USING GIST(geom);
CREATE INDEX IF NOT EXISTS idx_communities_external_ids ON communities USING GIN(external_ids);

-- Partial index for First Nations (common filter)
CREATE INDEX IF NOT EXISTS idx_communities_first_nations
    ON communities(id) WHERE community_type = 'first_nation';
