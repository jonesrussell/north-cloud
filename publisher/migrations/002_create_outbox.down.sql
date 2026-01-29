-- Rollback: Remove outbox table

DROP INDEX IF EXISTS idx_outbox_status;
DROP INDEX IF EXISTS idx_outbox_cleanup;
DROP INDEX IF EXISTS idx_outbox_crime;
DROP INDEX IF EXISTS idx_outbox_routing;
DROP INDEX IF EXISTS idx_outbox_retry;
DROP INDEX IF EXISTS idx_outbox_pending;
DROP TABLE IF EXISTS classified_outbox;
