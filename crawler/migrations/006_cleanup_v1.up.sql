-- Migration: Cleanup V1 Scheduler (DEFERRED)
-- Description: This migration was originally intended to remove V1 scheduler columns,
-- but the code still depends on them. Keeping as no-op until code is updated.
--
-- TODO: Re-enable column drops once job_repository.go is updated to not use:
--   - schedule_time
--   - lock_token
--   - lock_acquired_at
--   - scheduler_version

-- No-op migration - columns kept for backward compatibility
SELECT 1;
