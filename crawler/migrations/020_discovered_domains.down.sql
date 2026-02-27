-- Revert discovered domains migration: drop domain state table and remove columns.

DROP TRIGGER IF EXISTS update_discovered_domain_states_updated_at ON discovered_domain_states;
DROP TABLE IF EXISTS discovered_domain_states;

DROP INDEX IF EXISTS idx_discovered_links_domain_status;
DROP INDEX IF EXISTS idx_discovered_links_domain;

ALTER TABLE discovered_links
    DROP COLUMN IF EXISTS content_type,
    DROP COLUMN IF EXISTS http_status,
    DROP COLUMN IF EXISTS domain;
