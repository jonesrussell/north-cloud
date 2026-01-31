-- crawler/migrations/009_add_auto_managed_jobs.down.sql
-- Migration: Remove auto-managed job columns
-- Description: Rollback for 009_add_auto_managed_jobs.up.sql

BEGIN;

DROP INDEX IF EXISTS idx_jobs_due_priority;
DROP INDEX IF EXISTS idx_jobs_source_id;

ALTER TABLE jobs
    DROP COLUMN IF EXISTS auto_managed,
    DROP COLUMN IF EXISTS priority,
    DROP COLUMN IF EXISTS failure_count,
    DROP COLUMN IF EXISTS last_failure_at,
    DROP COLUMN IF EXISTS backoff_until;

COMMIT;
