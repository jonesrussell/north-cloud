-- Migration: Rename queued_links to discovered_links
-- Description: Renames table, indexes, and trigger to reflect that these are discovery records, not queued items
-- Date: 2026-01-04

BEGIN;

-- Rename table
ALTER TABLE queued_links RENAME TO discovered_links;

-- Rename indexes
ALTER INDEX idx_queued_links_source_id RENAME TO idx_discovered_links_source_id;
ALTER INDEX idx_queued_links_status RENAME TO idx_discovered_links_status;
ALTER INDEX idx_queued_links_priority_queued RENAME TO idx_discovered_links_priority_queued;
ALTER INDEX idx_queued_links_source_status RENAME TO idx_discovered_links_source_status;

-- Rename constraint (if exists, PostgreSQL doesn't auto-rename with table)
-- Note: Constraint names may vary, so we check and rename if needed
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'unique_source_url' AND conrelid = 'discovered_links'::regclass) THEN
        ALTER TABLE discovered_links RENAME CONSTRAINT unique_source_url TO unique_discovered_link_source_url;
    END IF;
END $$;

-- Drop old trigger
DROP TRIGGER IF EXISTS update_queued_links_updated_at ON discovered_links;

-- Create new trigger with new name
CREATE TRIGGER update_discovered_links_updated_at BEFORE UPDATE ON discovered_links
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
