# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with the crawler service.

## Quick Reference

```bash
# Development
task dev              # Start with hot reload
task test             # Run tests
task lint             # Run linter
task migrate:up       # Run migrations

# API (port 8060)
curl http://localhost:8060/api/v1/jobs
curl http://localhost:8060/api/v1/scheduler/metrics
```

## Architecture

```
internal/
├── api/           # HTTP handlers (Gin)
├── scheduler/     # Interval-based job scheduler (NOT cron)
├── crawler/       # Core web scraping logic
├── database/      # PostgreSQL repositories
├── domain/        # Job, JobExecution models
├── sources/       # Source manager API client
├── storage/       # Elasticsearch indexing
└── archive/       # MinIO HTML archiving
```

## Scheduler - Critical Concepts

### Interval-Based (NOT Cron)

Jobs use simple intervals, not cron expressions:
```json
{
  "interval_minutes": 30,
  "interval_type": "minutes",  // "minutes" | "hours" | "days"
  "schedule_enabled": true
}
```

### 7 Job States

```
pending ──→ scheduled ──→ running ──→ completed ──→ scheduled (recurring)
                       └─→ failed ──→ pending (retry)
scheduled ──→ paused ──→ scheduled (resume)
         └─→ cancelled
```

### State Transition Rules

| Action | Valid From States |
|--------|------------------|
| pause | scheduled only |
| resume | paused only |
| cancel | scheduled, running, paused, pending |
| retry | failed only |

### Distributed Locking

PostgreSQL CAS-based locking for multi-instance safety:
```sql
UPDATE jobs SET lock_token = ? WHERE id = ? AND lock_token IS NULL
```
- Stale locks cleared after 5 minutes
- Lock cleanup runs every 1 minute

## API Endpoints

**CRUD**: `GET/POST/PUT/DELETE /api/v1/jobs[/:id]`

**Job Control**:
- `POST /api/v1/jobs/:id/pause`
- `POST /api/v1/jobs/:id/resume`
- `POST /api/v1/jobs/:id/cancel`
- `POST /api/v1/jobs/:id/retry`

**History & Stats**:
- `GET /api/v1/jobs/:id/executions` - Execution history
- `GET /api/v1/jobs/:id/stats` - Success rate, avg duration
- `GET /api/v1/scheduler/metrics` - System-wide metrics

## Common Gotchas

1. **`source_id` is REQUIRED** - Jobs must have a valid `source_id` from source-manager. Job creation won't fail without it, but execution will.

2. **Interval fields are conditional**:
   - `interval_minutes = NULL` → one-time job
   - `interval_minutes > 0` → recurring job with auto-calculated `next_run_at`

3. **Retry backoff is exponential**: `base × 2^(attempt-1)`, capped at 1 hour

4. **Execution history pruning**: Auto-deletes executions > 30 days OR > 100 per job

5. **Lock stuck after crash**: Stale lock cleanup takes up to 5 minutes

## Database Schema

**jobs** table key fields:
- `source_id` (required), `url`, `status`
- `interval_minutes`, `interval_type`, `next_run_at`
- `is_paused`, `lock_token`, `lock_acquired_at`
- `max_retries`, `retry_backoff_seconds`, `current_retry_count`

**job_executions** table:
- `job_id`, `execution_number`, `status`
- `started_at`, `completed_at`, `duration_ms`
- `items_crawled`, `items_indexed`, `error_message`

## Code Examples

### Create Recurring Job
```bash
curl -X POST http://localhost:8060/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "source_id": "uuid-from-source-manager",
    "url": "https://example.com",
    "interval_minutes": 360,
    "interval_type": "minutes",
    "schedule_enabled": true
  }'
```

### Create One-Time Job
```bash
curl -X POST http://localhost:8060/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "source_id": "uuid-from-source-manager",
    "url": "https://example.com",
    "schedule_enabled": false
  }'
```

## Scheduler Internal Flow

1. **Job Poller** (10s interval): Queries jobs where `next_run_at <= NOW()`
2. **Lock Acquisition**: Atomic CAS update on `lock_token`
3. **Execution**: Creates `JobExecution`, runs crawler, updates status
4. **Reschedule**: On success, calculates next `next_run_at` for recurring jobs
5. **Retry**: On failure, applies exponential backoff or marks as failed

## Documentation

- `/crawler/docs/INTERVAL_SCHEDULER.md` - Comprehensive scheduler guide
