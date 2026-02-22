DROP INDEX IF EXISTS idx_sources_feed_disabled_at;
ALTER TABLE sources DROP COLUMN IF EXISTS feed_disable_reason;
ALTER TABLE sources DROP COLUMN IF EXISTS feed_disabled_at;
