# Scheduler V2 Architecture

The V2 scheduler is a complete redesign of the crawler's job scheduling system, replacing the PostgreSQL-polling V1 scheduler with a Redis Streams-based architecture.

## Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Scheduler V2                                 │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐         │
│  │    Cron      │    │   Interval   │    │    Event     │         │
│  │  Scheduler   │    │  Scheduler   │    │   Triggers   │         │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘         │
│         │                   │                   │                  │
│         └───────────────────┼───────────────────┘                  │
│                             ▼                                       │
│                    ┌─────────────────┐                             │
│                    │  Redis Streams  │                             │
│                    │  Priority Queue │                             │
│                    └────────┬────────┘                             │
│                             │                                       │
│                             ▼                                       │
│                    ┌─────────────────┐                             │
│                    │   Worker Pool   │                             │
│                    │  (Semaphore)    │                             │
│                    └────────┬────────┘                             │
│                             │                                       │
│              ┌──────────────┼──────────────┐                       │
│              ▼              ▼              ▼                       │
│         ┌────────┐    ┌────────┐    ┌────────┐                    │
│         │Worker 1│    │Worker 2│    │Worker N│                    │
│         └────────┘    └────────┘    └────────┘                    │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Key Features

### 1. Multiple Scheduling Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| `cron` | Standard cron expressions | "Run at 6am daily" |
| `interval` | Simple time intervals | "Run every 30 minutes" |
| `immediate` | Run once immediately | One-time crawls |
| `event` | Triggered by external events | Webhook/Pub-Sub triggers |

### 2. Priority Queues

Jobs are processed by priority:
- **High (1)**: Critical jobs, processed first
- **Normal (2)**: Standard jobs (default)
- **Low (3)**: Background jobs, processed when capacity allows

### 3. Redis Streams

Uses Redis Streams for reliable job queuing:
- Consumer groups for distributed processing
- Automatic retry with pending entry list (PEL)
- Message acknowledgment prevents loss

### 4. Bounded Worker Pool

Semaphore-based concurrency control:
- Configurable pool size (default: 10)
- Prevents resource exhaustion
- Graceful draining for deployments

### 5. Event Triggers

**Webhooks**: HTTP endpoints that trigger jobs
```
POST /api/v2/triggers/webhook/sources/*/crawl
```

**Redis Pub/Sub**: Subscribe to channels for event-driven scheduling
```
Channel: content:new → Triggers associated jobs
```

### 6. Circuit Breakers

Per-domain circuit breakers prevent cascading failures:
- Tracks success/failure rates
- Opens on threshold breach
- Half-open state for recovery testing

### 7. Leader Election

Redis-based leader election for distributed deployments:
- Only leader schedules cron jobs
- Automatic failover on leader loss
- Prevents duplicate scheduling

## Configuration

```bash
# Enable V2 scheduler
SCHEDULER_V2_ENABLED=true

# Worker pool
SCHEDULER_WORKER_POOL_SIZE=10
SCHEDULER_JOB_TIMEOUT=3600

# Queue
SCHEDULER_QUEUE_BATCH_SIZE=50
SCHEDULER_QUEUE_PREFIX=crawler

# Leader election
SCHEDULER_LEADER_TTL=30s
SCHEDULER_LEADER_KEY=scheduler-leader

# Circuit breaker
SCHEDULER_CB_THRESHOLD=5
SCHEDULER_CB_TIMEOUT=60s
```

## API Endpoints

### Job Management

```
POST   /api/v2/jobs                    # Create job (extended schema)
GET    /api/v2/jobs                    # List jobs (with priority filter)
GET    /api/v2/jobs/:id                # Get job
PUT    /api/v2/jobs/:id                # Update job
DELETE /api/v2/jobs/:id                # Delete job

POST   /api/v2/jobs/:id/pause          # Pause job
POST   /api/v2/jobs/:id/resume         # Resume job
POST   /api/v2/jobs/:id/cancel         # Cancel job
POST   /api/v2/jobs/:id/force-run      # Force immediate execution
```

### Scheduler Control

```
GET    /api/v2/scheduler/health        # Scheduler health
GET    /api/v2/scheduler/workers       # Worker status
GET    /api/v2/scheduler/metrics       # Extended metrics
POST   /api/v2/scheduler/drain         # Drain workers
POST   /api/v2/scheduler/resume        # Resume workers
```

### Triggers

```
POST   /api/v2/triggers/webhook        # Webhook trigger endpoint
GET    /api/v2/triggers/status         # Trigger system status
GET    /api/v2/triggers/webhooks       # List webhook patterns
GET    /api/v2/triggers/channels       # List Pub/Sub channels
POST   /api/v2/triggers/webhooks       # Register webhook
POST   /api/v2/triggers/channels       # Register channel
```

## Job Schema (V2)

```json
{
  "source_id": "uuid",
  "url": "https://example.com",

  "schedule_type": "cron",
  "cron_expression": "0 */6 * * *",

  "priority": 1,
  "timeout_seconds": 7200,
  "depends_on": ["job-uuid-1"],

  "trigger_webhook": "/sources/*/crawl",
  "trigger_channel": "content:new",

  "schedule_enabled": true
}
```

## Prometheus Metrics

```
# Job metrics
crawler_scheduler_jobs_scheduled_total{schedule_type, priority}
crawler_scheduler_jobs_executed_total{status, source_id}
crawler_scheduler_job_duration_seconds{source_id}
crawler_scheduler_jobs_currently_running

# Worker metrics
crawler_scheduler_worker_pool_size
crawler_scheduler_workers_busy
crawler_scheduler_workers_idle

# Queue metrics
crawler_scheduler_queue_depth{priority}
crawler_scheduler_queue_enqueued_total{priority}
crawler_scheduler_queue_dequeued_total{priority}

# Trigger metrics
crawler_scheduler_triggers_fired_total{type}
crawler_scheduler_triggers_matched_total{type}

# Circuit breaker metrics
crawler_scheduler_circuit_breaker_state{domain}
crawler_scheduler_circuit_breaker_trips_total{domain}

# Leader election metrics
crawler_scheduler_is_leader
crawler_scheduler_leader_election_attempts_total
```

## Migration from V1

### 1. Enable Shadow Mode

Both V1 and V2 schedulers run simultaneously:
- V1 processes jobs with `scheduler_version = 1`
- V2 processes jobs with `scheduler_version = 2`

### 2. Create V2 Jobs

New jobs created via V2 API automatically use V2 scheduler:
```bash
curl -X POST http://localhost:8060/api/v2/jobs \
  -d '{"source_id":"uuid","url":"https://example.com","schedule_type":"cron","cron_expression":"0 */6 * * *"}'
```

### 3. Migrate Existing Jobs

Update existing jobs to V2:
```sql
UPDATE jobs SET scheduler_version = 2 WHERE id = 'job-uuid';
```

### 4. Monitor Both Systems

Compare metrics between V1 and V2 to ensure parity.

### 5. Complete Migration

Once all jobs are on V2 and stable, run cleanup migration.

## Troubleshooting

### Jobs Not Executing

1. Check scheduler health:
   ```bash
   curl http://localhost:8060/api/v2/scheduler/health
   ```

2. Check worker status:
   ```bash
   curl http://localhost:8060/api/v2/scheduler/workers
   ```

3. Check Redis Streams:
   ```bash
   redis-cli XINFO GROUPS crawler:jobs:normal
   ```

### High Queue Depth

1. Check worker pool size
2. Check for stuck jobs (circuit breaker open?)
3. Increase worker pool size if needed

### Leader Election Issues

1. Check Redis connectivity
2. Verify leader TTL configuration
3. Check for network partitions

## Package Structure

```
internal/scheduler/v2/
├── scheduler.go           # Main orchestrator
├── config.go              # Configuration
│
├── schedule/              # Scheduling strategies
│   ├── cron.go            # go-quartz cron scheduling
│   ├── interval.go        # Interval scheduling
│   └── event.go           # Event matching
│
├── triggers/              # Event triggers
│   ├── webhook.go         # HTTP webhook handler
│   ├── pubsub.go          # Redis Pub/Sub listener
│   └── router.go          # Trigger routing
│
├── observability/         # Metrics, tracing
│   ├── metrics.go         # Prometheus metrics
│   ├── tracing.go         # OpenTelemetry setup
│   └── logging.go         # Structured logging
│
└── domain/                # Extended models
    └── job.go             # JobV2 with cron, priority, etc.
```

## Dependencies

- `github.com/reugn/go-quartz` - Cron scheduling
- `github.com/redis/go-redis/v9` - Redis client
- `github.com/prometheus/client_golang` - Metrics
- `go.opentelemetry.io/otel` - Tracing
