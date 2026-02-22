ALTER TABLE sources ADD COLUMN feed_disabled_at    TIMESTAMP WITH TIME ZONE;
ALTER TABLE sources ADD COLUMN feed_disable_reason VARCHAR(20);

CREATE INDEX idx_sources_feed_disabled_at ON sources(feed_disabled_at)
    WHERE feed_disabled_at IS NOT NULL;
