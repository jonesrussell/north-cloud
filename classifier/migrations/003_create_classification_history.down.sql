-- Drop indexes
DROP INDEX IF EXISTS idx_history_training_data;
DROP INDEX IF EXISTS idx_history_confidence;
DROP INDEX IF EXISTS idx_history_method;
DROP INDEX IF EXISTS idx_history_is_crime;
DROP INDEX IF EXISTS idx_history_content_type;
DROP INDEX IF EXISTS idx_history_classified_at;
DROP INDEX IF EXISTS idx_history_source;
DROP INDEX IF EXISTS idx_history_content_id;

-- Drop table
DROP TABLE IF EXISTS classification_history;

