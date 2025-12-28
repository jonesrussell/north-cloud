-- Drop trigger
DROP TRIGGER IF EXISTS update_source_reputation_updated_at ON source_reputation;

-- Drop indexes
DROP INDEX IF EXISTS idx_source_last_classified;
DROP INDEX IF EXISTS idx_source_reputation;
DROP INDEX IF EXISTS idx_source_category;
DROP INDEX IF EXISTS idx_source_name;

-- Drop table
DROP TABLE IF EXISTS source_reputation;

