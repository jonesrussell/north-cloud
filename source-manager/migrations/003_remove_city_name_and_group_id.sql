-- Migration: Remove city_name and group_id columns from sources
-- Description: Removes the city_name and group_id columns as they are no longer needed
-- Version: 003
-- Date: 2025-01-XX

-- Drop the unique constraint on city_name first (required before dropping column)
ALTER TABLE sources DROP CONSTRAINT IF EXISTS unique_city_name;

-- Drop the index on city_name
DROP INDEX IF EXISTS idx_sources_city_name;

-- Drop the city_name column
ALTER TABLE sources DROP COLUMN IF EXISTS city_name;

-- Drop the group_id column
ALTER TABLE sources DROP COLUMN IF EXISTS group_id;

