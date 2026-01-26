-- Migration: Rollback V1 Cleanup
-- Description: Restores V1 scheduler columns if needed for rollback
-- Author: Claude Code
-- Date: 2026-01-26
--
-- WARNING: This rollback re-adds V1 columns but does NOT restore data.
-- If you need to rollback to V1 scheduler, you may need to restore from backup.

BEGIN;

-- ============================================================================
-- STEP 1: Restore locking columns
-- ============================================================================

ALTER TABLE jobs ADD COLUMN IF NOT EXISTS lock_token VARCHAR(36);
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS lock_acquired_at TIMESTAMP WITH TIME ZONE;

-- ============================================================================
-- STEP 2: Restore legacy cron column
-- ============================================================================

ALTER TABLE jobs ADD COLUMN IF NOT EXISTS schedule_time VARCHAR(100);

-- ============================================================================
-- STEP 3: Restore scheduler_version column
-- ============================================================================

ALTER TABLE jobs ADD COLUMN IF NOT EXISTS scheduler_version INTEGER DEFAULT 1;

-- Add constraint back
ALTER TABLE jobs ADD CONSTRAINT valid_scheduler_version CHECK (
    scheduler_version IN (1, 2)
);

-- Set all existing jobs to V2 (since they were created under V2)
UPDATE jobs SET scheduler_version = 2 WHERE scheduler_version IS NULL;

-- ============================================================================
-- STEP 4: Restore V1-specific indexes
-- ============================================================================

CREATE INDEX IF NOT EXISTS idx_jobs_status_next_run
    ON jobs(status, next_run_at)
    WHERE is_paused = false AND lock_token IS NULL;

CREATE INDEX IF NOT EXISTS idx_jobs_lock_status
    ON jobs(lock_token, status);

-- ============================================================================
-- STEP 5: Restore schedule_type default
-- ============================================================================

ALTER TABLE jobs ALTER COLUMN schedule_type SET DEFAULT 'interval';

-- ============================================================================
-- STEP 6: Restore scheduler consumer offsets table
-- ============================================================================

CREATE TABLE IF NOT EXISTS scheduler_consumer_offsets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    consumer_group VARCHAR(100) NOT NULL,
    stream_name VARCHAR(100) NOT NULL,
    last_id VARCHAR(50) NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT unique_consumer_stream UNIQUE (consumer_group, stream_name)
);

-- ============================================================================
-- STEP 7: Update comment
-- ============================================================================

COMMENT ON TABLE jobs IS 'Crawler jobs (supports both V1 and V2 schedulers)';

COMMIT;
