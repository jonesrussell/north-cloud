-- Rollback: Drop pipeline event-sourcing schema
-- WARNING: This will permanently delete all pipeline events and article data

-- Drop indexes on pipeline_events
DROP INDEX IF EXISTS idx_events_occurred;
DROP INDEX IF EXISTS idx_events_article;
DROP INDEX IF EXISTS idx_events_stage_time;

-- Drop partitions
DROP TABLE IF EXISTS pipeline_events_2026_04;
DROP TABLE IF EXISTS pipeline_events_2026_03;
DROP TABLE IF EXISTS pipeline_events_2026_02;
DROP TABLE IF EXISTS pipeline_events_2026_01;

-- Drop partitioned table
DROP TABLE IF EXISTS pipeline_events;

-- Drop stage_ordering table
DROP TABLE IF EXISTS stage_ordering;

-- Drop enum type
DROP TYPE IF EXISTS pipeline_stage;

-- Drop indexes on articles
DROP INDEX IF EXISTS idx_articles_domain;
DROP INDEX IF EXISTS idx_articles_hash;
DROP INDEX IF EXISTS idx_articles_source;

-- Drop articles table
DROP TABLE IF EXISTS articles;
