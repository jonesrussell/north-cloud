-- Migration: Cleanup V1 Scheduler
-- Description: Removes V1 scheduler columns and constraints after full V2 migration
-- Author: Claude Code
-- Date: 2026-01-26
--
-- WARNING: Only run this migration AFTER all jobs have been migrated to V2
-- and the V1 scheduler has been decommissioned.
--
-- Verification before running:
--   SELECT COUNT(*) FROM jobs WHERE scheduler_version = 1;
--   -- Should return 0

BEGIN;

-- ============================================================================
-- STEP 1: Verify no V1 jobs remain (safety check)
-- ============================================================================

DO $$
DECLARE
    v1_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO v1_count FROM jobs WHERE scheduler_version = 1 OR scheduler_version IS NULL;
    IF v1_count > 0 THEN
        RAISE EXCEPTION 'Cannot cleanup V1: % jobs still use V1 scheduler', v1_count;
    END IF;
END $$;

-- ============================================================================
-- STEP 2: Remove legacy locking columns (no longer needed with Redis)
-- ============================================================================

ALTER TABLE jobs DROP COLUMN IF EXISTS lock_token;
ALTER TABLE jobs DROP COLUMN IF EXISTS lock_acquired_at;

-- ============================================================================
-- STEP 3: Remove legacy cron column (replaced by cron_expression)
-- ============================================================================

ALTER TABLE jobs DROP COLUMN IF EXISTS schedule_time;

-- ============================================================================
-- STEP 4: Remove scheduler_version column (all jobs are V2)
-- ============================================================================

-- First drop the constraint
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS valid_scheduler_version;

-- Then drop the column
ALTER TABLE jobs DROP COLUMN IF EXISTS scheduler_version;

-- ============================================================================
-- STEP 5: Drop V1-specific indexes
-- ============================================================================

-- These indexes were for the polling-based V1 scheduler
DROP INDEX IF EXISTS idx_jobs_status_next_run;
DROP INDEX IF EXISTS idx_jobs_lock_status;

-- ============================================================================
-- STEP 6: Update default for schedule_type (no longer need 'interval' default)
-- ============================================================================

-- Keep the constraint but remove the default since V2 requires explicit type
ALTER TABLE jobs ALTER COLUMN schedule_type DROP DEFAULT;

-- ============================================================================
-- STEP 7: Drop scheduler consumer offsets table if empty
-- ============================================================================

-- Only drop if not being used (safety check)
DO $$
DECLARE
    offset_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO offset_count FROM scheduler_consumer_offsets;
    IF offset_count = 0 THEN
        DROP TABLE IF EXISTS scheduler_consumer_offsets;
    END IF;
END $$;

-- ============================================================================
-- STEP 8: Add comment indicating V2-only schema
-- ============================================================================

COMMENT ON TABLE jobs IS 'Crawler jobs (V2 scheduler only - Redis Streams based)';

COMMIT;
