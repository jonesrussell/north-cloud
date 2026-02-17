-- Create host_state table
CREATE TABLE IF NOT EXISTS host_state (
    host                TEXT PRIMARY KEY,
    last_fetch_at       TIMESTAMP WITH TIME ZONE,
    min_delay_ms        INTEGER NOT NULL DEFAULT 1000,
    robots_txt          TEXT,
    robots_fetched_at   TIMESTAMP WITH TIME ZONE,
    robots_ttl_hours    INTEGER NOT NULL DEFAULT 24,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Create trigger to automatically update updated_at
CREATE TRIGGER update_host_state_updated_at BEFORE UPDATE ON host_state
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
