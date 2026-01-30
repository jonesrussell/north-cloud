# Phase 3 Integration Test Results

## Date: 2026-01-30

## Test: Job Migration to Auto-Managed

### Prerequisites

1. Phase 2 complete and all migrations applied
2. Redis running with source events enabled
3. PostgreSQL running
4. source-manager running
5. crawler running with `REDIS_EVENTS_ENABLED=true`

---

### Test 1: Migration of Job with Valid Source

1. Create a manual job (using deprecated endpoint):
```bash
curl -X POST http://localhost:8060/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "source_id": "{existing-source-uuid}",
    "url": "https://example.com",
    "interval_minutes": 60,
    "schedule_enabled": true
  }'
```

2. Note the deprecation headers in response

3. Run migration:
```bash
curl -X POST http://localhost:8060/api/v1/jobs/migrate?batch_size=10
```

4. Verify job is now auto-managed:
```bash
curl http://localhost:8060/api/v1/jobs/{job_id} | jq '.auto_managed, .migration_status'
```

**Expected:**
- [ ] `auto_managed = true`
- [ ] `migration_status = "migrated"`
- [ ] `interval_minutes` computed from source metadata

---

### Test 2: Migration of Orphaned Job

1. Create a job with non-existent source ID:
```bash
curl -X POST http://localhost:8060/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "source_id": "00000000-0000-0000-0000-000000000000",
    "url": "https://orphan.example.com",
    "interval_minutes": 120
  }'
```

2. Run migration:
```bash
curl -X POST http://localhost:8060/api/v1/jobs/migrate?batch_size=10
```

3. Check migration status:
```bash
curl http://localhost:8060/api/v1/jobs/{job_id} | jq '.migration_status'
```

**Expected:**
- [ ] `migration_status = "orphaned"`
- [ ] `auto_managed` unchanged (still false)

---

### Test 3: Migration Stats

1. Check migration progress:
```bash
curl http://localhost:8060/api/v1/jobs/migration-stats
```

**Expected:**
```json
{
  "migration_status": {
    "pending": N,
    "migrated": N,
    "orphaned": N
  }
}
```

---

### Test 4: Deprecation Headers

1. Create a job via deprecated endpoint:
```bash
curl -v -X POST http://localhost:8060/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{...}'
```

**Expected Headers:**
- [ ] `Deprecation: true`
- [ ] `Sunset: 2026-06-01`
- [ ] `X-Deprecation-Notice: ...`

---

### Test 5: Idempotent Migration

1. Run migration twice:
```bash
curl -X POST http://localhost:8060/api/v1/jobs/migrate
curl -X POST http://localhost:8060/api/v1/jobs/migrate
```

2. Verify already-migrated jobs aren't reprocessed:
```bash
curl http://localhost:8060/api/v1/jobs/migration-stats
```

**Expected:**
- [ ] Second run shows 0 processed (all already migrated)

---

## Verification Commands

```bash
# Check manual jobs remaining
docker exec -it north-cloud-postgres-crawler psql -U postgres -d crawler \
  -c "SELECT COUNT(*) FROM jobs WHERE auto_managed = false OR auto_managed IS NULL;"

# Check migration status distribution
docker exec -it north-cloud-postgres-crawler psql -U postgres -d crawler \
  -c "SELECT COALESCE(migration_status, 'pending') as status, COUNT(*) FROM jobs GROUP BY migration_status;"

# List orphaned jobs
docker exec -it north-cloud-postgres-crawler psql -U postgres -d crawler \
  -c "SELECT id, source_id, url FROM jobs WHERE migration_status = 'orphaned';"
```

---

## Notes

Phase 3 migrates existing manual jobs to auto-managed without data loss:
- Valid sources: Jobs become auto-managed with computed schedules
- Missing sources: Jobs marked "orphaned" for operator review
- Already migrated: Skipped in subsequent runs (idempotent)

Next steps:
1. Review orphaned jobs and either delete or link to valid sources
2. After all jobs migrated, disable `POST /api/v1/jobs` endpoint (Phase 4)
