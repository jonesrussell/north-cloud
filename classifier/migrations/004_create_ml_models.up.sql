-- Migration: Create ml_models table
-- Description: Stores metadata about machine learning models
-- Version: 004
-- Date: 2025-12-22

-- Create ml_models table
CREATE TABLE IF NOT EXISTS ml_models (
    id SERIAL PRIMARY KEY,
    model_name VARCHAR(255) NOT NULL,
    model_version VARCHAR(50) NOT NULL,
    model_type VARCHAR(50) CHECK (model_type IN ('content_type', 'topic', 'quality')),

    -- Performance metrics
    accuracy FLOAT CHECK (accuracy >= 0.0 AND accuracy <= 1.0),
    f1_score FLOAT CHECK (f1_score >= 0.0 AND f1_score <= 1.0),
    precision_score FLOAT CHECK (precision_score >= 0.0 AND precision_score <= 1.0),
    recall_score FLOAT CHECK (recall_score >= 0.0 AND recall_score <= 1.0),

    -- Metadata
    trained_at TIMESTAMP,
    feature_set TEXT[], -- Array of feature names used
    hyperparameters JSONB, -- Store model configuration as JSON
    model_path VARCHAR(500), -- Path to serialized model file (local or S3)

    -- Status
    is_active BOOLEAN DEFAULT FALSE,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),

    UNIQUE(model_name, model_version)
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_models_name_version ON ml_models(model_name, model_version);
CREATE INDEX IF NOT EXISTS idx_models_type ON ml_models(model_type);
CREATE INDEX IF NOT EXISTS idx_models_active ON ml_models(is_active);
CREATE INDEX IF NOT EXISTS idx_models_enabled ON ml_models(enabled);
CREATE INDEX IF NOT EXISTS idx_models_accuracy ON ml_models(accuracy DESC);

-- Create trigger to update updated_at
CREATE TRIGGER update_ml_models_updated_at BEFORE UPDATE ON ml_models
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Create constraint to ensure only one active model per type
-- Note: Partial unique indexes don't support IF NOT EXISTS, but this is unlikely to conflict
CREATE UNIQUE INDEX IF NOT EXISTS idx_ml_models_active_per_type ON ml_models(model_type)
    WHERE is_active = TRUE;

-- Comments
COMMENT ON TABLE ml_models IS 'Metadata and performance metrics for machine learning models';
COMMENT ON COLUMN ml_models.model_type IS 'Type of model: content_type, topic, or quality';
COMMENT ON COLUMN ml_models.feature_set IS 'Array of feature names used to train the model';
COMMENT ON COLUMN ml_models.hyperparameters IS 'Model configuration stored as JSON';
COMMENT ON COLUMN ml_models.model_path IS 'Path to serialized model file (filesystem or S3)';
COMMENT ON COLUMN ml_models.is_active IS 'Whether this model is currently active for predictions';
COMMENT ON COLUMN ml_models.enabled IS 'Whether this model is enabled (can be activated)';
COMMENT ON INDEX idx_ml_models_active_per_type IS 'Ensures only one active model per type';

