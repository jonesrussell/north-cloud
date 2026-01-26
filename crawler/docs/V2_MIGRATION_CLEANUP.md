# V2 Scheduler Migration Cleanup Guide

This document describes the cleanup steps to perform after the V2 scheduler has been validated in production and all jobs have been migrated from V1.

## Prerequisites

Before performing cleanup:

1. **All jobs migrated**: Verify no V1 jobs remain
   ```sql
   SELECT COUNT(*) FROM jobs WHERE scheduler_version = 1 OR scheduler_version IS NULL;
   -- Must return 0
   ```

2. **V2 scheduler stable**: V2 scheduler running in production for at least 2 weeks without issues

3. **Monitoring verified**: All Prometheus metrics and alerts working correctly

4. **Backup taken**: Full database backup before cleanup

## Migration 006: Database Cleanup

Run the cleanup migration to remove V1-specific columns:

```bash
cd crawler && go run cmd/migrate/main.go up
```

This migration:
- Removes `lock_token` and `lock_acquired_at` columns (V1 locking)
- Removes `schedule_time` column (legacy cron)
- Removes `scheduler_version` column
- Drops V1-specific indexes

## Files to Delete

After migration 006 succeeds, delete these V1 scheduler files:

### Core V1 Scheduler
```
crawler/internal/scheduler/interval_scheduler.go  # V1 scheduler implementation
crawler/internal/scheduler/metrics.go             # V1 metrics (replaced by v2/observability)
crawler/internal/scheduler/options.go             # V1 scheduler options
crawler/internal/scheduler/state_machine.go       # V1 state machine (if exists)
```

### Documentation
```
crawler/docs/DATABASE_SCHEDULER.md                # Legacy cron scheduler docs
crawler/docs/INTERVAL_SCHEDULER.md                # V1 interval scheduler docs (update or archive)
```

## Code Changes

### 1. Update cmd/httpd/httpd.go

Remove V1 scheduler initialization:
- Remove `NewIntervalScheduler()` calls
- Remove V1 scheduler start/stop
- Keep only V2 scheduler initialization

### 2. Update internal/api/api.go

Remove V1 deprecation middleware (no longer needed):
```go
// Remove: middleware.V1DeprecationMiddleware()
```

### 3. Update internal/database/interfaces.go

Remove V1-specific repository methods if no longer needed:
- `AcquireLock`
- `ReleaseLock`
- `ClearStaleLocks`

### 4. Update internal/domain/job.go

Remove V1-specific fields:
```go
// Remove these fields:
LockToken      *string
LockAcquiredAt *time.Time
ScheduleTime   *string  // Legacy cron
SchedulerVersion int     // No longer needed
```

### 5. Update internal/database/job_repository.go

- Remove `scheduler_version` from all queries
- Remove lock-related methods
- Update column lists in SELECT/INSERT/UPDATE

## Verification Checklist

After cleanup, verify:

- [ ] `go build ./...` succeeds
- [ ] `golangci-lint run ./...` passes
- [ ] All tests pass: `go test ./...`
- [ ] V2 scheduler starts correctly
- [ ] Jobs are scheduled and executed
- [ ] Prometheus metrics are exported
- [ ] API endpoints respond correctly
- [ ] No V1 references in logs

## Rollback Plan

If issues arise:

1. Stop the V2 scheduler
2. Run migration rollback: `go run cmd/migrate/main.go down 1`
3. Restore deleted files from git: `git checkout HEAD~1 -- <files>`
4. Restart with V1 scheduler

## Timeline

Recommended timeline:
- Week 1-2: V2 scheduler in shadow mode (both running)
- Week 3-4: V2 as primary, V1 as fallback
- Week 5-6: Migrate all jobs to V2
- Week 7: Run cleanup migration
- Week 8: Delete V1 files

## Support

If you encounter issues during cleanup:
1. Check logs for errors
2. Verify database state
3. Restore from backup if needed
4. Contact the infrastructure team
