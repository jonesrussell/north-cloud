-- crawler/migrations/011_add_unique_source_id.up.sql
-- Migration: Add unique constraint on source_id for auto-managed jobs
-- Description: Enables ON CONFLICT upsert for one job per source

BEGIN;

-- Drop the existing index (if it exists) and create a unique constraint
DROP INDEX IF EXISTS idx_jobs_source_id;

-- Add unique constraint on source_id
-- This ensures one job per source, enabling upsert semantics
ALTER TABLE jobs
    ADD CONSTRAINT jobs_source_id_unique UNIQUE (source_id);

-- Documentation
COMMENT ON CONSTRAINT jobs_source_id_unique ON jobs IS 'One job per source for auto-managed job lifecycle';

COMMIT;
