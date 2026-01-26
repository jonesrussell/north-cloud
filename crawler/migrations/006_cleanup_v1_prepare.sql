-- Migration: Prepare for V1 Cleanup
-- Description: Delete all V1 jobs before running migration 006
-- WARNING: This will permanently delete all jobs using V1 scheduler

BEGIN;

-- Delete all jobs using V1 scheduler (scheduler_version = 1 or NULL)
DELETE FROM jobs 
WHERE scheduler_version = 1 OR scheduler_version IS NULL;

-- Verify deletion
DO $$
DECLARE
    remaining_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO remaining_count 
    FROM jobs 
    WHERE scheduler_version = 1 OR scheduler_version IS NULL;
    
    IF remaining_count > 0 THEN
        RAISE EXCEPTION 'Still % V1 jobs remaining after deletion', remaining_count;
    END IF;
    
    RAISE NOTICE 'Successfully deleted all V1 jobs. Migration 006 can now proceed.';
END $$;

COMMIT;
