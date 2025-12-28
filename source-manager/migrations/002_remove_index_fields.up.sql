-- Migration: Remove article_index and page_index columns from sources
-- Description: Removes the redundant article_index and page_index columns since index names are now derived dynamically from source names
-- Version: 002
-- Date: 2024-12-24

-- Drop the index on article_index first (required before dropping column)
DROP INDEX IF EXISTS idx_sources_article_index;

-- Drop the article_index column
ALTER TABLE sources DROP COLUMN IF EXISTS article_index;

-- Drop the page_index column
ALTER TABLE sources DROP COLUMN IF EXISTS page_index;

-- Add a comment to the table to reflect the change
COMMENT ON TABLE sources IS 'Content sources configuration. Index names (e.g., {source_name}_raw_content) are derived dynamically from source names, not stored.';

