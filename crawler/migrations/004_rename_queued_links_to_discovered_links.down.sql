-- Rollback: Rename discovered_links back to queued_links

BEGIN;

-- Drop new trigger
DROP TRIGGER IF EXISTS update_discovered_links_updated_at ON discovered_links;

-- Rename constraint back
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'unique_discovered_link_source_url' AND conrelid = 'discovered_links'::regclass) THEN
        ALTER TABLE discovered_links RENAME CONSTRAINT unique_discovered_link_source_url TO unique_source_url;
    END IF;
END $$;

-- Rename indexes back
ALTER INDEX idx_discovered_links_source_id RENAME TO idx_queued_links_source_id;
ALTER INDEX idx_discovered_links_status RENAME TO idx_queued_links_status;
ALTER INDEX idx_discovered_links_priority_queued RENAME TO idx_queued_links_priority_queued;
ALTER INDEX idx_discovered_links_source_status RENAME TO idx_queued_links_source_status;

-- Rename table back
ALTER TABLE discovered_links RENAME TO queued_links;

-- Create old trigger
CREATE TRIGGER update_queued_links_updated_at BEFORE UPDATE ON queued_links
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
