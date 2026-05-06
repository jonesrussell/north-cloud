-- 0001_initial_schema.down.sql
-- Drops poll_checkpoint and alert_catalogue tables.

DROP INDEX IF EXISTS idx_catalogue_active_seen;
DROP TABLE IF EXISTS alert_catalogue;
DROP TABLE IF EXISTS poll_checkpoint;
