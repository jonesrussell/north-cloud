-- crawler/migrations/012_fix_priority_constraint.down.sql
-- Rollback: Restore original priority constraint

BEGIN;

-- Drop new constraint
ALTER TABLE jobs DROP CONSTRAINT IF EXISTS valid_priority;

-- Restore old constraint (this may fail if data has priority > 3)
ALTER TABLE jobs ADD CONSTRAINT valid_priority CHECK (
    priority BETWEEN 1 AND 3
);

-- Restore old comment
COMMENT ON COLUMN jobs.priority IS 'Job priority: 1=high, 2=normal (default), 3=low';

COMMIT;
