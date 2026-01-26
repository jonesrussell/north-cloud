-- Migration: Rollback Scheduler V2 Schema
-- Description: Removes V2 scheduler columns and tables
-- Author: Claude Code
-- Date: 2026-01-26

BEGIN;

-- ============================================================================
-- STEP 1: Drop leader election table
-- ============================================================================

DROP TABLE IF EXISTS scheduler_leader_election;

-- ============================================================================
-- STEP 2: Drop consumer offsets table
-- ============================================================================

DROP TABLE IF EXISTS scheduler_consumer_offsets;

-- ============================================================================
-- STEP 3: Drop triggers and functions
-- ============================================================================

DROP TRIGGER IF EXISTS trigger_validate_job_dependencies ON jobs;
DROP FUNCTION IF EXISTS validate_job_dependencies();
DROP FUNCTION IF EXISTS check_job_dependencies_satisfied(UUID);

-- ============================================================================
-- STEP 4: Drop indexes
-- ============================================================================

DROP INDEX IF EXISTS idx_jobs_v2_schedule;
DROP INDEX IF EXISTS idx_jobs_cron_expression;
DROP INDEX IF EXISTS idx_jobs_trigger_channel;
DROP INDEX IF EXISTS idx_jobs_priority_queue;

-- ============================================================================
-- STEP 5: Drop constraints
-- ============================================================================

ALTER TABLE jobs DROP CONSTRAINT IF EXISTS valid_schedule_type;
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS valid_priority;
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS valid_timeout;
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS valid_scheduler_version;
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS cron_expression_required;
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS event_trigger_required;

-- ============================================================================
-- STEP 6: Drop columns
-- ============================================================================

ALTER TABLE jobs DROP COLUMN IF EXISTS cron_expression;
ALTER TABLE jobs DROP COLUMN IF EXISTS schedule_type;
ALTER TABLE jobs DROP COLUMN IF EXISTS priority;
ALTER TABLE jobs DROP COLUMN IF EXISTS timeout_seconds;
ALTER TABLE jobs DROP COLUMN IF EXISTS depends_on;
ALTER TABLE jobs DROP COLUMN IF EXISTS trigger_webhook;
ALTER TABLE jobs DROP COLUMN IF EXISTS trigger_channel;
ALTER TABLE jobs DROP COLUMN IF EXISTS scheduler_version;

COMMIT;
