-- Drop trigger
DROP TRIGGER IF EXISTS update_sources_updated_at ON sources;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_sources_article_index;
DROP INDEX IF EXISTS idx_sources_enabled;
DROP INDEX IF EXISTS idx_sources_city_name;
DROP INDEX IF EXISTS idx_sources_name;

-- Drop table
DROP TABLE IF EXISTS sources;

