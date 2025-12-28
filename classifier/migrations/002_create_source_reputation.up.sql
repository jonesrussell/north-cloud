-- Migration: Create source_reputation table
-- Description: Stores source trustworthiness and quality metrics
-- Version: 002
-- Date: 2025-12-22

-- Create source_reputation table
CREATE TABLE IF NOT EXISTS source_reputation (
    id SERIAL PRIMARY KEY,
    source_name VARCHAR(255) NOT NULL UNIQUE,
    source_url VARCHAR(500),
    category VARCHAR(50) DEFAULT 'unknown' CHECK (category IN ('news', 'blog', 'government', 'unknown')),
    reputation_score INT DEFAULT 50 CHECK (reputation_score >= 0 AND reputation_score <= 100),
    total_articles INT DEFAULT 0,
    average_quality_score FLOAT DEFAULT 0.0,
    spam_count INT DEFAULT 0,
    last_classified_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes
CREATE INDEX idx_source_name ON source_reputation(source_name);
CREATE INDEX idx_source_category ON source_reputation(category);
CREATE INDEX idx_source_reputation ON source_reputation(reputation_score DESC);
CREATE INDEX idx_source_last_classified ON source_reputation(last_classified_at DESC);

-- Create trigger to update updated_at
CREATE TRIGGER update_source_reputation_updated_at BEFORE UPDATE ON source_reputation
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Comments
COMMENT ON TABLE source_reputation IS 'Tracks source trustworthiness and quality metrics';
COMMENT ON COLUMN source_reputation.source_name IS 'Unique source name (e.g., example.com)';
COMMENT ON COLUMN source_reputation.category IS 'Source category: news, blog, government, or unknown';
COMMENT ON COLUMN source_reputation.reputation_score IS 'Overall reputation score (0-100)';
COMMENT ON COLUMN source_reputation.total_articles IS 'Total number of articles classified from this source';
COMMENT ON COLUMN source_reputation.average_quality_score IS 'Average quality score of articles from this source';
COMMENT ON COLUMN source_reputation.spam_count IS 'Number of spam/low-quality articles detected';
COMMENT ON COLUMN source_reputation.last_classified_at IS 'Timestamp of most recent classification';

