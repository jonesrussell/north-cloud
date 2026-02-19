# Click Tracking Design for NorthCloud Search

**Date**: 2026-02-19
**Status**: Approved

## Overview

Privacy-respectful click tracking system for NorthCloud Search. Server-side 302 redirect with HMAC-signed URLs, anonymous session-level analytics, and 30-day data retention.

**Key decisions**:
- Privacy tier: Anonymous session-level (no user profiles, no PII)
- Mechanism: Server-side 302 redirect
- Storage: Dedicated PostgreSQL database with monthly partitions
- Deployment: New standalone `click-tracker` Go microservice
- URL format: Signed query parameters (HMAC-SHA256)

---

## Section 1: Industry Patterns

Three tiers exist in production:

**Google (maximum signal)**: Uses `onmousedown` JS to rewrite link `href` to a tracking URL containing a protobuf-encoded `ved` token (position, element type, SERP timestamp). Fires HTTP 204 to log the click asynchronously. Feeds into Navboost, a re-ranking system using 13-month rolling windows with differential privacy. "Voters and votes" architecture anonymizes at the system level.

**Bing (moderate signal)**: Server-side redirect through `/ck/a` with HMAC-hashed parameters. Persistent MUID cookie across Microsoft properties. 10-minute `anonchk` cookie for click verification. MSCLKID for ad attribution (90-day lifetime).

**DuckDuckGo (privacy-first)**: No individual click tracking. Uses `r.duckduckgo.com` redirect solely to strip HTTP Referer headers (protecting users from leaking query terms to destination sites). Falls back to `<meta name="referrer" content="no-referrer">` on modern browsers.

**Takeaways for NorthCloud**:
- Server-side redirect is proven at Bing scale
- HMAC signing is standard for tamper prevention
- Referrer stripping (`rel="noopener noreferrer"`) is already in place on our frontend
- Short retention windows (30 days) and anonymous session IDs align with DuckDuckGo's privacy philosophy while enabling Bing-level ranking signals

---

## Section 2: What NorthCloud Should Track

### Click Event Fields

| Field | Example | Purpose |
|-------|---------|---------|
| `query_id` | `q_a7f3b2` | Links click back to the search query. Generated per-search, short-lived. |
| `result_id` | `r_doc456` | Elasticsearch document `_id`. Enables per-document CTR. |
| `position` | `3` | Rank position (1-indexed). Critical for position bias analysis. |
| `page` | `1` | Results page number. Position 3 on page 2 differs from page 1. |
| `timestamp` | `1708300000` | Unix epoch seconds. SERP render time (when click URL was generated). |
| `clicked_at` | `2026-02-19T14:30:00Z` | Server-side timestamp when redirect is processed. Delta = time-to-click. |
| `user_agent_hash` | `sha256[:12]` | Truncated SHA-256 of User-Agent. Bot detection without fingerprinting. |
| `session_id` | `s_8f2a1b` | Anonymous session cookie (first-party, 30-min TTL). Click-chain analysis. |
| `destination_hash` | SHA-256 | Hash of destination URL. No raw URLs in analytics tables. |

### What We Do NOT Track

- IP addresses (not stored, not hashed)
- User identity (no login required for search)
- Cross-session tracking (session cookie expires in 30 minutes)
- Browser fingerprinting (only hashed UA string)
- Query terms in click logs (query_id references query, not text)

### Derived Signals (Computed, Not Directly Tracked)

- **CTR by position**: clicks / impressions per rank position
- **Pogo-sticking**: Same session clicks result N, then different result within 30s
- **Skip rate**: Results shown but never clicked
- **Abandonment**: Searches with zero clicks

---

## Section 3: URL Format

### Click URL Format

```
/api/click?q={query_id}&r={result_id}&p={position}&pg={page}&t={timestamp}&u={encoded_destination}&sig={signature}
```

Example:
```
https://northcloud.biz/api/click?q=q_a7f3b2&r=r_doc456&p=3&pg=1&t=1708300000&u=https%3A%2F%2Fsudbury.com%2Farticle&sig=e4a2f7b91c3d
```

### Search API Response Change

```json
{
  "url": "https://sudbury.com/article-123",
  "click_url": "https://northcloud.biz/api/click?q=q_a7f3b2&r=r_doc456&p=3&pg=1&t=1708300000&u=https%3A%2F%2Fsudbury.com%2Farticle-123&sig=e4a2f7b91c3d"
}
```

The search service generates `click_url` for each hit. Frontend uses `click_url` for `<a href>` but displays `url` as visible text. Raw `url` remains available for direct-link/no-tracking mode.

### Signature Scheme

```
signature = HMAC-SHA256(
    key = CLICK_TRACKER_SECRET,
    message = "q_a7f3b2|r_doc456|3|1|1708300000|https://sudbury.com/article-123"
)[:12]  // truncated to 12 hex chars (48 bits)
```

- **Truncation to 12 hex chars**: 48 bits sufficient for tamper detection (2^48 brute-force attempts per URL)
- **Timestamp window**: Reject clicks where `now - t > 24 hours`
- **No nonce needed**: `query_id + result_id + position + timestamp` is already unique
- **Destination included in signature**: Prevents destination URL tampering

### Alternatives Considered

| Approach | Latency | Debuggability | Complexity | Verdict |
|----------|---------|---------------|------------|---------|
| **Signed query params** | ~10-50ms | Excellent | Low | **Chosen** |
| Encrypted opaque token | ~10-50ms | None | Medium | Overkill for non-PII |
| Pre-generated click IDs | ~10-50ms + SERP overhead | Poor | High | Doesn't scale |
| `sendBeacon` (client-side) | 0ms | Good | Medium | Misses right-click, no-JS |
| `ping` attribute | 0ms | Good | Low | Not all browsers |

---

## Section 4: Backend Architecture

### Data Flow

```
search-service → generates click_urls per hit
       ↓
search-frontend → renders <a href="click_url">
       ↓
User clicks → browser navigates to /api/click?...
       ↓
nginx → proxies to click-tracker:8093
       ↓
click-tracker:
  1. Parse query params
  2. Validate HMAC signature
  3. Check timestamp window (reject if > 24h)
  4. Async-insert click event (buffered)
  5. Return 302 Found → Location: destination_url
       ↓
PostgreSQL (click_events table, monthly partitions)
+ stdout JSON logs → Loki
```

### Click-Tracker Service

New standalone Go service at `/click-tracker`, port 8093.

**Endpoints**:
- `GET /click` — Validate, log, 302 redirect (public)
- `GET /health` — Health check (public)
- `GET /ready` — Readiness check (public)
- `GET /api/v1/stats` — Analytics data (authenticated, future)
- `GET /api/v1/stats/ctr` — CTR by position (authenticated, future)

### Database Schema (`postgres-click-tracker`)

```sql
CREATE TABLE click_events (
    id              BIGSERIAL PRIMARY KEY,
    query_id        VARCHAR(32) NOT NULL,
    result_id       VARCHAR(128) NOT NULL,
    position        SMALLINT NOT NULL,
    page            SMALLINT NOT NULL DEFAULT 1,
    destination_hash VARCHAR(64) NOT NULL,  -- SHA-256 of destination URL
    session_id      VARCHAR(32),
    user_agent_hash VARCHAR(12),
    generated_at    TIMESTAMPTZ NOT NULL,   -- from 't' param (SERP render time)
    clicked_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
) PARTITION BY RANGE (clicked_at);

-- Monthly partitions, auto-drop after 30 days
CREATE TABLE click_events_2026_02 PARTITION OF click_events
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');

CREATE INDEX idx_click_events_query_id ON click_events (query_id);
CREATE INDEX idx_click_events_result_id ON click_events (result_id);
CREATE INDEX idx_click_events_position ON click_events (position);
CREATE INDEX idx_click_events_clicked_at ON click_events (clicked_at);
```

### Write Buffering

Channel-based buffer for sub-10ms redirect latency:
- Buffer capacity: 1000 events
- Flush every 1 second or at 500 events (whichever comes first)
- If buffer full: drop event + log warning (never block the redirect)

### Bot Detection (MVP)

- **Rate limiting**: Max 10 clicks per session_id per minute → 429 response
- **UA blocklist**: Known bot patterns (Googlebot, Bingbot, etc.) → redirect without logging
- **Missing headers**: No Accept or User-Agent → flag as suspicious

### Fraud Prevention

- HMAC signature prevents fabricated click URLs
- 24-hour timestamp window prevents stale URL replay
- Rate limiting prevents click flooding
- No ad revenue = low incentive for click fraud

---

## Section 5: Privacy & Compliance

### Collection Boundaries

| Collected | Not Collected |
|-----------|---------------|
| Anonymous session ID (30-min cookie) | IP addresses |
| Hashed User-Agent (12 chars SHA-256) | User identity / login state |
| Query ID (reference, not query text) | Raw query terms in click logs |
| Result ID + position | Browser fingerprint data |
| Destination URL hash (SHA-256) | Cross-session identifiers |
| Click timestamp | Geolocation |

### Session Cookie

```
Cookie: nc_sid=s_8f2a1b; Path=/api/click; HttpOnly; Secure; SameSite=Strict; Max-Age=1800
```

- First-party only (northcloud.biz)
- HttpOnly (no JS access)
- Secure (HTTPS only)
- SameSite=Strict (no cross-site)
- 30-minute TTL
- Random value (not derived from user data)
- Optional (clicks logged without session_id if missing)

### Query Term Isolation

Click logs contain `query_id`, never query text. The `query_id → query_text` mapping lives only in search service memory and is not persisted in the click-tracker database.

### Retention Policy

- **Click events**: 30-day retention via monthly PostgreSQL partitions. Drop partitions via scheduled task.
- **Session cookies**: 30-minute browser TTL, no server-side session store.
- **Aggregated metrics**: CTR summaries retained indefinitely (fully anonymized).

### Opt-Out / No-Tracking Mode

1. **Direct link fallback**: Frontend preference toggle switches `<a href>` from `click_url` to `url`. Stored in localStorage.
2. **DNT / GPC headers**: If browser sends `DNT: 1` or `Sec-GPC: 1`, search service omits `click_url` from responses.

### GDPR / CCPA

- No consent banner needed for MVP: no PII, no third-party cookies, no user profiles
- Data minimization: every field has documented analytical purpose
- Right to erasure: not applicable (no identifiable data)
- If user accounts are added later: revisit consent requirements

---

## Section 6: Recommended MVP

### In Scope

- `click-tracker` Go service with `/click` redirect + `/health`
- HMAC-signed click URLs generated by search service
- PostgreSQL with monthly partitions and 30-day retention
- Buffered async writes (channel-based, batch insert)
- Bot UA blocklist (redirect without logging)
- Rate limiting (10 clicks/session/minute)
- nginx routing (`/api/click → click-tracker:8093`)
- Frontend switches `<a href>` from `url` to `click_url`
- Direct link fallback (raw `url` always available)
- Structured JSON logging to stdout (Loki)

### Not in MVP

- Analytics dashboard / stats endpoints
- Dwell time / pogo-sticking detection
- A/B experiment bucketing
- DNT/GPC header handling
- Session cookie (MVP logs clicks without session correlation)
- Aggregated metrics materialization

### Service Structure

```
click-tracker/
├── main.go
├── Dockerfile
├── config.yml
├── Taskfile.yml
├── .air.toml
├── internal/
│   ├── bootstrap/
│   ├── config/
│   │   └── config.go
│   ├── domain/
│   │   └── click_event.go
│   ├── handler/
│   │   ├── click.go            # GET /click → validate, log, 302
│   │   └── health.go
│   ├── middleware/
│   │   ├── ratelimit.go
│   │   └── botfilter.go
│   ├── signature/
│   │   └── hmac.go             # HMAC-SHA256 sign/verify
│   ├── storage/
│   │   └── postgres.go         # Buffered batch insert
│   └── service/
│       └── tracker.go
├── cmd/
│   └── migrate/
│       └── main.go
└── migrations/
    └── 001_create_click_events.up.sql
```

### Search Service Changes

- Add `CLICK_TRACKER_SECRET` env var
- Add `click_url` field to `SearchHit` struct
- Generate `query_id` per request (8-char random hex)
- Sign click URLs for each hit in response mapper

### Frontend Changes

```vue
<!-- SearchResultItem.vue -->
<a :href="result.click_url || result.url" target="_blank" rel="noopener noreferrer">
```

### Configuration

```yaml
service:
  port: 8093
  hmac_secret: ${CLICK_TRACKER_SECRET}
  signature_length: 12
  max_timestamp_age: 86400
  buffer_size: 1000
  flush_interval: 1s
  flush_threshold: 500

ratelimit:
  max_clicks_per_minute: 10
  window: 60s

retention:
  days: 30
  partition_interval: monthly
```

---

## Section 7: Future Enhancements

### Phase 2: Session Tracking & Analytics Dashboard
- Session cookie (`nc_sid`, 30-min TTL) for click-chain correlation
- Stats endpoints: CTR by position, most-clicked results
- Dashboard integration: "Click Analytics" tab in Vue dashboard
- DNT/GPC header respect

### Phase 3: Dwell Time & Behavioral Signals
- Pogo-sticking detection (same session, rapid back-click)
- Last longest click tracking
- Skip tracking (served but never clicked)
- Requires lightweight session state store (Redis or in-memory)

### Phase 4: A/B Experiment Framework
- `exp` parameter in click URLs
- Consistent hashing for experiment bucket assignment
- Per-bucket CTR, pogo-stick rate, abandonment comparison

### Phase 5: Advanced Fraud Detection
- ML-based bot detection on click patterns
- Click farm detection via statistical anomalies
- Honeypot results (invisible results only bots would click)

### Phase 6: Ranking Signal Feedback Loop
- CTR → Elasticsearch relevance boosting
- Position-debiased CTR normalization
- Navboost-style re-ranking from 30-day aggregates
- Offline evaluation with historical data
