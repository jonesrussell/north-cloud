# Real-Time Logging Architecture: Redis Streams

**Date:** 2026-01-31
**Status:** Approved
**Author:** Claude + Jones

## Problem Statement

The current SSE-based job log streaming is unreliable. Logs don't appear during job execution, only after completion. The root cause is tight coupling between in-memory buffers and the SSE broker—if logs aren't captured at the right moment, they're lost.

## Constraints

- Logs must stream live to the dashboard
- Logs must be replayable for late-joiners
- Logs must persist after job completion
- Architecture must survive service restarts
- Must work across microservices (crawler, classifier, publisher)
- Must be implementable in 1–2 days

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         PROPOSED ARCHITECTURE                            │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────┐    ┌──────────┐    ┌───────────┐                          │
│  │ Crawler  │    │Classifier│    │ Publisher │                          │
│  └────┬─────┘    └────┬─────┘    └─────┬─────┘                          │
│       │               │                │                                 │
│       └───────────────┼────────────────┘                                │
│                       ▼                                                  │
│            ┌─────────────────────┐                                      │
│            │   Redis Streams     │  ← Single source of truth            │
│            │  logs:{job_id}      │  ← TTL: 24 hours                     │
│            └──────────┬──────────┘                                      │
│                       │                                                  │
│         ┌─────────────┼─────────────┐                                   │
│         ▼             ▼             ▼                                   │
│  ┌────────────┐ ┌───────────┐ ┌───────────┐                            │
│  │ SSE Gateway│ │ Archiver  │ │ Dashboard │                            │
│  │ (thin)     │ │ (async)   │ │ (polling) │                            │
│  └─────┬──────┘ └─────┬─────┘ └───────────┘                            │
│        │              │                                                  │
│        ▼              ▼                                                  │
│  ┌──────────┐   ┌──────────┐                                           │
│  │ Browser  │   │  MinIO   │                                           │
│  │  (SSE)   │   │ (archive)│                                           │
│  └──────────┘   └──────────┘                                           │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

**Key Change:** Replace in-memory buffers with Redis Streams. Redis becomes the single source of truth for live logs. Services write, consumers read. Replay is native to Redis Streams via `XREAD` with message IDs.

## Component Descriptions

### 1. Log Producer (Crawler, Classifier, Publisher)

- Each service writes logs to Redis Stream: `XADD logs:{job_id} * level info category lifecycle message "Starting job"`
- Stream key format: `logs:{job_id}` (one stream per job)
- TTL set on first write: 24 hours (auto-cleanup)
- Fields: `timestamp`, `level`, `category`, `message`, `fields` (JSON)
- No buffering, no callbacks—just a direct Redis write

### 2. Redis Streams

- Stores log entries as stream messages with auto-generated IDs (timestamp-based)
- Native replay: `XREAD` from any message ID returns all subsequent entries
- Native blocking: `XREAD BLOCK 0` waits for new entries (no polling)
- Memory-efficient: ~100 bytes per log line, 10K logs = ~1MB per job
- Survives service restarts—logs persist in Redis

### 3. SSE Gateway (thin adapter in crawler service)

- Endpoint: `GET /api/v1/jobs/:id/logs/stream`
- On connect: `XREAD` from ID `0` (all logs) or client-provided `lastEventId`
- Blocking read loop: `XREAD BLOCK 5000` for new entries
- Translates Redis messages → SSE events
- Stateless—no in-memory buffer needed

### 4. Archiver (background worker)

- Listens for job completion events
- Reads entire stream: `XRANGE logs:{job_id} - +`
- Compresses and uploads to MinIO
- Updates PostgreSQL with object key
- Deletes stream: `DEL logs:{job_id}` (optional, TTL handles it anyway)

### 5. Dashboard

- Connects via SSE with `Last-Event-ID` header for resume
- Falls back to polling if SSE unavailable
- For completed jobs: fetches from MinIO archive

## Approach Comparison

| Criteria | SSE (Current) | WebSockets | Redis Streams + SSE |
|----------|---------------|------------|---------------------|
| **Complexity** | Medium - SSE broker, in-memory buffer, archiver all tightly coupled | High - connection management, heartbeats, reconnect logic | Low - Redis handles storage/replay, SSE is thin adapter |
| **Restart Survival** | No - Buffer lost | No - Buffer lost | Yes - Redis persists |
| **Replay** | Custom buffer code | Custom buffer code | Native `XREAD` from ID |
| **Late Joiners** | Buffer replay (limited) | Buffer replay (limited) | Full history from Redis |
| **Multi-Service** | No - Each service needs SSE broker | No - Each service needs WS server | Yes - All services write to Redis |
| **Browser Support** | Universal | Universal | Universal (via SSE) |
| **Scaling** | No - Sticky sessions needed | No - Sticky sessions needed | Yes - Stateless gateways |
| **Existing Infra** | N/A | New dependency | Redis already in stack |
| **Implementation** | N/A | 3-4 days | 1-2 days |
| **Debugging** | Hard - ephemeral buffer | Hard - ephemeral buffer | Easy - `redis-cli XRANGE` |

## Recommendation

**Recommended: Redis Streams + SSE Gateway**

### Why this approach wins

1. **Eliminates the root cause.** The current bug exists because in-memory buffers are tightly coupled to the SSE broker. If the broker doesn't receive logs (timing, connection issue, race condition), they're lost. Redis Streams decouples writing from reading entirely.

2. **Already in your stack.** Redis is running for publisher pub/sub. No new infrastructure. Redis Streams is just a different data structure in the same Redis instance.

3. **Native replay solves late-joiners.** No custom ring buffer code. `XREAD STREAMS logs:{job_id} 0` returns everything. `XREAD STREAMS logs:{job_id} 1706745600000-0` resumes from that ID. The protocol handles it.

4. **Survives restarts by default.** Crawler crashes mid-job? Logs are in Redis. Dashboard reconnects? Picks up from `Last-Event-ID`. Archiver runs later? Full history available.

5. **Cross-service logging is trivial.** Classifier and publisher can write to the same stream. Just `XADD logs:{job_id}`. No SSE broker coordination needed.

6. **Debuggable in production.** `redis-cli XRANGE logs:{job_id} - +` shows exactly what's in the stream. No guessing about ephemeral buffer state.

### What we're keeping

- SSE as browser transport (works well, no changes to frontend event handling)
- MinIO for archival (already works)
- PostgreSQL for metadata (already works)

### What we're replacing

- In-memory buffer → Redis Streams
- SSE broker log capture → Direct Redis writes
- Custom replay logic → Native `XREAD`

## Migration Plan

### Phase 1: Add Redis Streams Writer (Day 1 morning)

**Files to modify:**
- `crawler/internal/logs/redis_writer.go` (new)
- `crawler/internal/logs/job_logger_impl.go` (add Redis write alongside buffer)

**Tasks:**
- Create `RedisStreamWriter` that calls `XADD`
- Dual-write: existing buffer AND Redis Stream
- Feature flag: `JOB_LOGS_REDIS_ENABLED=true`
- No breaking changes—old SSE path still works

### Phase 2: New SSE Gateway (Day 1 afternoon)

**Files to modify:**
- `crawler/internal/api/logs_stream_handler.go` (new)
- `crawler/internal/api/routes.go` (add new endpoint)

**Tasks:**
- New endpoint: `GET /api/v1/jobs/:id/logs/stream/v2`
- Reads from Redis Stream with `XREAD BLOCK`
- Translates to existing SSE event format
- Dashboard can switch endpoints without code changes

### Phase 3: Dashboard Switch (Day 2 morning)

**Files to modify:**
- `dashboard/src/features/intake/components/JobLogs.vue`
- `dashboard/src/features/intake/api/jobs.ts`

**Tasks:**
- Point SSE connection to `/stream/v2`
- Pass `Last-Event-ID` for resume (already supported by EventSource)
- Test live streaming, reconnection, late-join replay

### Phase 4: Cleanup (Day 2 afternoon)

**Tasks:**
- Remove old SSE broker log capture
- Remove in-memory buffer code
- Remove `/stream` endpoint (or redirect to v2)
- Update archiver to read from Redis instead of buffer

### Rollback Plan

- Feature flag `JOB_LOGS_REDIS_ENABLED=false` reverts to old path
- Old endpoint remains until v2 is proven stable
- No database migrations required

## Redis Stream Schema

```
Key: logs:{job_id}
TTL: 86400 seconds (24 hours)

Entry fields:
  timestamp  - RFC3339 timestamp
  level      - debug | info | warn | error
  category   - crawler.lifecycle | crawler.fetch | crawler.extract | etc.
  message    - Human-readable log message
  fields     - JSON-encoded structured data (optional)
  exec_id    - Execution ID (for multi-execution jobs)
  service    - crawler | classifier | publisher
```

Example:
```
XADD logs:f89c37e0-8ca1-43a4-b893-9338fe72489d * \
  timestamp "2026-01-31T12:03:30Z" \
  level "info" \
  category "crawler.lifecycle" \
  message "Starting job execution" \
  service "crawler" \
  exec_id "exec-001"
```

## SSE Event Format (unchanged)

```typescript
// log:line event (same as current)
{
  event: "log:line",
  id: "1706745600000-0",  // Redis stream ID as SSE event ID
  data: {
    job_id: "f89c37e0-8ca1-43a4-b893-9338fe72489d",
    execution_id: "exec-001",
    level: "info",
    category: "crawler.lifecycle",
    message: "Starting job execution",
    timestamp: "2026-01-31T12:03:30Z"
  }
}
```

## Success Criteria

1. Logs appear in dashboard within 1 second of generation
2. Late-joining clients see full log history
3. Logs survive crawler restart mid-job
4. `redis-cli XRANGE logs:{job_id} - +` shows live logs
5. Archived logs in MinIO match Redis stream contents
