-- Migration: Create classification_rules table
-- Description: Stores rules for content type and topic classification
-- Version: 001
-- Date: 2025-12-22

-- Create classification_rules table
CREATE TABLE IF NOT EXISTS classification_rules (
    id SERIAL PRIMARY KEY,
    rule_name VARCHAR(255) NOT NULL UNIQUE,
    rule_type VARCHAR(50) NOT NULL CHECK (rule_type IN ('content_type', 'topic', 'quality')),
    topic_name VARCHAR(100), -- Only for topic rules
    keywords TEXT[] NOT NULL DEFAULT '{}', -- Array of keywords for matching
    min_confidence FLOAT DEFAULT 0.5 CHECK (min_confidence >= 0.0 AND min_confidence <= 1.0),
    enabled BOOLEAN DEFAULT TRUE,
    priority INT DEFAULT 0, -- Higher priority rules evaluated first
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes
CREATE INDEX idx_rules_type ON classification_rules(rule_type);
CREATE INDEX idx_rules_topic ON classification_rules(topic_name);
CREATE INDEX idx_rules_enabled ON classification_rules(enabled);
CREATE INDEX idx_rules_priority ON classification_rules(priority DESC);

-- Create trigger to update updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_rules_updated_at BEFORE UPDATE ON classification_rules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Insert default rules
INSERT INTO classification_rules (rule_name, rule_type, topic_name, keywords, min_confidence, priority) VALUES
    ('crime_detection', 'topic', 'crime', ARRAY[
        'police', 'arrest', 'arrested', 'charged', 'court', 'murder', 'assault',
        'robbery', 'theft', 'suspect', 'investigation', 'detective', 'crime',
        'criminal', 'victim', 'shooting', 'stabbing', 'homicide', 'burglary',
        'stolen', 'warrant', 'jail', 'prison', 'conviction', 'sentence', 'trial'
    ], 0.3, 10),
    ('sports_detection', 'topic', 'sports', ARRAY[
        'game', 'team', 'player', 'score', 'tournament', 'championship',
        'season', 'coach', 'win', 'loss', 'match', 'league', 'playoff'
    ], 0.4, 5),
    ('politics_detection', 'topic', 'politics', ARRAY[
        'election', 'government', 'minister', 'policy', 'vote', 'parliament',
        'senator', 'congressman', 'mayor', 'council', 'legislation', 'bill'
    ], 0.4, 5),
    ('local_news_detection', 'topic', 'local_news', ARRAY[
        'community', 'local', 'neighborhood', 'city', 'town', 'resident',
        'downtown', 'area', 'region', 'municipal'
    ], 0.3, 3);

-- Comments
COMMENT ON TABLE classification_rules IS 'Rules for classifying content by type and topic';
COMMENT ON COLUMN classification_rules.rule_type IS 'Type of rule: content_type, topic, or quality';
COMMENT ON COLUMN classification_rules.topic_name IS 'Topic name for topic rules (e.g., crime, sports)';
COMMENT ON COLUMN classification_rules.keywords IS 'Array of keywords to match against content';
COMMENT ON COLUMN classification_rules.min_confidence IS 'Minimum confidence score (0.0-1.0) required for classification';
COMMENT ON COLUMN classification_rules.priority IS 'Higher priority rules are evaluated first';
