-- crawler/migrations/012_fix_priority_constraint.up.sql
-- Migration: Fix priority constraint to allow 0-100 range
-- Description: Updates valid_priority constraint to match migration 009's documented 0-100 scale
-- Note: Migration 005 set priority as 1-3, but migration 009 redefined it as 0-100

BEGIN;

-- Drop old constraint (1-3 range from migration 005)
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS valid_priority;

-- Add new constraint matching migration 009's 0-100 scale
ALTER TABLE jobs ADD CONSTRAINT valid_priority CHECK (
    priority >= 0 AND priority <= 100
);

-- Update comment to match the new scale
COMMENT ON COLUMN jobs.priority IS 'Numeric priority 0-100, higher = scheduled sooner (100=critical, 75=high, 50=normal, 25=low)';

COMMIT;
