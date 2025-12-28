-- Migration: Create classification_history table
-- Description: Audit trail for classifications (used for ML training data)
-- Version: 003
-- Date: 2025-12-22

-- Create classification_history table
CREATE TABLE IF NOT EXISTS classification_history (
    id SERIAL PRIMARY KEY,
    content_id VARCHAR(255) NOT NULL,
    content_url VARCHAR(500) NOT NULL,
    source_name VARCHAR(255) NOT NULL,

    -- Classification results
    content_type VARCHAR(50),
    content_subtype VARCHAR(50),
    quality_score INT CHECK (quality_score >= 0 AND quality_score <= 100),
    topics TEXT[], -- Array of topics
    is_crime_related BOOLEAN DEFAULT FALSE,
    source_reputation_score INT CHECK (source_reputation_score >= 0 AND source_reputation_score <= 100),

    -- Metadata
    classifier_version VARCHAR(50) DEFAULT '1.0.0',
    classification_method VARCHAR(50) CHECK (classification_method IN ('rule_based', 'ml_model', 'hybrid')),
    model_version VARCHAR(50),
    confidence FLOAT CHECK (confidence >= 0.0 AND confidence <= 1.0),
    processing_time_ms INT, -- Processing time in milliseconds

    -- Timestamp
    classified_at TIMESTAMP DEFAULT NOW(),

    CONSTRAINT fk_source_name FOREIGN KEY (source_name)
        REFERENCES source_reputation(source_name) ON DELETE SET NULL
);

-- Create indexes
CREATE INDEX idx_history_content_id ON classification_history(content_id);
CREATE INDEX idx_history_source ON classification_history(source_name);
CREATE INDEX idx_history_classified_at ON classification_history(classified_at DESC);
CREATE INDEX idx_history_content_type ON classification_history(content_type);
CREATE INDEX idx_history_is_crime ON classification_history(is_crime_related);
CREATE INDEX idx_history_method ON classification_history(classification_method);
CREATE INDEX idx_history_confidence ON classification_history(confidence DESC);

-- Create index for ML training data queries (high-confidence classifications from last 6 months)
CREATE INDEX idx_history_training_data ON classification_history(classified_at DESC, confidence DESC)
    WHERE confidence > 0.7;

-- Comments
COMMENT ON TABLE classification_history IS 'Audit trail of all classifications for analysis and ML training';
COMMENT ON COLUMN classification_history.content_id IS 'Elasticsearch document ID';
COMMENT ON COLUMN classification_history.classification_method IS 'Method used: rule_based, ml_model, or hybrid';
COMMENT ON COLUMN classification_history.confidence IS 'Overall classification confidence (0.0-1.0)';
COMMENT ON COLUMN classification_history.processing_time_ms IS 'Time taken to classify in milliseconds';
COMMENT ON INDEX idx_history_training_data IS 'Optimized for extracting ML training data (high-confidence, recent)';

