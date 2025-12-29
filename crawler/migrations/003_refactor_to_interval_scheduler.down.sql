-- Migration Rollback: Refactor to Interval-Based Scheduler
-- Description: Reverts interval-based scheduling changes and restores cron-based system
-- Author: Claude Code
-- Date: 2025-12-29

BEGIN;

-- ============================================================================
-- STEP 1: Drop triggers
-- ============================================================================
DROP TRIGGER IF EXISTS trigger_calculate_next_run_at ON jobs;
DROP TRIGGER IF EXISTS trigger_calculate_duration ON job_executions;

-- ============================================================================
-- STEP 2: Drop functions
-- ============================================================================
DROP FUNCTION IF EXISTS calculate_next_run_at();
DROP FUNCTION IF EXISTS calculate_execution_duration();
DROP FUNCTION IF EXISTS cleanup_old_executions();

-- ============================================================================
-- STEP 3: Drop job_executions table
-- ============================================================================
DROP TABLE IF EXISTS job_executions CASCADE;

-- ============================================================================
-- STEP 4: Remove new columns from jobs table
-- ============================================================================
ALTER TABLE jobs DROP COLUMN IF EXISTS interval_minutes;
ALTER TABLE jobs DROP COLUMN IF EXISTS interval_type;
ALTER TABLE jobs DROP COLUMN IF EXISTS next_run_at;
ALTER TABLE jobs DROP COLUMN IF EXISTS is_paused;
ALTER TABLE jobs DROP COLUMN IF EXISTS max_retries;
ALTER TABLE jobs DROP COLUMN IF EXISTS retry_backoff_seconds;
ALTER TABLE jobs DROP COLUMN IF EXISTS current_retry_count;
ALTER TABLE jobs DROP COLUMN IF EXISTS lock_token;
ALTER TABLE jobs DROP COLUMN IF EXISTS lock_acquired_at;
ALTER TABLE jobs DROP COLUMN IF EXISTS paused_at;
ALTER TABLE jobs DROP COLUMN IF EXISTS cancelled_at;
ALTER TABLE jobs DROP COLUMN IF EXISTS metadata;

-- ============================================================================
-- STEP 5: Drop new indexes
-- ============================================================================
DROP INDEX IF EXISTS idx_jobs_next_run;
DROP INDEX IF EXISTS idx_jobs_lock_token;
DROP INDEX IF EXISTS idx_jobs_schedule_enabled;

-- ============================================================================
-- STEP 6: Restore original status constraint
-- ============================================================================
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS valid_status;
ALTER TABLE jobs ADD CONSTRAINT valid_status CHECK (status IN (
    'pending', 'processing', 'completed', 'failed'
));

-- ============================================================================
-- STEP 7: Drop new constraints
-- ============================================================================
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS valid_interval;
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS valid_interval_type;

-- ============================================================================
-- STEP 8: Revert job statuses to original states
-- ============================================================================
UPDATE jobs
SET status = CASE
    WHEN status = 'scheduled' THEN 'pending'
    WHEN status = 'running' THEN 'processing'
    WHEN status = 'paused' THEN 'pending'
    WHEN status = 'cancelled' THEN 'failed'
    ELSE status
END;

-- ============================================================================
-- STEP 9: Recreate original index on status
-- ============================================================================
DROP INDEX IF EXISTS idx_jobs_status;
CREATE INDEX idx_jobs_status ON jobs(status);

-- ============================================================================
-- STEP 10: Remove comments
-- ============================================================================
COMMENT ON COLUMN jobs.schedule_time IS NULL;

-- ============================================================================
-- STEP 11: Inform about backup table
-- ============================================================================
-- NOTE: jobs_backup table still exists for manual data recovery if needed
-- To fully restore from backup:
-- DROP TABLE jobs CASCADE;
-- ALTER TABLE jobs_backup RENAME TO jobs;
-- Then recreate indexes and triggers from 001_create_jobs_table.up.sql

COMMIT;
