-- Rollback: 003_routing_v2
-- Reverts to the original schema from 001_initial_schema

-- Drop new tables
DROP TABLE IF EXISTS publisher_cursor CASCADE;
DROP TABLE IF EXISTS channels CASCADE;

-- Recreate original schema (from 001_initial_schema.up.sql)
CREATE TABLE sources (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    index_pattern VARCHAR(255) NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE channels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE routes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_id UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    channel_id UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    min_quality_score INT DEFAULT 50,
    topics TEXT[],
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(source_id, channel_id)
);

-- Recreate indexes
CREATE INDEX idx_sources_enabled ON sources(enabled);
CREATE INDEX idx_sources_name ON sources(name);
CREATE INDEX idx_channels_enabled ON channels(enabled);
CREATE INDEX idx_channels_name ON channels(name);
CREATE INDEX idx_routes_source ON routes(source_id);
CREATE INDEX idx_routes_channel ON routes(channel_id);
CREATE INDEX idx_routes_enabled ON routes(enabled);

-- Recreate triggers
CREATE TRIGGER update_sources_updated_at BEFORE UPDATE ON sources
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_channels_updated_at BEFORE UPDATE ON channels
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER update_routes_updated_at BEFORE UPDATE ON routes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
