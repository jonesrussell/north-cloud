-- Migration 008 Rollback: Revert content_url column size
-- Description: Revert content_url from TEXT back to VARCHAR(500)
-- 
-- WARNING: This will truncate any URLs longer than 500 characters.
-- Only use this rollback if absolutely necessary.

BEGIN;

-- Revert content_url column from TEXT to VARCHAR(500)
-- WARNING: URLs longer than 500 characters will be truncated
ALTER TABLE classification_history 
    ALTER COLUMN content_url TYPE VARCHAR(500);

-- Restore original comment
COMMENT ON COLUMN classification_history.content_url IS 'URL of the classified content';

COMMIT;
