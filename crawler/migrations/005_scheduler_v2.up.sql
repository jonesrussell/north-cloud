-- Migration: Scheduler V2 Schema
-- Description: Adds cron scheduling, priority queues, event triggers, and V2 scheduler support
-- Author: Claude Code
-- Date: 2026-01-26

BEGIN;

-- ============================================================================
-- STEP 1: Add new scheduling columns to jobs table
-- ============================================================================

-- Cron scheduling support
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS cron_expression TEXT;

-- Schedule type to support multiple scheduling modes
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS schedule_type VARCHAR(20) DEFAULT 'interval';

-- Priority for queue ordering (1=high, 2=normal, 3=low)
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS priority INTEGER DEFAULT 2;

-- Execution timeout
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS timeout_seconds INTEGER DEFAULT 3600;

-- Job dependencies (array of job UUIDs)
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS depends_on TEXT[];

-- Event triggers
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS trigger_webhook TEXT;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS trigger_channel TEXT;

-- Scheduler version for migration (1=v1 interval, 2=v2 redis streams)
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS scheduler_version INTEGER DEFAULT 1;

-- ============================================================================
-- STEP 2: Add constraints for new columns
-- ============================================================================

-- Valid schedule types
ALTER TABLE jobs ADD CONSTRAINT valid_schedule_type CHECK (
    schedule_type IN ('cron', 'interval', 'immediate', 'event')
);

-- Valid priority values
ALTER TABLE jobs ADD CONSTRAINT valid_priority CHECK (
    priority BETWEEN 1 AND 3
);

-- Positive timeout
ALTER TABLE jobs ADD CONSTRAINT valid_timeout CHECK (
    timeout_seconds > 0
);

-- Scheduler version must be 1 or 2
ALTER TABLE jobs ADD CONSTRAINT valid_scheduler_version CHECK (
    scheduler_version IN (1, 2)
);

-- Cron expression required when schedule_type is 'cron'
ALTER TABLE jobs ADD CONSTRAINT cron_expression_required CHECK (
    (schedule_type = 'cron' AND cron_expression IS NOT NULL) OR
    (schedule_type != 'cron')
);

-- Event trigger required when schedule_type is 'event'
ALTER TABLE jobs ADD CONSTRAINT event_trigger_required CHECK (
    (schedule_type = 'event' AND (trigger_webhook IS NOT NULL OR trigger_channel IS NOT NULL)) OR
    (schedule_type != 'event')
);

-- ============================================================================
-- STEP 3: Create indexes for V2 scheduling
-- ============================================================================

-- Index for V2 scheduler to find ready jobs by priority
CREATE INDEX IF NOT EXISTS idx_jobs_v2_schedule
    ON jobs(schedule_type, priority, next_run_at)
    WHERE scheduler_version = 2 AND is_paused = false;

-- Index for cron jobs
CREATE INDEX IF NOT EXISTS idx_jobs_cron_expression
    ON jobs(cron_expression)
    WHERE cron_expression IS NOT NULL AND schedule_type = 'cron';

-- Index for event-triggered jobs
CREATE INDEX IF NOT EXISTS idx_jobs_trigger_channel
    ON jobs(trigger_channel)
    WHERE trigger_channel IS NOT NULL AND schedule_type = 'event';

-- Index for priority queue ordering
CREATE INDEX IF NOT EXISTS idx_jobs_priority_queue
    ON jobs(priority, created_at)
    WHERE scheduler_version = 2 AND status IN ('pending', 'scheduled');

-- ============================================================================
-- STEP 4: Add comments for new columns
-- ============================================================================

COMMENT ON COLUMN jobs.cron_expression IS 'Cron expression for schedule_type=cron (e.g., "0 */6 * * *")';
COMMENT ON COLUMN jobs.schedule_type IS 'Scheduling mode: cron, interval, immediate, or event';
COMMENT ON COLUMN jobs.priority IS 'Job priority: 1=high, 2=normal (default), 3=low';
COMMENT ON COLUMN jobs.timeout_seconds IS 'Maximum execution time before timeout (default: 3600)';
COMMENT ON COLUMN jobs.depends_on IS 'Array of job UUIDs that must complete before this job runs';
COMMENT ON COLUMN jobs.trigger_webhook IS 'Webhook pattern that triggers this job (e.g., "/sources/*/crawl")';
COMMENT ON COLUMN jobs.trigger_channel IS 'Redis Pub/Sub channel that triggers this job';
COMMENT ON COLUMN jobs.scheduler_version IS 'Scheduler version: 1=v1 (PostgreSQL polling), 2=v2 (Redis Streams)';

-- ============================================================================
-- STEP 5: Create function to validate job dependencies exist
-- ============================================================================

CREATE OR REPLACE FUNCTION validate_job_dependencies()
RETURNS TRIGGER AS $$
DECLARE
    dep_id TEXT;
BEGIN
    IF NEW.depends_on IS NOT NULL THEN
        FOREACH dep_id IN ARRAY NEW.depends_on
        LOOP
            IF NOT EXISTS (SELECT 1 FROM jobs WHERE id = dep_id::UUID) THEN
                RAISE EXCEPTION 'Dependency job % does not exist', dep_id;
            END IF;
        END LOOP;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_validate_job_dependencies
    BEFORE INSERT OR UPDATE ON jobs
    FOR EACH ROW
    WHEN (NEW.depends_on IS NOT NULL)
    EXECUTE FUNCTION validate_job_dependencies();

-- ============================================================================
-- STEP 6: Create function to check if job dependencies are satisfied
-- ============================================================================

CREATE OR REPLACE FUNCTION check_job_dependencies_satisfied(job_id UUID)
RETURNS BOOLEAN AS $$
DECLARE
    deps TEXT[];
    dep_id TEXT;
BEGIN
    SELECT depends_on INTO deps FROM jobs WHERE id = job_id;

    IF deps IS NULL OR array_length(deps, 1) IS NULL THEN
        RETURN TRUE;
    END IF;

    FOREACH dep_id IN ARRAY deps
    LOOP
        IF NOT EXISTS (
            SELECT 1 FROM jobs
            WHERE id = dep_id::UUID
            AND status = 'completed'
        ) THEN
            RETURN FALSE;
        END IF;
    END LOOP;

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- STEP 7: Create table for tracking Redis Streams consumer offsets
-- ============================================================================

CREATE TABLE IF NOT EXISTS scheduler_consumer_offsets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    consumer_group VARCHAR(100) NOT NULL,
    stream_name VARCHAR(100) NOT NULL,
    last_id VARCHAR(50) NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT unique_consumer_stream UNIQUE (consumer_group, stream_name)
);

COMMENT ON TABLE scheduler_consumer_offsets IS 'Tracks Redis Streams consumer group offsets for recovery';

-- ============================================================================
-- STEP 8: Create table for leader election state
-- ============================================================================

CREATE TABLE IF NOT EXISTS scheduler_leader_election (
    id VARCHAR(50) PRIMARY KEY DEFAULT 'scheduler-leader',
    leader_id VARCHAR(100) NOT NULL,
    acquired_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb
);

COMMENT ON TABLE scheduler_leader_election IS 'Leader election state for distributed scheduler coordination';

-- Create index for quick expiry checks
CREATE INDEX IF NOT EXISTS idx_leader_election_expires
    ON scheduler_leader_election(expires_at);

-- ============================================================================
-- STEP 9: Update existing interval jobs to have schedule_type='interval'
-- ============================================================================

UPDATE jobs
SET schedule_type = 'interval'
WHERE interval_minutes IS NOT NULL
  AND schedule_type IS NULL;

UPDATE jobs
SET schedule_type = 'immediate'
WHERE interval_minutes IS NULL
  AND schedule_enabled = false
  AND schedule_type IS NULL;

COMMIT;
