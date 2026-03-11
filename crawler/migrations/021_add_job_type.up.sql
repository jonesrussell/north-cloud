-- Add job type column to distinguish crawl vs leadership_scrape jobs.
-- Default 'crawl' ensures all existing jobs remain unchanged.
ALTER TABLE jobs ADD COLUMN type VARCHAR(50) NOT NULL DEFAULT 'crawl';

-- Index for filtering jobs by type
CREATE INDEX idx_jobs_type ON jobs(type);
