# Phase 2 Integration Test Results

## Date: 2026-01-30

## Test: Full Job Lifecycle via Events

### Prerequisites

1. Redis running:
```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d redis
```

2. PostgreSQL running with migrations applied:
```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d postgres-crawler
cd crawler && go run cmd/migrate/main.go up
```

3. source-manager running:
```bash
cd source-manager && go run main.go
```

4. crawler running with events enabled:
```bash
export REDIS_EVENTS_ENABLED=true
export REDIS_ADDRESS=localhost:6379
export SOURCE_MANAGER_URL=http://localhost:8050
cd crawler && go run main.go
```

---

### Test 1: Source Created → Job Created

1. Create a source:
```bash
curl -X POST http://localhost:8050/api/v1/sources \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Phase 2 Test Source",
    "url": "https://example.com",
    "rate_limit": 10,
    "max_depth": 2,
    "enabled": true,
    "priority": "high"
  }'
```

2. Verify job created in crawler:
```bash
curl http://localhost:8060/api/v1/jobs | jq '.[] | select(.source_name == "Phase 2 Test Source")'
```

**Expected:**
- [ ] Job exists with `auto_managed=true`
- [ ] Interval computed based on priority (high=30 min base)
- [ ] Status is `"pending"`
- [ ] `schedule_enabled=true`

---

### Test 2: Source Updated → Job Rescheduled

1. Update the source (change rate_limit):
```bash
curl -X PUT http://localhost:8050/api/v1/sources/{id} \
  -H "Content-Type: application/json" \
  -d '{
    "rate_limit": 5
  }'
```

2. Verify job interval changed:
```bash
curl http://localhost:8060/api/v1/jobs/{job_id}
```

**Expected:**
- [ ] Interval increased (low rate limit = +50% = 45 min)
- [ ] Priority unchanged

---

### Test 3: Source Disabled → Job Paused

1. Disable the source:
```bash
curl -X POST http://localhost:8050/api/v1/sources/{id}/disable
```

2. Verify job paused:
```bash
curl http://localhost:8060/api/v1/jobs/{job_id}
```

**Expected:**
- [ ] Status is `"paused"`

---

### Test 4: Source Enabled → Job Resumed

1. Enable the source:
```bash
curl -X POST http://localhost:8050/api/v1/sources/{id}/enable
```

2. Verify job resumed:
```bash
curl http://localhost:8060/api/v1/jobs/{job_id}
```

**Expected:**
- [ ] Status is `"pending"`
- [ ] `next_run_at` is near current time

---

### Test 5: Source Deleted → Job Deleted

1. Delete the source:
```bash
curl -X DELETE http://localhost:8050/api/v1/sources/{id}
```

2. Verify job deleted:
```bash
curl http://localhost:8060/api/v1/jobs | jq '.[] | select(.source_name == "Phase 2 Test Source")'
```

**Expected:**
- [ ] Job no longer exists

---

### Test 6: Idempotency

1. Note event count in processed_events table:
```bash
docker exec -it north-cloud-postgres-crawler psql -U postgres -d crawler \
  -c "SELECT COUNT(*) FROM processed_events;"
```

2. Restart crawler

3. Wait for event replay (consumer resumes from last checkpoint)

4. Verify no duplicate jobs created:
```bash
curl http://localhost:8060/api/v1/jobs | jq 'length'
```

**Expected:**
- [ ] Same number of jobs before and after restart
- [ ] No duplicate event processing

---

### Test 7: Disabled Source Creation → No Job

1. Create a disabled source:
```bash
curl -X POST http://localhost:8050/api/v1/sources \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Disabled Test Source",
    "url": "https://disabled.example.com",
    "enabled": false
  }'
```

2. Verify no job created:
```bash
curl http://localhost:8060/api/v1/jobs | jq '.[] | select(.source_name == "Disabled Test Source")'
```

**Expected:**
- [ ] No job exists for disabled source
- [ ] Event recorded in processed_events (idempotent)

---

## Verification Commands

```bash
# Check Redis stream length
docker exec -it north-cloud-redis redis-cli XLEN source-events

# View all events in stream
docker exec -it north-cloud-redis redis-cli XRANGE source-events - +

# Check consumer group info
docker exec -it north-cloud-redis redis-cli XINFO GROUPS source-events

# Check processed events count
docker exec -it north-cloud-postgres-crawler psql -U postgres -d crawler \
  -c "SELECT COUNT(*) FROM processed_events;"

# List auto-managed jobs
docker exec -it north-cloud-postgres-crawler psql -U postgres -d crawler \
  -c "SELECT id, source_id, url, status, auto_managed, interval_minutes, priority FROM jobs WHERE auto_managed = true;"
```

---

## Schedule Computation Reference

| Priority | Base Interval |
|----------|---------------|
| critical | 15 minutes    |
| high     | 30 minutes    |
| normal   | 60 minutes    |
| low      | 180 minutes   |

**Rate Limit Adjustments:**
| Rate Limit | Adjustment |
|------------|------------|
| ≤5         | +50%       |
| 6-10       | base       |
| 11-20      | -25%       |
| >20        | -50%       |

**Depth Adjustments:**
| Max Depth | Adjustment |
|-----------|------------|
| ≤2        | base       |
| 3-5       | +25%       |
| >5        | +50%       |

---

## Feature Flag Behavior

| REDIS_EVENTS_ENABLED | Behavior |
|---------------------|----------|
| `false` (default)   | Events disabled, no Redis connection attempted, NoOpHandler used |
| `true`              | Events enabled, EventService handles job lifecycle |

## Graceful Degradation

Both services handle Redis unavailability gracefully:
- **source-manager**: Logs warning, continues without publishing events
- **crawler**: Logs warning, continues without consuming events (uses NoOpHandler fallback)

This ensures existing functionality is unaffected when Redis is down.

---

## Notes

Phase 2 implements the full event-driven job lifecycle:
- EventService replaces NoOpHandler
- Jobs are automatically created/updated/deleted based on source events
- ScheduleComputer dynamically determines crawl intervals
- Idempotency ensures at-least-once delivery works correctly

Phase 3 will migrate existing sources to auto-managed jobs and deprecate manual job creation.
