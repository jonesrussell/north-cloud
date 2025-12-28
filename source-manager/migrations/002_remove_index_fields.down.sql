-- Migration: Restore article_index and page_index columns
-- Description: Restores the article_index and page_index columns (reverse of migration 002)

-- Add back the article_index column
ALTER TABLE sources ADD COLUMN IF NOT EXISTS article_index VARCHAR(255);

-- Add back the page_index column
ALTER TABLE sources ADD COLUMN IF NOT EXISTS page_index VARCHAR(255);

-- Recreate the index on article_index
CREATE INDEX IF NOT EXISTS idx_sources_article_index ON sources(article_index);

-- Remove the comment
COMMENT ON TABLE sources IS NULL;

