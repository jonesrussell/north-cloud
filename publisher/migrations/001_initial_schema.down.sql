-- Drop triggers
DROP TRIGGER IF EXISTS update_routes_updated_at ON routes;
DROP TRIGGER IF EXISTS update_channels_updated_at ON channels;
DROP TRIGGER IF EXISTS update_sources_updated_at ON sources;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_publish_history_article_channel;
DROP INDEX IF EXISTS idx_publish_history_published;
DROP INDEX IF EXISTS idx_publish_history_channel;
DROP INDEX IF EXISTS idx_publish_history_route;
DROP INDEX IF EXISTS idx_publish_history_article;
DROP INDEX IF EXISTS idx_routes_enabled;
DROP INDEX IF EXISTS idx_routes_channel;
DROP INDEX IF EXISTS idx_routes_source;
DROP INDEX IF EXISTS idx_channels_name;
DROP INDEX IF EXISTS idx_channels_enabled;
DROP INDEX IF EXISTS idx_sources_name;
DROP INDEX IF EXISTS idx_sources_enabled;

-- Drop tables
DROP TABLE IF EXISTS publish_history;
DROP TABLE IF EXISTS routes;
DROP TABLE IF EXISTS channels;
DROP TABLE IF EXISTS sources;

