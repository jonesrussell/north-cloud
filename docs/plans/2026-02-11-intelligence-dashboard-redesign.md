# Intelligence Dashboard Redesign

## Primary Question

**"Is the pipeline healthy and producing useful content right now?"**

Every element on the page exists to answer this. No infra metrics (CPU, pods) - those belong elsewhere. This page is pipeline health + content intelligence.

---

## Architecture

### Layout (top to bottom, single scrollable page)

1. **Problems Banner** - conditional, hidden when healthy
2. **Pipeline KPI Strip** - 4-6 key metrics, always visible
3. **Source Health Table** - the workhorse, one row per source
4. **Content Intelligence Cards** - compact crime/mining/quality/location summaries

### Data Flow

The page makes parallel requests to 3 services on mount:
- **Index-manager**: `/api/v1/aggregations/source-health` (new), `/api/v1/aggregations/overview`, `/api/v1/stats`
- **Crawler**: `/api/v1/scheduler/metrics`, `/api/v1/jobs?status=failed`
- **Publisher**: `/api/v1/stats/overview?period=today`, `/api/v1/channels`

All fetched via a single composable (`usePipelineHealth`). If any service fetch fails, that itself is surfaced as a problem ("Crawler metrics unavailable - service may be down"). No unknown states.

---

## Problems Detection System

Co-located in `dashboard/src/features/intelligence/problems/`. Pure functions, fully testable.

### File Structure

```
problems/
  types.ts          # Problem, Severity, PipelineMetrics interfaces
  rules.ts          # detectProblems(metrics: PipelineMetrics): Problem[]
  rules.test.ts     # Unit tests per rule + happy path
  ProblemsBanner.vue # Receives Problem[] - no data fetching
```

### Problem Type

```typescript
interface Problem {
  id: string
  kind: 'crawler' | 'publisher' | 'index' | 'system'
  severity: 'error' | 'warning'
  title: string           // "18 failed crawl jobs"
  action: string          // "Open job details, check last error, consider disabling stale sources"
  link?: string           // Route to drill-down
  count?: number
  sourceIds?: string[]    // Pre-filter drill-down views
}
```

### Detection Rules

| Rule ID | Kind | Severity | Condition | Suggested Action |
|---------|------|----------|-----------|-----------------|
| `failed-crawls` | crawler | error | failedJobs > 0 | Open job details, check last error, consider disabling or fixing source config |
| `empty-indexes` | index | warning | classified_count = 0 AND active = true | Verify crawler is configured and running for this source |
| `near-empty-indexes` | index | warning | classified_count < 10 AND source active > 7 days | Check crawler selectors, source may require tuning |
| `inactive-channels` | publisher | warning | channel.active = false | Enable channel or remove if no longer needed |
| `zero-publishing` | publisher | error | total_published_24h = 0 | Check channel status, route configuration, and classified content availability |
| `classification-backlog` | index | warning | raw_count - classified_count > threshold | Classifier may be stalled or slow, check service logs |
| `stale-scheduled-jobs` | crawler | error | next_run_at in the past | Crawler scheduler may be down, check service health |
| `cluster-health` | system | error/warning | health != 'green' | Check Elasticsearch cluster status |
| `service-unreachable` | system | error | any fetch returns null | Service may be down or auth misconfigured |

### Testing Requirements

- One test per rule (given metrics with the condition, expect the problem)
- Happy path test: all metrics healthy, expect empty problem list
- Edge cases: thresholds at boundary values

---

## Pipeline KPI Strip

Always visible. 4-6 metrics in a horizontal row.

| Metric | Source | Highlight |
|--------|--------|-----------|
| Crawled (24h) | sum delta_24h across raw indexes | Red if 0 |
| Classified (24h) | sum delta_24h across classified indexes | Red if 0 |
| Published (24h) | publisher stats `period=today` | Red if 0 |
| Failed jobs | crawler scheduler metrics | Red if > 0, hidden if 0 |
| Empty indexes | source-health where classified_count=0 AND active | Amber if > 0, hidden if 0 |
| Pipeline yield | published_24h / crawled_24h as % | Amber if < 10% |

Failed jobs and empty indexes KPIs only appear when non-zero to keep the healthy state clean.

---

## Source Health Table

One row per source. Designed for fast triage during incidents.

### Columns

| Column | Description | View |
|--------|-------------|------|
| Source | Name, extracted from index naming | Both |
| Status | Derived dot (green/amber/red) + last-error tooltip on hover | Both |
| Raw docs | `{source}_raw_content` doc count | Ops |
| Classified docs | `{source}_classified_content` doc count | Ops |
| Backlog | raw - classified, amber if > 0 | Ops |
| Crawl status | Last execution result + error tooltip | Ops |
| Next run | From crawler job | Ops |
| Avg quality | Mean quality_score | Dev |
| 24h delta | Classified docs gained, "stale" if 0 for 48h+ | Dev |
| Published (24h) | Published count from publisher stats | Dev |
| Publish ratio | Published / Classified | Dev |

### Quick Filters (chip bar)

- **All** (default, errors sort to top)
- **Only errors**
- **Only warnings**
- **No docs** (active sources only - paused sources excluded)
- **Backlog present**

### Column Presets

Toggle between Ops view and Dev view. Single control, no separate pages. Preference persisted to localStorage.

### Inactive Sources

Sources with `active: false` render with muted styling and are excluded from problem detection. The "No docs" filter ignores them by default.

---

## Content Intelligence Summary

Below the source table. One row of 4 compact cards replacing the current large navigation tiles.

- **Crime**: total count + top 3 sub-categories. Links to `/intelligence/crime`
- **Mining**: total count + top 3 commodities. Links to `/intelligence/mining`
- **Quality**: horizontal bar (high/medium/low split). Inline, no drill-down needed
- **Location**: top 3 cities. Links to `/intelligence/location`

Drill-down pages (CrimeBreakdownView, MiningBreakdownView, LocationBreakdownView) stay as-is. Index Explorer stays at `/intelligence/indexes`.

---

## Backend Changes

### New: Source Health Endpoint (index-manager)

`GET /api/v1/aggregations/source-health`

Queries all ES index pairs in one pass. Returns per-source stats.

```go
type SourceHealth struct {
    Source          string  `json:"source"`
    SourceID        string  `json:"source_id"`
    Active          bool    `json:"active"`
    RawCount        int     `json:"raw_count"`
    ClassifiedCount int     `json:"classified_count"`
    Backlog         int     `json:"backlog"`
    Delta24h        int     `json:"delta_24h"`
    AvgQuality      float64 `json:"avg_quality"`
    PublishedCount  int     `json:"published_count"`
    LastCrawlError  string  `json:"last_crawl_error"`
    LastCrawlAt     string  `json:"last_crawl_at"`
    NextRunAt       string  `json:"next_run_at"`
}
```

Implementation: index-manager already enumerates `*_raw_content` / `*_classified_content` pairs. Add aggregation queries for doc counts, 24h delta (date range on `created_at`), and avg quality score. Crawl status and publisher data are joined client-side from parallel fetches.

Simplified backend shape (index-manager only owns what it can query from ES):

```go
type SourceHealth struct {
    Source          string  `json:"source"`
    RawCount        int     `json:"raw_count"`
    ClassifiedCount int     `json:"classified_count"`
    Backlog         int     `json:"backlog"`
    Delta24h        int     `json:"delta_24h"`
    AvgQuality      float64 `json:"avg_quality"`
}
```

Crawler fields (`last_crawl_error`, `next_run_at`) and publisher fields (`published_count`) come from their respective APIs and are joined in the frontend composable.

### Fix: MCP list_sources Deserialization (mcp-north-cloud)

**File**: `mcp-north-cloud/internal/client/source_manager.go`

Change unmarshal target from `[]Source` to wrapper struct:

```go
var response struct {
    Sources []Source `json:"sources"`
    Total   int      `json:"total"`
}
```

### Fix: Publisher Routes API (publisher)

Implement missing `/api/v1/routes` endpoint group:

- `GET /api/v1/routes` - list with optional `source_id`/`channel_id` filters
- `POST /api/v1/routes` - create route
- `DELETE /api/v1/routes/:id` - delete route
- `GET /api/v1/routes/:id/preview` - preview matching articles

Database tables already exist. This is HTTP layer + service methods.

### Dashboard API Client Additions

Add `crawlerApi` and `publisherApi` sections to `dashboard/src/api/client.ts`. Wire to services via nginx proxy paths (`/api/crawler/*`, `/api/publisher/*`).

Check nginx config for existing proxy rules; add if missing.

---

## Frontend Changes

### New Files

```
dashboard/src/features/intelligence/
  problems/
    types.ts
    rules.ts
    rules.test.ts
    ProblemsBanner.vue
  composables/
    usePipelineHealth.ts       # Parallel fetches to all 3 services
  SourceHealthTable.vue
  PipelineKPIs.vue
  ContentSummaryCards.vue
```

### Deleted Files (replaced by unified dashboard)

- `dashboard/src/views/intelligence/IntelligenceOverviewView.vue` - rewritten in place
- `dashboard/src/components/intelligence/ContentIntelligenceSummary.vue` - replaced by ContentSummaryCards
- `dashboard/src/composables/useIntelligenceOverview.ts` - replaced by usePipelineHealth
- `dashboard/src/config/intelligence.ts` - drill-down config no longer needed

### Modified Files

- `dashboard/src/views/intelligence/IntelligenceOverviewView.vue` - rewritten to compose: ProblemsBanner + PipelineKPIs + SourceHealthTable + ContentSummaryCards
- `dashboard/src/api/client.ts` - add crawlerApi, publisherApi sections
- `dashboard/src/router/index.ts` - remove dead route references if any config imports are cleaned up

### Unchanged

- `IndexesView.vue`, `IndexDetailView.vue`, `DocumentDetailView.vue` - kept as-is
- `CrimeBreakdownView.vue`, `MiningBreakdownView.vue`, `LocationBreakdownView.vue` - kept as-is
- `IndexesTable.vue`, `IndexStatsCards.vue`, `IndexesFilterBar.vue` - kept as-is
- `useIndexes.ts` composable - kept as-is

---

## Operational Fixes (included in scope)

1. **Investigate 18 failed crawl jobs** - check error messages, fix or disable stale sources
2. **Investigate inactive channels** - Crime Feed and Mining Feed both `active: false`
3. **Investigate zero publishing** - pipeline not flowing, likely blocked by inactive channels
4. **Fix MCP list_sources bug** - wrapper struct mismatch
5. **Fix MCP list_routes / publisher routes** - implement missing endpoints

---

## Implementation Order

1. **Bug fixes first**: MCP list_sources, publisher routes API
2. **Backend**: source-health endpoint on index-manager
3. **Frontend**: problems engine (types, rules, tests, banner)
4. **Frontend**: usePipelineHealth composable, API client additions
5. **Frontend**: PipelineKPIs, SourceHealthTable, ContentSummaryCards
6. **Frontend**: rewrite IntelligenceOverviewView to compose new components
7. **Cleanup**: delete old components/composables, verify no dead imports
8. **Operational**: investigate and fix failed crawls, inactive channels, zero publishing
