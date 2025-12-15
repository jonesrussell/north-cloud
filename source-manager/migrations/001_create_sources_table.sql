-- Create sources table
CREATE TABLE IF NOT EXISTS sources (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    article_index VARCHAR(255) NOT NULL,
    page_index VARCHAR(255) NOT NULL,
    rate_limit VARCHAR(50) NOT NULL DEFAULT '1s',
    max_depth INTEGER NOT NULL DEFAULT 2,
    time JSONB,
    selectors JSONB NOT NULL,
    city_name VARCHAR(255),
    group_id VARCHAR(36),
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_source_name UNIQUE (name),
    CONSTRAINT unique_city_name UNIQUE (city_name) DEFERRABLE INITIALLY DEFERRED
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_sources_name ON sources(name);
CREATE INDEX IF NOT EXISTS idx_sources_city_name ON sources(city_name);
CREATE INDEX IF NOT EXISTS idx_sources_enabled ON sources(enabled);
CREATE INDEX IF NOT EXISTS idx_sources_article_index ON sources(article_index);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger to automatically update updated_at
CREATE TRIGGER update_sources_updated_at
    BEFORE UPDATE ON sources
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

