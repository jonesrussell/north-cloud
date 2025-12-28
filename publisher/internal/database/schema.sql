-- Publisher Database Schema
-- This schema supports the database-backed routing platform with Redis pub/sub

-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Source Indexes (Elasticsearch indexes to monitor)
CREATE TABLE sources (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,              -- e.g., "sudbury_com"
    index_pattern VARCHAR(255) NOT NULL,            -- e.g., "sudbury_com_classified_content"
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Redis Channels (Topic-based channels for pub/sub)
CREATE TABLE channels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,              -- e.g., "articles:crime", "articles:news"
    description TEXT,                               -- Human-readable description
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Routes (Many-to-Many: Sources → Channels)
CREATE TABLE routes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_id UUID NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    channel_id UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,

    -- Filtering criteria (publisher responsibility)
    min_quality_score INT DEFAULT 50,               -- Min quality threshold (0-100)
    topics TEXT[],                                  -- e.g., ARRAY['crime', 'news']

    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Ensure unique source-channel pairing
    UNIQUE(source_id, channel_id)
);

-- Publishing History (Audit trail)
CREATE TABLE publish_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    route_id UUID REFERENCES routes(id) ON DELETE SET NULL,
    article_id VARCHAR(255) NOT NULL,               -- Elasticsearch document ID
    article_title TEXT,
    article_url TEXT,
    channel_name VARCHAR(255) NOT NULL,             -- Redis channel published to

    published_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Metadata for debugging
    quality_score INT,
    topics TEXT[]
);

-- Indexes for performance
CREATE INDEX idx_sources_enabled ON sources(enabled);
CREATE INDEX idx_sources_name ON sources(name);

CREATE INDEX idx_channels_enabled ON channels(enabled);
CREATE INDEX idx_channels_name ON channels(name);

CREATE INDEX idx_routes_source ON routes(source_id);
CREATE INDEX idx_routes_channel ON routes(channel_id);
CREATE INDEX idx_routes_enabled ON routes(enabled);

CREATE INDEX idx_publish_history_article ON publish_history(article_id);
CREATE INDEX idx_publish_history_route ON publish_history(route_id);
CREATE INDEX idx_publish_history_channel ON publish_history(channel_name);
CREATE INDEX idx_publish_history_published ON publish_history(published_at DESC);
CREATE INDEX idx_publish_history_article_channel ON publish_history(article_id, channel_name);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers to auto-update updated_at
CREATE TRIGGER update_sources_updated_at BEFORE UPDATE ON sources
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_channels_updated_at BEFORE UPDATE ON channels
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_routes_updated_at BEFORE UPDATE ON routes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Comments for documentation
COMMENT ON TABLE sources IS 'Elasticsearch indexes to monitor for articles';
COMMENT ON TABLE channels IS 'Redis pub/sub channels for routing articles by topic';
COMMENT ON TABLE routes IS 'Many-to-many routing rules: source → channel with filters';
COMMENT ON TABLE publish_history IS 'Audit trail of all articles published to Redis channels';

COMMENT ON COLUMN routes.min_quality_score IS 'Minimum quality score (0-100) for articles to be published';
COMMENT ON COLUMN routes.topics IS 'Array of topics to filter by (e.g., crime, news). NULL means no topic filtering.';
COMMENT ON COLUMN publish_history.article_id IS 'Elasticsearch document ID for deduplication';
COMMENT ON COLUMN publish_history.channel_name IS 'Denormalized channel name for faster querying';
