-- crawler/migrations/010_add_job_migration_status.down.sql
-- Migration: Remove migration status column
-- Description: Rollback for 010_add_job_migration_status.up.sql

BEGIN;

DROP INDEX IF EXISTS idx_jobs_migration_status;

ALTER TABLE jobs
    DROP COLUMN IF EXISTS migration_status;

COMMIT;
