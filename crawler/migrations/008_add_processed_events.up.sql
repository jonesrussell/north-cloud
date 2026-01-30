-- crawler/migrations/008_add_processed_events.up.sql
-- Migration: Add processed_events table for event idempotency
-- Description: Tracks which events have been processed to prevent duplicate handling

BEGIN;

CREATE TABLE IF NOT EXISTS processed_events (
    event_id UUID PRIMARY KEY,
    processed_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for cleanup of old events
CREATE INDEX IF NOT EXISTS idx_processed_events_cleanup
    ON processed_events (processed_at);

COMMENT ON TABLE processed_events IS
    'Tracks processed source events for idempotency (at-least-once delivery)';

COMMENT ON INDEX idx_processed_events_cleanup IS
    'Index for efficient time-based cleanup queries to remove old processed events';

COMMIT;
