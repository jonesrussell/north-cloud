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
    jobs_table_exists BOOLEAN;
BEGIN
    -- Check if jobs table exists
    SELECT EXISTS (
        SELECT FROM information_schema.tables 
        WHERE table_schema = 'public' 
        AND table_name = 'jobs'
    ) INTO jobs_table_exists;
    
    -- If table doesn't exist, skip the check (migrations may not have run yet)
    IF NOT jobs_table_exists THEN
        RAISE NOTICE 'Jobs table does not exist, skipping V1 job check';
        RETURN;
    END IF;
    
    -- Check for V1 jobs only if table exists
    SELECT COUNT(*) INTO v1_count FROM jobs WHERE scheduler_version = 1 OR scheduler_version IS NULL;
    IF v1_count > 0 THEN
        RAISE EXCEPTION 'Cannot cleanup V1: % jobs still use V1 scheduler', v1_count;
    END IF;
END $$;

-- ============================================================================
-- STEP 2: Remove legacy locking columns (no longer needed with Redis)
-- ============================================================================

DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'jobs') THEN
        ALTER TABLE jobs DROP COLUMN IF EXISTS lock_token;
        ALTER TABLE jobs DROP COLUMN IF EXISTS lock_acquired_at;
    END IF;
END $$;

-- ============================================================================
-- STEP 3: Remove legacy cron column (replaced by cron_expression)
-- ============================================================================

DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'jobs') THEN
        ALTER TABLE jobs DROP COLUMN IF EXISTS schedule_time;
    END IF;
END $$;

-- ============================================================================
-- STEP 4: Remove scheduler_version column (all jobs are V2)
-- ============================================================================

DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'jobs') THEN
        -- First drop the constraint
        ALTER TABLE jobs DROP CONSTRAINT IF EXISTS valid_scheduler_version;
        -- Then drop the column
        ALTER TABLE jobs DROP COLUMN IF EXISTS scheduler_version;
    END IF;
END $$;

-- ============================================================================
-- STEP 5: Drop V1-specific indexes
-- ============================================================================

-- These indexes were for the polling-based V1 scheduler
DROP INDEX IF EXISTS idx_jobs_status_next_run;
DROP INDEX IF EXISTS idx_jobs_lock_status;

-- ============================================================================
-- STEP 6: Update default for schedule_type (no longer need 'interval' default)
-- ============================================================================

DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'jobs') THEN
        -- Keep the constraint but remove the default since V2 requires explicit type
        ALTER TABLE jobs ALTER COLUMN schedule_type DROP DEFAULT;
    END IF;
END $$;

-- ============================================================================
-- STEP 7: Drop scheduler consumer offsets table if empty
-- ============================================================================

-- Only drop if not being used (safety check)
DO $$
DECLARE
    offset_count INTEGER;
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'scheduler_consumer_offsets') THEN
        SELECT COUNT(*) INTO offset_count FROM scheduler_consumer_offsets;
        IF offset_count = 0 THEN
            DROP TABLE scheduler_consumer_offsets;
        END IF;
    END IF;
END $$;

-- ============================================================================
-- STEP 8: Add comment indicating V2-only schema
-- ============================================================================

DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'jobs') THEN
        COMMENT ON TABLE jobs IS 'Crawler jobs (V2 scheduler only - Redis Streams based)';
    END IF;
END $$;

COMMIT;
