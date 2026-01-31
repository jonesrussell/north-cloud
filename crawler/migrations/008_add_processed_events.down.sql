-- crawler/migrations/008_add_processed_events.down.sql
-- Migration: Remove processed_events table
-- Description: Rollback for 008_add_processed_events.up.sql

BEGIN;

DROP INDEX IF EXISTS idx_processed_events_cleanup;
DROP TABLE IF EXISTS processed_events;

COMMIT;
