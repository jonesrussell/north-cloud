-- crawler/migrations/010_add_job_migration_status.up.sql
-- Migration: Add migration status tracking for Phase 3
-- Description: Track migration state for gradual job conversion

BEGIN;

-- Add column to track migration status
-- Values: NULL (not migrated), 'migrated', 'orphaned', 'skipped'
ALTER TABLE jobs
    ADD COLUMN IF NOT EXISTS migration_status VARCHAR(20);

-- Index for efficient migration queries
CREATE INDEX IF NOT EXISTS idx_jobs_migration_status
    ON jobs (migration_status)
    WHERE migration_status IS NOT NULL;

-- Documentation
COMMENT ON COLUMN jobs.migration_status IS 'Phase 3 migration status: migrated, orphaned, or skipped';

COMMIT;
