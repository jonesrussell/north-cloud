-- Migration: Remove job execution log metadata columns
-- Description: Rollback for 007_add_job_logs.up.sql

BEGIN;

-- Drop index first
DROP INDEX IF EXISTS idx_job_executions_log_object_key;

-- Remove log metadata columns
ALTER TABLE job_executions
DROP COLUMN IF EXISTS log_object_key,
DROP COLUMN IF EXISTS log_size_bytes,
DROP COLUMN IF EXISTS log_line_count;

COMMIT;
