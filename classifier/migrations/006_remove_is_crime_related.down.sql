-- Migration: Restore is_crime_related column
-- Description: Restores the is_crime_related column (reverse of migration 006)

-- Add back the is_crime_related column
ALTER TABLE classification_history ADD COLUMN IF NOT EXISTS is_crime_related BOOLEAN DEFAULT FALSE;

-- Recreate the index on is_crime_related
CREATE INDEX IF NOT EXISTS idx_history_is_crime ON classification_history(is_crime_related);

-- Restore original comment
COMMENT ON TABLE classification_history IS 'Audit trail of all classifications for analysis and ML training';

