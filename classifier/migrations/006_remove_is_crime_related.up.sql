-- Migration: Remove is_crime_related column from classification_history
-- Description: Removes the redundant is_crime_related boolean column in favor of using the topics array
-- Version: 006
-- Date: 2025-12-24

-- Drop the index on is_crime_related first (required before dropping column)
DROP INDEX IF EXISTS idx_history_is_crime;

-- Drop the is_crime_related column
ALTER TABLE classification_history DROP COLUMN IF EXISTS is_crime_related;

-- Comments
COMMENT ON TABLE classification_history IS 'Audit trail of all classifications for analysis and ML training. Use topics array to check for crime-related content.';

