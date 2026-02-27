# Discovered Domains: Domain-Centric Source Discovery Pipeline

**Date**: 2026-02-26
**Status**: Approved

---

## Problem Statement

The current `/intake/discovered-links` page shows a flat list of external URLs discovered during crawling. Operators must mentally group URLs by domain, assess patterns, and decide whether a domain is worth adding as a source. This is tedious and doesn't scale — with thousands of discovered links, the signal-to-noise ratio is poor.

Operators need a **domain-centric** workflow: evaluate domains (not individual URLs), see quality signals at a glance, and promote promising domains into new source configurations.

## Goals

1. Replace the flat URL list with a domain-grouped primary view
2. Provide domain-level quality scoring from real crawl data (HTTP status, content type)
3. Enable domain lifecycle management: active → reviewing → ignored/promoted
4. Support bulk operations on domains
5. Track which sources discovered each domain for cross-referencing
6. Pre-fill the source creation form when promoting a domain

## Non-Goals

- Automated source recommendation (future — scoring model enables this later)
- Real-time probing of URLs (quality data comes from crawl-time tracking)
- Replacing the URL frontier (this is a separate operational system)
- Feed discovery from discovered domains (future enhancement)
- Domain reputation scoring from external services

## User Roles & Workflows

**Primary user**: Operator (authenticated dashboard user)

**Core workflow**:
1. Operator opens Discovered Domains page
2. Scans domain list sorted by quality score (highest first)
3. Sees domain "example.com" with score 82, 47 links from 3 sources, 95% HTML
4. Clicks to expand — sees path clusters (`/article/*: 32 links`, `/news/*: 12 links`)
5. Decides this is a good source candidate
6. Clicks "Create Source" — redirected to `/sources/new` pre-filled with domain info
7. Alternatively, marks low-quality domains as "ignored" (individually or in bulk)

**Triage workflow**:
1. Filter by "Active" status, sort by "Last Seen" descending
2. Review newly discovered domains
3. Bulk-select junk domains → "Ignore Selected"
4. Mark interesting domains as "Reviewing"
5. Come back later to promote or ignore reviewed domains

---

## Data Model Changes

### Migration: Extend `discovered_links` table

Add three columns to the existing `discovered_links` table:

```sql
ALTER TABLE discovered_links
  ADD COLUMN domain VARCHAR(255),
  ADD COLUMN http_status SMALLINT,
  ADD COLUMN content_type VARCHAR(100);

-- Backfill domain from existing URLs
UPDATE discovered_links
SET domain = substring(url FROM '://([^/]+)');

-- Make domain NOT NULL after backfill
ALTER TABLE discovered_links
  ALTER COLUMN domain SET NOT NULL;

-- Index for GROUP BY aggregation
CREATE INDEX idx_discovered_links_domain ON discovered_links(domain);

-- Composite index for domain + status queries
CREATE INDEX idx_discovered_links_domain_status ON discovered_links(domain, status);
```

| Column | Type | Purpose |
|--------|------|---------|
| `domain` | `VARCHAR(255) NOT NULL` | Extracted hostname from URL, indexed for GROUP BY |
| `http_status` | `SMALLINT` | HTTP response code (200, 404, etc.). NULL if not yet observed |
| `content_type` | `VARCHAR(100)` | Response content-type. NULL if not yet observed |

### New table: `discovered_domain_states`

Stores per-domain operator decisions. Only rows that have been explicitly acted on exist (no row = "active" default state).

```sql
CREATE TABLE discovered_domain_states (
    domain VARCHAR(255) PRIMARY KEY,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    notes TEXT,
    ignored_at TIMESTAMP WITH TIME ZONE,
    ignored_by VARCHAR(100),
    promoted_at TIMESTAMP WITH TIME ZONE,
    promoted_source_id VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TRIGGER update_discovered_domain_states_updated_at
    BEFORE UPDATE ON discovered_domain_states
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

| Column | Type | Purpose |
|--------|------|---------|
| `domain` | `VARCHAR(255) PK` | The domain hostname |
| `status` | `VARCHAR(20)` | `active`, `ignored`, `reviewing`, `promoted` |
| `notes` | `TEXT` | Operator notes |
| `ignored_at` | `TIMESTAMP` | When marked as ignored |
| `ignored_by` | `VARCHAR(100)` | Who ignored it |
| `promoted_at` | `TIMESTAMP` | When promoted to source |
| `promoted_source_id` | `VARCHAR(255)` | Links to created source if promoted |

---

## Quality Score Model

Computed at query time from aggregated `discovered_links` data per domain.

### Input signals

| Signal | Source | Weight |
|--------|--------|--------|
| `ok_ratio` | % of links with `http_status` 2xx (where NOT NULL) | 0.3 |
| `html_ratio` | % of links with `content_type LIKE 'text/html%'` (where NOT NULL) | 0.3 |
| `source_count_norm` | `MIN(distinct_source_count / 5, 1.0)` — normalized, caps at 5 sources | 0.2 |
| `recency_norm` | `1.0 - MIN(days_since_last_seen / 30, 1.0)` — decays over 30 days | 0.2 |

### Formula

```
quality_score = ROUND(
  (ok_ratio * 30) +
  (html_ratio * 30) +
  (source_count_norm * 20) +
  (recency_norm * 20)
)
```

Result: integer 0-100. Domains with no HTTP data yet get a partial score from source_count and recency only (max 40).

### Score display

| Range | Color | Label |
|-------|-------|-------|
| 70-100 | Green | High quality |
| 40-69 | Yellow/Amber | Medium quality |
| 0-39 | Red | Low quality |

---

## API Changes

All endpoints on the **crawler service** under `/api/v1/`.

### New endpoints

#### `GET /api/v1/discovered-domains`

List domains with aggregated stats, quality scores, and operator state.

**Query parameters**:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `limit` | int | 25 | Page size |
| `offset` | int | 0 | Pagination offset |
| `sort` | string | `quality_score` | Sort field: `quality_score`, `link_count`, `last_seen`, `source_count`, `domain` |
| `order` | string | `desc` | `asc` or `desc` |
| `status` | string | | Filter by domain state: `active`, `ignored`, `reviewing`, `promoted` |
| `search` | string | | Search domain name (ILIKE) |
| `min_score` | int | | Minimum quality score filter |
| `hide_existing` | bool | false | Exclude domains that already exist as sources |

**Response**:

```json
{
  "domains": [
    {
      "domain": "example.com",
      "status": "active",
      "link_count": 47,
      "source_count": 3,
      "referring_sources": ["CBC News", "Toronto Star", "Globe and Mail"],
      "ok_ratio": 0.89,
      "html_ratio": 0.95,
      "avg_depth": 1.4,
      "first_seen": "2026-01-15T10:30:00Z",
      "last_seen": "2026-02-25T14:20:00Z",
      "quality_score": 82,
      "is_existing_source": false,
      "notes": null
    }
  ],
  "total": 156
}
```

**Backend implementation**: SQL aggregation query:

```sql
SELECT
  dl.domain,
  COALESCE(ds.status, 'active') AS status,
  COUNT(*) AS link_count,
  COUNT(DISTINCT dl.source_id) AS source_count,
  AVG(dl.depth) AS avg_depth,
  MIN(dl.discovered_at) AS first_seen,
  MAX(dl.discovered_at) AS last_seen,
  COUNT(CASE WHEN dl.http_status BETWEEN 200 AND 299 THEN 1 END)::float
    / NULLIF(COUNT(dl.http_status), 0) AS ok_ratio,
  COUNT(CASE WHEN dl.content_type LIKE 'text/html%' THEN 1 END)::float
    / NULLIF(COUNT(dl.content_type), 0) AS html_ratio,
  ds.notes
FROM discovered_links dl
LEFT JOIN discovered_domain_states ds ON dl.domain = ds.domain
GROUP BY dl.domain, ds.status, ds.notes
HAVING 1=1  -- filters applied here
ORDER BY quality_score DESC
LIMIT $1 OFFSET $2
```

Quality score and `is_existing_source` computed in Go after the SQL query (score from the ratios, existing-source check via source-manager API call with domain list).

#### `GET /api/v1/discovered-domains/:domain`

Single domain detail with full stats. Same shape as a single item from the list response.

#### `GET /api/v1/discovered-domains/:domain/links`

Paginated URLs within a domain, with path clustering.

**Query parameters**: `limit`, `offset`, `sort`, `order`

**Response**:

```json
{
  "links": [
    {
      "id": "uuid",
      "url": "https://example.com/article/123",
      "path": "/article/123",
      "http_status": 200,
      "content_type": "text/html",
      "depth": 1,
      "source_id": "source-uuid",
      "source_name": "CBC News",
      "discovered_at": "2026-02-20T08:15:00Z",
      "status": "pending"
    }
  ],
  "path_clusters": [
    { "pattern": "/article/*", "count": 32 },
    { "pattern": "/news/*", "count": 12 },
    { "pattern": "/", "count": 3 }
  ],
  "total": 47
}
```

**Path clustering**: Computed in Go by extracting first path segment from each URL's path, grouping, and counting. Pattern: `/{first_segment}/*` for paths with 2+ segments, `/` for root.

#### `PATCH /api/v1/discovered-domains/:domain/state`

Update domain operator state.

**Request body**:

```json
{
  "status": "ignored",
  "notes": "Low-quality aggregator site"
}
```

**Behavior**: Upserts into `discovered_domain_states`. If status is `ignored`, sets `ignored_at` to now. If status is `promoted`, sets `promoted_at` to now.

#### `POST /api/v1/discovered-domains/bulk-state`

Bulk update domain states.

**Request body**:

```json
{
  "domains": ["spam.com", "junk.net", "low-quality.org"],
  "status": "ignored",
  "notes": "Bulk ignored — irrelevant domains"
}
```

### Existing endpoints (unchanged)

All current `/api/v1/discovered-links/*` endpoints remain for backward compatibility and individual link operations (delete link, create job from link).

---

## UI/UX Structure

### Route changes

| Route | View Component | Purpose |
|-------|---------------|---------|
| `/intake/discovered-links` | `DiscoveredDomainsView.vue` | Primary domain list (replaces current) |
| `/intake/discovered-links/:domain` | `DiscoveredDomainDetailView.vue` | Domain drill-down |

Navigation label changes from "Discovered Links" to "Discovered Domains" in the sidebar.

### Primary View: Discovered Domains List

**Header section**:
- Title: "Discovered Domains"
- Subtitle: "External domains found during crawling — evaluate and promote to sources."
- Refresh button (top right)

**Filter bar**:
- Search input (matches domain name, debounced)
- Status dropdown: All | Active | Reviewing | Ignored | Promoted
- Minimum score input (number, 0-100)
- "Hide existing sources" toggle checkbox

**Table**:

| Column | Sortable | Rendering |
|--------|----------|-----------|
| Checkbox | no | Bulk selection |
| Domain | yes | Domain name. If `is_existing_source`, show small "Source" badge next to name |
| Score | yes | Colored badge (green/yellow/red) with number |
| Links | yes | Integer count |
| Sources | yes | Integer count, tooltip shows referring source names |
| HTML % | no | Percentage text or small inline bar |
| Last Seen | yes | Relative time (e.g., "2 hours ago") |
| Status | no | Badge: active (neutral), reviewing (blue), ignored (gray), promoted (green) |
| Actions | no | Dropdown: View Details, Ignore, Mark Reviewing, Create Source |

**Bulk action bar** (appears when items selected):
- "X domains selected"
- "Ignore Selected" button
- "Mark Reviewing" button
- "Clear Selection" link

**Default sort**: Quality score descending.

**Empty states**:
- No discovered domains yet: "No external domains have been discovered. Domains appear here as the crawler finds links to external sites."
- No filter matches: "No domains match your filters."

### Secondary View: Domain Detail

**Header**:
- Back link: "← Discovered Domains"
- Domain name (large)
- Quality score badge + status badge
- Action buttons: "Create Source" (primary), "Ignore" / "Mark Reviewing" (secondary)

**Summary cards** (row of 5):
- Total Links | OK Rate | HTML Rate | Avg Depth | Discovered From

**Path Clusters section**:
- Title: "URL Patterns"
- Simple table: Pattern | Count
- Sorted by count descending

**URLs table** (paginated):
- Columns: Path (truncated URL without domain), HTTP Status (badge: green 2xx, red 4xx/5xx), Content Type, Depth, Source, Discovered
- Row actions: Create Job, Delete

**"Create Source" behavior**:
- Navigates to `/sources/new?domain=example.com&name=Example+Com`
- Source form reads query params and pre-fills domain as `allowed_domains[0]`, name from domain
- After source creation, operator can return and mark domain as "promoted"

---

## Crawler Changes

### Link handler modification

In `crawler/internal/crawler/link_handler.go`, when `HandleLink` processes a discovered external link:

1. Extract domain from URL using `net/url` (already done for external link detection)
2. Pass domain to the `CreateOrUpdate` call

The `http_status` and `content_type` are captured via Colly's `OnResponse` callback:
- When a response is received for a URL that exists in `discovered_links`, update its `http_status` and `content_type`
- This happens naturally during the crawl — no separate probing needed

### Domain extraction

```go
func extractDomain(rawURL string) string {
    u, err := url.Parse(rawURL)
    if err != nil {
        return ""
    }
    return strings.TrimPrefix(u.Hostname(), "www.")
}
```

Strip `www.` prefix so `www.example.com` and `example.com` group together.

---

## Bulk Operations Model

### Frontend

- Checkbox column in domain table
- Selection state tracked in component (array of domain strings)
- Floating action bar appears when `selectedDomains.length > 0`
- Actions dispatch `POST /api/v1/discovered-domains/bulk-state`
- On success, invalidate query cache and clear selection

### Backend

- Single endpoint handles bulk state updates
- Transaction wraps all upserts to `discovered_domain_states`
- Returns count of updated domains
- Max batch size: 100 domains per request (frontend chunks if needed)

---

## Future Automation Hooks

The scoring model and domain tracking enable future automation without schema changes:

1. **Auto-suggest sources**: Cron job queries domains with `quality_score >= 80`, `source_count >= 3`, `status = 'active'`, `is_existing_source = false` — surfaces them in a "Suggested Sources" widget
2. **Auto-ignore**: Domains consistently scoring below 20 for 30+ days could be auto-ignored
3. **Feed discovery**: When a domain is promoted, automatically probe for RSS/Atom feeds
4. **Domain reputation**: External reputation APIs could contribute to the quality score as an additional signal
