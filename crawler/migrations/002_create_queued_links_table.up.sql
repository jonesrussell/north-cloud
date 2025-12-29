-- Create queued_links table
CREATE TABLE IF NOT EXISTS queued_links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id VARCHAR(255) NOT NULL,
    source_name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    parent_url TEXT,
    depth INTEGER DEFAULT 0,
    discovered_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    queued_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    priority INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_source_url UNIQUE (source_id, url)
);

-- Create index on source_id for faster lookups
CREATE INDEX idx_queued_links_source_id ON queued_links(source_id);

-- Create index on status for faster filtering
CREATE INDEX idx_queued_links_status ON queued_links(status);

-- Create index on priority and queued_at for sorting
CREATE INDEX idx_queued_links_priority_queued ON queued_links(priority DESC, queued_at ASC);

-- Create composite index for source and status filtering
CREATE INDEX idx_queued_links_source_status ON queued_links(source_id, status);

-- Create trigger to automatically update updated_at
CREATE TRIGGER update_queued_links_updated_at BEFORE UPDATE ON queued_links
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

