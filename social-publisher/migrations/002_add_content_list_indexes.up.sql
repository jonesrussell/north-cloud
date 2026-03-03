CREATE INDEX idx_content_type ON content (type);
CREATE INDEX idx_content_created ON content (created_at DESC);
CREATE INDEX idx_content_source ON content (source);
