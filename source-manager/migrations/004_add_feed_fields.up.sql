ALTER TABLE sources ADD COLUMN feed_url TEXT;
ALTER TABLE sources ADD COLUMN sitemap_url TEXT;
ALTER TABLE sources ADD COLUMN ingestion_mode VARCHAR(10) NOT NULL DEFAULT 'spider';
ALTER TABLE sources ADD COLUMN feed_poll_interval_minutes INTEGER NOT NULL DEFAULT 15;

CREATE INDEX idx_sources_ingestion_mode ON sources(ingestion_mode);
