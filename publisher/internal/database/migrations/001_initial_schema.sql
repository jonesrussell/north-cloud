-- Migration: 001_initial_schema
-- Description: Create initial publisher database schema
-- Created: 2025-12-28

-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Source Indexes (Elasticsearch indexes to monitor)
CREATE TABLE sources (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    index_pattern VARCHAR(255) NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Redis Channels (Topic-based channels for pub/sub)
CREATE TABLE channels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Routes (Many-to-Many: Sources → Channels)
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

-- Publishing History (Audit trail)
CREATE TABLE publish_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    route_id UUID REFERENCES routes(id) ON DELETE SET NULL,
    article_id VARCHAR(255) NOT NULL,
    article_title TEXT,
    article_url TEXT,
    channel_name VARCHAR(255) NOT NULL,
    published_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    quality_score INT,
    topics TEXT[]
);

-- Indexes
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

-- Update timestamp function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers
CREATE TRIGGER update_sources_updated_at BEFORE UPDATE ON sources
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_channels_updated_at BEFORE UPDATE ON channels
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_routes_updated_at BEFORE UPDATE ON routes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
