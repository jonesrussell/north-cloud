-- crawler/migrations/009_add_auto_managed_jobs.up.sql
-- Migration: Add columns for auto-managed job lifecycle
-- Description: Extends jobs table with priority, backoff, and auto-management tracking

BEGIN;

-- Add columns for auto-managed job lifecycle
ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS auto_managed BOOLEAN DEFAULT false,
    ADD COLUMN IF NOT EXISTS priority INTEGER DEFAULT 50,
    ADD COLUMN IF NOT EXISTS failure_count INTEGER DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_failure_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS backoff_until TIMESTAMPTZ;

-- Index for efficient due job queries with priority ordering
CREATE INDEX IF NOT EXISTS idx_jobs_due_priority
    ON jobs (next_run_at, priority DESC)
    WHERE status = 'pending' AND (backoff_until IS NULL OR backoff_until < NOW());

-- Index for finding jobs by source_id (used by JobService)
CREATE INDEX IF NOT EXISTS idx_jobs_source_id
    ON jobs (source_id);

-- Documentation
COMMENT ON COLUMN jobs.auto_managed IS 'True if job is managed by event-driven automation';
COMMENT ON COLUMN jobs.priority IS 'Numeric priority 0-100, higher = scheduled sooner';
COMMENT ON COLUMN jobs.failure_count IS 'Consecutive failure count for backoff calculation';
COMMENT ON COLUMN jobs.backoff_until IS 'Do not run until this time (failure backoff)';

COMMIT;
