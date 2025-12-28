-- Drop indexes
DROP INDEX IF EXISTS idx_ml_models_active_per_type;
DROP INDEX IF EXISTS idx_models_accuracy;
DROP INDEX IF EXISTS idx_models_enabled;
DROP INDEX IF EXISTS idx_models_active;
DROP INDEX IF EXISTS idx_models_type;
DROP INDEX IF EXISTS idx_models_name_version;

-- Drop trigger
DROP TRIGGER IF EXISTS update_ml_models_updated_at ON ml_models;

-- Drop table
DROP TABLE IF EXISTS ml_models;

