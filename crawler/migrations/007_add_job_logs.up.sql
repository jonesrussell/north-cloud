-- Migration: Add job execution log metadata columns
-- Description: Adds columns for storing MinIO log object keys and metadata

BEGIN;

-- Add log metadata columns to job_executions
ALTER TABLE job_executions
ADD COLUMN IF NOT EXISTS log_object_key TEXT,
ADD COLUMN IF NOT EXISTS log_size_bytes BIGINT,
ADD COLUMN IF NOT EXISTS log_line_count INTEGER;

-- Index for efficient log retrieval by job_id
CREATE INDEX IF NOT EXISTS idx_job_executions_log_object_key
ON job_executions(log_object_key)
WHERE log_object_key IS NOT NULL;

-- Comment documentation
COMMENT ON COLUMN job_executions.log_object_key IS
    'MinIO object key for archived execution logs (gzipped)';
COMMENT ON COLUMN job_executions.log_size_bytes IS
    'Size of archived logs in bytes (compressed)';
COMMENT ON COLUMN job_executions.log_line_count IS
    'Number of log lines in the archived file';

COMMIT;
