-- crawler/migrations/011_add_unique_source_id.down.sql
-- Rollback: Remove unique constraint on source_id

BEGIN;

-- Remove unique constraint
ALTER TABLE jobs
    DROP CONSTRAINT IF EXISTS jobs_source_id_unique;

-- Recreate the regular index
CREATE INDEX IF NOT EXISTS idx_jobs_source_id
    ON jobs (source_id);

COMMIT;
