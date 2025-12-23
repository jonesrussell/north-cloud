# Database-Backed Job Scheduler

## Overview

The database-backed scheduler replaces the old `sources.yml` configuration-based scheduler with a dynamic system that executes jobs stored in the PostgreSQL database. Jobs can be created, updated, and deleted via the REST API and are automatically picked up and executed by the scheduler.

## Key Features

### 1. Database-Backed Jobs
- Jobs are stored in PostgreSQL database
- Jobs can be created/updated/deleted via REST API (`/api/v1/jobs`)
- Scheduler automatically picks up new jobs without restart

### 2. Immediate Execution
- Jobs with `schedule_enabled: false` are executed immediately
- Checked every 10 seconds for new immediate jobs
- Perfect for one-time crawls triggered by user action

### 3. Cron-Based Scheduling
- Uses `robfig/cron/v3` library for cron expression parsing
- Supports standard cron format: `* * * * *` (minute hour day month weekday)
- Examples:
  - `0 * * * *` - Every hour
  - `*/30 * * * *` - Every 30 minutes
  - `0 0 * * *` - Daily at midnight
  - `0 9,17 * * 1-5` - 9 AM and 5 PM on weekdays

### 4. Job Status Tracking
- **pending**: Job is waiting to be executed
- **processing**: Job is currently running
- **completed**: Job finished successfully
- **failed**: Job encountered an error
- Timestamps tracked: `created_at`, `updated_at`, `started_at`, `completed_at`

### 5. Concurrency Safety
- Prevents duplicate execution of same job
- Tracks active jobs to avoid conflicts
- Thread-safe job management with mutexes

### 6. Periodic Job Reload
- Reloads jobs from database every 5 minutes
- Picks up schedule changes without restart
- Automatically adjusts to new/modified/deleted jobs

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     HTTP API Server (httpd)                  │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌────────────────┐        ┌──────────────────┐            │
│  │  Jobs Handler  │◄───────┤  Job Repository  │            │
│  │  (REST API)    │        │   (PostgreSQL)   │            │
│  └────────────────┘        └──────────────────┘            │
│                                      ▲                        │
│                                      │                        │
│                            ┌─────────┴─────────┐            │
│                            │                   │            │
│  ┌─────────────────────────┤  Database        │            │
│  │  DB Scheduler           │  Scheduler       │            │
│  │  ┌──────────────────┐   │                  │            │
│  │  │ Cron Scheduler   │   └──────────────────┘            │
│  │  │ (Scheduled Jobs) │             ▼                      │
│  │  └──────────────────┘   ┌──────────────────┐            │
│  │  ┌──────────────────┐   │   Crawler        │            │
│  │  │ Immediate Jobs   │───┤   Instance       │            │
│  │  │ (Check every 10s)│   └──────────────────┘            │
│  │  └──────────────────┘                                    │
│  │  ┌──────────────────┐                                    │
│  │  │ Periodic Reload  │                                    │
│  │  │ (Every 5 min)    │                                    │
│  │  └──────────────────┘                                    │
│  └────────────────────────                                  │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

## Implementation Details

### Files Created/Modified

#### New Files
- `crawler/internal/job/db_scheduler.go` - Database-backed scheduler implementation

#### Modified Files
- `crawler/cmd/httpd/httpd.go` - Integrated scheduler with httpd command
- `crawler/go.mod` - Added `github.com/robfig/cron/v3` dependency

### Database Schema

The scheduler uses the existing `jobs` table:

```sql
CREATE TABLE jobs (
    id              TEXT PRIMARY KEY,
    source_id       TEXT NOT NULL,
    source_name     TEXT,
    url             TEXT NOT NULL,
    schedule_time   TEXT,              -- Cron expression
    schedule_enabled BOOLEAN NOT NULL,  -- true=scheduled, false=immediate
    status          TEXT NOT NULL,      -- pending, processing, completed, failed
    created_at      TIMESTAMP NOT NULL,
    updated_at      TIMESTAMP NOT NULL,
    started_at      TIMESTAMP,
    completed_at    TIMESTAMP,
    error_message   TEXT
);
```

### Scheduler Configuration

Configurable constants in `db_scheduler.go`:

```go
const (
    checkInterval  = 10 * time.Second   // How often to check for immediate jobs
    reloadInterval = 5 * time.Minute    // How often to reload scheduled jobs
)
```

## Usage

### Starting the Scheduler

The scheduler starts automatically when the service starts:

```bash
./crawler
```

Logs will show:
```
INFO Starting database scheduler
INFO Database scheduler started successfully
```

### Creating Jobs via API

#### Immediate Job (Run Once, Now)
```bash
curl -X POST http://localhost:8060/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "source_id": "news-site",
    "source_name": "News Website",
    "url": "https://example.com",
    "schedule_enabled": false
  }'
```

#### Scheduled Job (Cron)
```bash
curl -X POST http://localhost:8060/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "source_id": "news-site",
    "source_name": "News Website",
    "url": "https://example.com",
    "schedule_time": "0 */6 * * *",
    "schedule_enabled": true
  }'
```

### Listing Jobs
```bash
# All jobs
curl http://localhost:8060/api/v1/jobs

# Filter by status
curl http://localhost:8060/api/v1/jobs?status=pending
curl http://localhost:8060/api/v1/jobs?status=completed

# Pagination
curl http://localhost:8060/api/v1/jobs?limit=20&offset=0
```

### Updating Jobs
```bash
curl -X PUT http://localhost:8060/api/v1/jobs/{job-id} \
  -H "Content-Type: application/json" \
  -d '{
    "schedule_time": "0 * * * *",
    "schedule_enabled": true
  }'
```

### Deleting Jobs
```bash
curl -X DELETE http://localhost:8060/api/v1/jobs/{job-id}
```

## Job Execution Flow

### For Scheduled Jobs
1. Job created with `schedule_enabled: true` and `schedule_time` (cron expression)
2. Scheduler loads job on startup or next reload (every 5 min)
3. Cron scheduler triggers job at specified times
4. Job status: `pending` → `processing` → `completed`/`failed`

### For Immediate Jobs
1. Job created with `schedule_enabled: false`
2. Scheduler checks every 10 seconds for pending immediate jobs
3. Job is executed immediately upon discovery
4. Job status: `pending` → `processing` → `completed`/`failed`

## Error Handling

### Job Failures
- Errors are logged with `job_id` and `error` fields
- Job status set to `failed`
- Error message stored in `error_message` field
- `completed_at` timestamp recorded

### Scheduler Failures
- Non-critical errors (e.g., failed to schedule one job) are logged but don't stop scheduler
- Critical errors (e.g., database connection lost) are logged as errors
- Scheduler continues processing other jobs

### Concurrency Protection
- Jobs already running are skipped if triggered again
- Active jobs tracked in memory to prevent duplicates
- Thread-safe operations with mutexes

## Monitoring

### Logs
The scheduler logs important events:

```
INFO Reloading jobs from database count=5
INFO Scheduling job job_id=abc123 schedule="0 * * * *"
INFO Found immediate job job_id=def456
INFO Executing job job_id=def456 url=https://example.com
INFO Job completed successfully job_id=def456
ERROR Failed to start crawler job_id=ghi789 error="connection refused"
```

### Metrics (Future Enhancement)
Recommended metrics to add:
- Total jobs processed
- Currently active jobs
- Failed job count
- Average job duration
- Jobs per status

## Migration from Old Scheduler

The old scheduler (`cmd/scheduler`) used `sources.yml` configuration. The new scheduler offers several advantages:

### Old Scheduler (`sources.yml`)
- Static configuration in YAML file
- Requires restart to add/modify sources
- Simple time-of-day matching (HH:MM)
- No job status tracking
- No API integration

### New Scheduler (Database)
- Dynamic configuration via database
- Live updates without restart
- Full cron expression support
- Complete job lifecycle tracking
- REST API for management
- Immediate execution support

### Migration Steps
1. The old scheduler can remain for backward compatibility if needed
2. Create database jobs for each source in `sources.yml`
3. Test database jobs work correctly
4. Gradually migrate all sources to database
5. Eventually deprecate old scheduler

**Note**: Since there's no requirement for backward compatibility, the old scheduler can be removed entirely.

## Best Practices

### Job Design
1. **Use source_name**: Helps identify jobs in logs and UI
2. **Set realistic schedules**: Don't overwhelm sites with too frequent crawls
3. **Monitor failed jobs**: Check `error_message` to diagnose issues
4. **Clean up old jobs**: Delete completed jobs periodically to avoid clutter

### Cron Expressions
1. **Test expressions**: Use https://crontab.guru to validate
2. **Consider timezones**: Server timezone applies to all cron jobs
3. **Avoid peak times**: Schedule heavy crawls during off-peak hours
4. **Use random delays**: Stagger jobs to avoid spikes

### Error Recovery
1. **Retry failed jobs**: Update status back to `pending` to retry
2. **Check error messages**: Diagnose issues before retrying
3. **Adjust schedules**: If jobs consistently fail, reduce frequency
4. **Monitor patterns**: Look for recurring failures

## Future Enhancements

### Planned Features
1. **Job retries**: Automatic retry with exponential backoff
2. **Job priorities**: High-priority jobs execute first
3. **Job dependencies**: Jobs that run after others complete
4. **Job timeouts**: Kill long-running jobs automatically
5. **Dead letter queue**: Permanently failed jobs moved to separate queue
6. **Metrics endpoint**: `/metrics` for monitoring
7. **Health checks**: `/health` endpoint for scheduler status
8. **Job history**: Archive completed jobs for audit trail
9. **Notifications**: Webhook/email alerts on job completion/failure
10. **Resource limits**: Max concurrent jobs, memory limits

### Configuration Options
Make intervals configurable via environment variables:
```bash
SCHEDULER_CHECK_INTERVAL=10s
SCHEDULER_RELOAD_INTERVAL=5m
SCHEDULER_MAX_CONCURRENT_JOBS=10
SCHEDULER_JOB_TIMEOUT=30m
```

## Troubleshooting

### Jobs Not Executing
1. Check scheduler is running: Look for "Database scheduler started successfully" in logs
2. Verify job status: Should be `pending` for immediate jobs
3. Check database connection: Ensure PostgreSQL is accessible
4. Check crawler setup: Ensure crawler was created successfully

### Jobs Fail Immediately
1. Check error_message: Look at job record in database
2. Verify URL: Ensure URL is accessible
3. Check source configuration: If using source_name, verify source exists
4. Check crawler logs: Look for detailed error information

### Schedule Not Triggering
1. Validate cron expression: Use crontab.guru
2. Check schedule_enabled: Must be `true`
3. Check schedule_time: Must be non-empty valid cron expression
4. Wait for reload: Schedules refresh every 5 minutes
5. Check timezone: Ensure server timezone matches expectations

### Duplicate Job Execution
1. Check active jobs tracking: Should prevent duplicates
2. Review logs: Look for "Job already running" warnings
3. Check database: Verify job IDs are unique
4. Restart scheduler: May resolve transient issues

## Support

For issues or questions:
1. Check logs: `/var/log/crawler/` or container logs
2. Review this documentation
3. Check database: Query `jobs` table for job status
4. File GitHub issue: Include logs and job details

## References

- [robfig/cron documentation](https://pkg.go.dev/github.com/robfig/cron/v3)
- [Cron expression syntax](https://crontab.guru)
- [PostgreSQL documentation](https://www.postgresql.org/docs/)
- [Gin Web Framework](https://gin-gonic.com/docs/)
