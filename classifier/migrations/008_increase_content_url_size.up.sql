-- Migration 008: Increase content_url column size
-- Description: Change content_url from VARCHAR(500) to TEXT to accommodate long URLs
-- Date: 2026-01-08
-- 
-- URLs from crawled content can exceed 500 characters, especially with query parameters,
-- tracking IDs, and other metadata. This migration increases the column size to TEXT
-- (unlimited length) to prevent "value too long" errors.

BEGIN;

-- Alter content_url column from VARCHAR(500) to TEXT
-- This is a safe operation that won't lose data (TEXT can hold any VARCHAR value)
ALTER TABLE classification_history 
    ALTER COLUMN content_url TYPE TEXT;

-- Update comment to reflect the change
COMMENT ON COLUMN classification_history.content_url IS 'URL of the classified content (TEXT type to support long URLs with query parameters)';

COMMIT;
