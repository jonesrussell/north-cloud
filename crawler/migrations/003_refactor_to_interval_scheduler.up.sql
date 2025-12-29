-- Migration: Refactor to Interval-Based Scheduler
-- Description: Adds interval-based scheduling, execution tracking, distributed locking, and job control
-- Author: Claude Code
-- Date: 2025-12-29

BEGIN;

-- ============================================================================
-- STEP 1: Backup existing jobs table
-- ============================================================================
CREATE TABLE IF NOT EXISTS jobs_backup AS SELECT * FROM jobs;

-- ============================================================================
-- STEP 2: Create job_executions table for execution history
-- ============================================================================
CREATE TABLE job_executions (
    -- Identity
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL,

    -- Execution tracking
    execution_number INTEGER NOT NULL,
    status VARCHAR(50) NOT NULL,

    -- Timing
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    duration_ms BIGINT,

    -- Results
    items_crawled INTEGER DEFAULT 0,
    items_indexed INTEGER DEFAULT 0,
    error_message TEXT,
    stack_trace TEXT,

    -- Resource tracking
    cpu_time_ms BIGINT,
    memory_peak_mb INTEGER,

    -- Retry tracking
    retry_attempt INTEGER DEFAULT 0,

    -- Metadata
    metadata JSONB DEFAULT '{}'::jsonb,

    -- Constraints
    CONSTRAINT valid_execution_status CHECK (status IN ('running', 'completed', 'failed', 'cancelled')),
    CONSTRAINT fk_job FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
);

-- Indexes for job_executions
CREATE INDEX idx_executions_job_id ON job_executions(job_id);
CREATE INDEX idx_executions_started_at ON job_executions(started_at DESC);
CREATE INDEX idx_executions_status ON job_executions(status);
CREATE INDEX idx_executions_job_status ON job_executions(job_id, status);

-- ============================================================================
-- STEP 3: Add new columns to jobs table
-- ============================================================================

-- Interval-based scheduling
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS interval_minutes INTEGER;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS interval_type VARCHAR(20) DEFAULT 'minutes';
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMP WITH TIME ZONE;

-- Job control
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS is_paused BOOLEAN DEFAULT false;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS max_retries INTEGER DEFAULT 3;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS retry_backoff_seconds INTEGER DEFAULT 60;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS current_retry_count INTEGER DEFAULT 0;

-- Distributed locking
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS lock_token UUID;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS lock_acquired_at TIMESTAMP WITH TIME ZONE;

-- Additional timestamps
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS paused_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMP WITH TIME ZONE;

-- Metadata
ALTER TABLE jobs ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}'::jsonb;

-- ============================================================================
-- STEP 4: Update job status constraint to include new states
-- ============================================================================
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS valid_status;
ALTER TABLE jobs ADD CONSTRAINT valid_status CHECK (status IN (
    'pending', 'scheduled', 'running', 'paused', 'cancelled', 'completed', 'failed'
));

-- ============================================================================
-- STEP 5: Add new constraints
-- ============================================================================
ALTER TABLE jobs ADD CONSTRAINT valid_interval CHECK (
    (interval_minutes IS NULL AND schedule_enabled = false) OR
    (interval_minutes IS NOT NULL AND interval_minutes > 0)
);

ALTER TABLE jobs ADD CONSTRAINT valid_interval_type CHECK (
    interval_type IN ('minutes', 'hours', 'days')
);

-- ============================================================================
-- STEP 6: Create indexes for new columns
-- ============================================================================
CREATE INDEX IF NOT EXISTS idx_jobs_next_run ON jobs(next_run_at)
    WHERE next_run_at IS NOT NULL AND is_paused = false;

CREATE INDEX IF NOT EXISTS idx_jobs_lock_token ON jobs(lock_token)
    WHERE lock_token IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_jobs_schedule_enabled ON jobs(schedule_enabled)
    WHERE schedule_enabled = true;

-- Drop old index if it exists and recreate with new condition
DROP INDEX IF EXISTS idx_jobs_status;
CREATE INDEX idx_jobs_status ON jobs(status)
    WHERE status IN ('pending', 'scheduled', 'running');

-- ============================================================================
-- STEP 7: Migrate existing cron schedules to intervals (best-effort)
-- ============================================================================

-- Convert common cron patterns to intervals
UPDATE jobs
SET
    interval_minutes = CASE
        -- Hourly: 0 * * * *
        WHEN schedule_time = '0 * * * *' THEN 60
        -- Every 2 hours: 0 */2 * * *
        WHEN schedule_time = '0 */2 * * *' THEN 120
        -- Every 3 hours: 0 */3 * * *
        WHEN schedule_time = '0 */3 * * *' THEN 180
        -- Every 6 hours: 0 */6 * * *
        WHEN schedule_time = '0 */6 * * *' THEN 360
        -- Every 12 hours: 0 */12 * * *
        WHEN schedule_time = '0 */12 * * *' THEN 720
        -- Daily at midnight: 0 0 * * *
        WHEN schedule_time = '0 0 * * *' THEN 1440
        -- Default: 60 minutes for anything else
        ELSE 60
    END,
    interval_type = CASE
        WHEN schedule_time = '0 * * * *' THEN 'hours'
        WHEN schedule_time LIKE '0 */%' THEN 'hours'
        WHEN schedule_time = '0 0 * * *' THEN 'days'
        ELSE 'minutes'
    END,
    next_run_at = CASE
        WHEN schedule_enabled = true THEN NOW() + INTERVAL '1 minute'
        ELSE NULL
    END
WHERE schedule_time IS NOT NULL AND schedule_enabled = true;

-- Set immediate jobs (no schedule) to have NULL interval
UPDATE jobs
SET
    interval_minutes = NULL,
    next_run_at = NULL
WHERE schedule_time IS NULL OR schedule_enabled = false;

-- ============================================================================
-- STEP 8: Update job statuses for new state machine
-- ============================================================================
UPDATE jobs
SET status = CASE
    WHEN status = 'pending' AND schedule_enabled = true AND interval_minutes IS NOT NULL THEN 'scheduled'
    WHEN status = 'pending' AND (schedule_enabled = false OR interval_minutes IS NULL) THEN 'pending'
    WHEN status = 'processing' THEN 'running'
    ELSE status
END;

-- ============================================================================
-- STEP 9: Create trigger to auto-calculate next_run_at
-- ============================================================================
CREATE OR REPLACE FUNCTION calculate_next_run_at()
RETURNS TRIGGER AS $$
BEGIN
    -- Only calculate if job is scheduled and not paused
    IF NEW.schedule_enabled = true AND NEW.is_paused = false AND NEW.interval_minutes IS NOT NULL THEN
        -- If next_run_at is null or in the past, calculate from now
        IF NEW.next_run_at IS NULL OR NEW.next_run_at < NOW() THEN
            CASE NEW.interval_type
                WHEN 'minutes' THEN
                    NEW.next_run_at = NOW() + (NEW.interval_minutes || ' minutes')::INTERVAL;
                WHEN 'hours' THEN
                    NEW.next_run_at = NOW() + (NEW.interval_minutes || ' hours')::INTERVAL;
                WHEN 'days' THEN
                    NEW.next_run_at = NOW() + (NEW.interval_minutes || ' days')::INTERVAL;
            END CASE;
        END IF;
    ELSIF NEW.schedule_enabled = false OR NEW.is_paused = true THEN
        -- Clear next_run_at for disabled/paused jobs
        NEW.next_run_at = NULL;
    END IF;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER trigger_calculate_next_run_at
    BEFORE INSERT OR UPDATE ON jobs
    FOR EACH ROW
    EXECUTE FUNCTION calculate_next_run_at();

-- ============================================================================
-- STEP 10: Create trigger to auto-calculate execution duration
-- ============================================================================
CREATE OR REPLACE FUNCTION calculate_execution_duration()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.completed_at IS NOT NULL AND NEW.started_at IS NOT NULL THEN
        NEW.duration_ms = EXTRACT(EPOCH FROM (NEW.completed_at - NEW.started_at)) * 1000;
    END IF;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER trigger_calculate_duration
    BEFORE UPDATE ON job_executions
    FOR EACH ROW
    WHEN (NEW.completed_at IS NOT NULL)
    EXECUTE FUNCTION calculate_execution_duration();

-- ============================================================================
-- STEP 11: Create function for cleaning up old executions
-- ============================================================================
CREATE OR REPLACE FUNCTION cleanup_old_executions()
RETURNS void AS $$
BEGIN
    -- Delete executions older than 30 days, keeping at least 100 most recent per job
    DELETE FROM job_executions
    WHERE id IN (
        SELECT id FROM (
            SELECT
                id,
                ROW_NUMBER() OVER (PARTITION BY job_id ORDER BY started_at DESC) as rn,
                started_at
            FROM job_executions
        ) ranked
        WHERE rn > 100 AND started_at < NOW() - INTERVAL '30 days'
    );
END;
$$ language 'plpgsql';

-- ============================================================================
-- STEP 12: Add comment noting cron migration
-- ============================================================================
COMMENT ON COLUMN jobs.schedule_time IS 'DEPRECATED: Use interval_minutes and interval_type instead. Kept for rollback compatibility.';
COMMENT ON COLUMN jobs.interval_minutes IS 'Number of intervals between job executions. NULL = run once immediately.';
COMMENT ON COLUMN jobs.interval_type IS 'Type of interval: minutes, hours, or days.';
COMMENT ON COLUMN jobs.next_run_at IS 'Next scheduled execution time. Auto-calculated based on interval.';
COMMENT ON COLUMN jobs.is_paused IS 'Whether the job is temporarily paused.';
COMMENT ON COLUMN jobs.lock_token IS 'Distributed lock token for multi-instance coordination.';

COMMENT ON TABLE job_executions IS 'Execution history for all job runs. Tracks timing, results, and resource usage.';

COMMIT;
