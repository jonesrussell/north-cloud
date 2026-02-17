DROP INDEX IF EXISTS idx_sources_ingestion_mode;
ALTER TABLE sources DROP COLUMN IF EXISTS feed_poll_interval_minutes;
ALTER TABLE sources DROP COLUMN IF EXISTS ingestion_mode;
ALTER TABLE sources DROP COLUMN IF EXISTS sitemap_url;
ALTER TABLE sources DROP COLUMN IF EXISTS feed_url;
