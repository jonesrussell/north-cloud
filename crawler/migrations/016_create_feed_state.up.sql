-- Create feed_state table
CREATE TABLE IF NOT EXISTS feed_state (
    source_id           VARCHAR(36) PRIMARY KEY,
    feed_url            TEXT NOT NULL,
    last_polled_at      TIMESTAMP WITH TIME ZONE,
    last_etag           TEXT,
    last_modified       TEXT,
    last_item_count     INTEGER NOT NULL DEFAULT 0,
    consecutive_errors  INTEGER NOT NULL DEFAULT 0,
    last_error          TEXT,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create trigger to automatically update updated_at
CREATE TRIGGER update_feed_state_updated_at BEFORE UPDATE ON feed_state
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
