-- Migration: 003_routing_v2
-- Description: Replace per-source routing with topic-based routing
-- Created: 2026-02-02

-- 1. Create cursor table for restart safety
CREATE TABLE IF NOT EXISTS publisher_cursor (
    id          INTEGER PRIMARY KEY DEFAULT 1,
    last_sort   JSONB NOT NULL DEFAULT '[]',
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

-- 2. Drop legacy tables (order matters due to foreign keys)
DROP TABLE IF EXISTS routes CASCADE;
DROP TABLE IF EXISTS sources CASCADE;

-- 3. Drop existing channels table and recreate with new schema
DROP TABLE IF EXISTS channels CASCADE;

CREATE TABLE channels (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(255) NOT NULL UNIQUE,
    redis_channel   VARCHAR(255) NOT NULL UNIQUE,
    description     TEXT,
    rules           JSONB NOT NULL DEFAULT '{}',
    rules_version   INTEGER NOT NULL DEFAULT 1,
    enabled         BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

-- 4. Create indexes
CREATE INDEX idx_channels_enabled ON channels(enabled) WHERE enabled = true;
CREATE INDEX idx_channels_slug ON channels(slug);

-- 5. Create trigger for updated_at
CREATE TRIGGER update_channels_updated_at
    BEFORE UPDATE ON channels
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 6. Seed initial channel for StreetCode
INSERT INTO channels (name, slug, redis_channel, description, rules) VALUES (
    'StreetCode Crime Feed',
    'streetcode_crime_feed',
    'streetcode:crime_feed',
    'Aggregated crime content for StreetCode',
    '{
        "include_topics": ["violent_crime", "property_crime", "drug_crime", "organized_crime", "criminal_justice"],
        "exclude_topics": [],
        "min_quality_score": 50,
        "content_types": ["article"]
    }'::jsonb
);
