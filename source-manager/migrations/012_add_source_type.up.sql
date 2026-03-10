ALTER TABLE sources ADD COLUMN type TEXT NOT NULL DEFAULT 'news';

-- Backfill: sources with indigenous_region are indigenous type
UPDATE sources SET type = 'indigenous' WHERE indigenous_region IS NOT NULL;
