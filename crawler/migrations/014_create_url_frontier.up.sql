-- Create url_frontier table
CREATE TABLE IF NOT EXISTS url_frontier (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    url             TEXT NOT NULL,
    url_hash        CHAR(64) NOT NULL,
    host            TEXT NOT NULL,
    source_id       VARCHAR(36) NOT NULL,
    origin          VARCHAR(20) NOT NULL,
    parent_url      TEXT,
    depth           SMALLINT NOT NULL DEFAULT 0,
    priority        SMALLINT NOT NULL DEFAULT 5,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    next_fetch_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_fetched_at TIMESTAMP WITH TIME ZONE,
    fetch_count     INTEGER NOT NULL DEFAULT 0,
    content_hash    CHAR(64),
    etag            TEXT,
    last_modified   TEXT,
    retry_count     SMALLINT NOT NULL DEFAULT 0,
    last_error      TEXT,
    discovered_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_frontier_url_hash UNIQUE (url_hash)
);

-- Create index for claiming next batch of URLs to fetch
CREATE INDEX idx_frontier_claimable ON url_frontier (priority DESC, next_fetch_at ASC) WHERE status = 'pending';

-- Create index for host-based politeness lookups
CREATE INDEX idx_frontier_host ON url_frontier (host, last_fetched_at DESC);

-- Create index for source and status filtering
CREATE INDEX idx_frontier_source_status ON url_frontier (source_id, status);

-- Create index for content deduplication
CREATE INDEX idx_frontier_content_hash ON url_frontier (content_hash) WHERE content_hash IS NOT NULL;

-- Create trigger to automatically update updated_at
CREATE TRIGGER update_url_frontier_updated_at BEFORE UPDATE ON url_frontier
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
