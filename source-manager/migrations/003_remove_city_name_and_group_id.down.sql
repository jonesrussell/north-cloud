-- Migration: Restore city_name and group_id columns
-- Description: Restores the city_name and group_id columns (reverse of migration 003)

-- Add back the city_name column
ALTER TABLE sources ADD COLUMN IF NOT EXISTS city_name VARCHAR(255);

-- Add back the group_id column
ALTER TABLE sources ADD COLUMN IF NOT EXISTS group_id VARCHAR(36);

-- Recreate the unique constraint on city_name (only if it doesn't exist)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'unique_city_name'
    ) THEN
        ALTER TABLE sources ADD CONSTRAINT unique_city_name UNIQUE (city_name) DEFERRABLE INITIALLY DEFERRED;
    END IF;
END $$;

-- Recreate the index on city_name
CREATE INDEX IF NOT EXISTS idx_sources_city_name ON sources(city_name);

