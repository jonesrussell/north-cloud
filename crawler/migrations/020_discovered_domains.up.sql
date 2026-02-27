-- Add domain tracking columns to discovered_links and create domain state table.

BEGIN;

-- Step 1: Add new columns to discovered_links (nullable initially for backfill).
ALTER TABLE discovered_links
    ADD COLUMN domain       VARCHAR(255),
    ADD COLUMN http_status  SMALLINT,
    ADD COLUMN content_type VARCHAR(100);

-- Step 2: Backfill domain from existing URLs (extract hostname, strip www. prefix).
UPDATE discovered_links
SET domain = regexp_replace(
    substring(url FROM '://([^/:]+)'),
    '^www\.', ''
)
WHERE domain IS NULL AND url IS NOT NULL;

-- Step 3: Make domain NOT NULL after backfill.
ALTER TABLE discovered_links
    ALTER COLUMN domain SET NOT NULL;

-- Step 4: Add indexes for domain queries.
CREATE INDEX idx_discovered_links_domain ON discovered_links (domain);
CREATE INDEX idx_discovered_links_domain_status ON discovered_links (domain, status);

-- Step 5: Create discovered_domain_states table.
CREATE TABLE IF NOT EXISTS discovered_domain_states (
    domain              VARCHAR(255) PRIMARY KEY,
    status              VARCHAR(20) NOT NULL DEFAULT 'active',
    notes               TEXT,
    ignored_at          TIMESTAMP WITH TIME ZONE,
    ignored_by          VARCHAR(255),
    promoted_at         TIMESTAMP WITH TIME ZONE,
    promoted_source_id  VARCHAR(36),
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Step 6: Add update trigger for updated_at on the new table.
CREATE TRIGGER update_discovered_domain_states_updated_at BEFORE UPDATE ON discovered_domain_states
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
