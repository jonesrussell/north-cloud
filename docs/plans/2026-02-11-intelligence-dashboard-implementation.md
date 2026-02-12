# Intelligence Dashboard Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the Intelligence overview page with a unified dashboard that surfaces pipeline health problems, shows per-source status, and retains content intelligence drill-downs.

**Architecture:** Frontend-driven problem detection. The `usePipelineHealth` composable fetches from 3 services in parallel (index-manager, crawler, publisher). Pure `detectProblems()` rules run client-side over the metrics. The only new backend endpoint is `GET /api/v1/aggregations/source-health` on index-manager, which returns per-source doc counts, backlog, 24h delta, and avg quality from Elasticsearch.

**Tech Stack:** Go 1.25+ (index-manager), Vue 3 Composition API, TypeScript strict, Tailwind CSS 4, TanStack Query, Vitest (new), Lucide icons

**Design doc:** `docs/plans/2026-02-11-intelligence-dashboard-redesign.md`

---

## Task 1: Fix MCP list_sources Deserialization Bug

**Files:**
- Modify: `mcp-north-cloud/internal/client/source_manager.go:119-149`

**Step 1: Fix the unmarshal target**

The source-manager API returns `{"sources": [...], "total": N}` but the client unmarshals into `[]Source`. Change lines 143-148:

```go
// Before (broken):
var sources []Source
if err = json.Unmarshal(body, &sources); err != nil {
    return nil, fmt.Errorf("failed to parse response: %w", err)
}
return sources, nil

// After (fixed):
var response struct {
    Sources []Source `json:"sources"`
    Total   int      `json:"total"`
}
if err = json.Unmarshal(body, &response); err != nil {
    return nil, fmt.Errorf("failed to parse response: %w", err)
}
return response.Sources, nil
```

**Step 2: Lint**

Run: `cd mcp-north-cloud && golangci-lint run ./internal/client/...`
Expected: No errors

**Step 3: Build**

Run: `cd mcp-north-cloud && go build -o /dev/null .`
Expected: Compiles cleanly

**Step 4: Commit**

```bash
git add mcp-north-cloud/internal/client/source_manager.go
git commit -m "fix(mcp): fix list_sources JSON deserialization

API returns {sources: [...], total: N} wrapper but client expected
bare array. Use wrapper struct to match actual response format."
```

---

## Task 2: Add SourceHealth Domain Type (index-manager)

**Files:**
- Modify: `index-manager/internal/domain/aggregation.go`

**Step 1: Add the SourceHealth struct**

Append to `index-manager/internal/domain/aggregation.go` after the `AggregationRequest` struct (after line 49):

```go
// SourceHealth represents per-source pipeline health metrics from Elasticsearch
type SourceHealth struct {
	Source          string  `json:"source"`
	RawCount        int64   `json:"raw_count"`
	ClassifiedCount int64   `json:"classified_count"`
	Backlog         int64   `json:"backlog"`
	Delta24h        int64   `json:"delta_24h"`
	AvgQuality      float64 `json:"avg_quality"`
}

// SourceHealthResponse represents the response for source health aggregation
type SourceHealthResponse struct {
	Sources []SourceHealth `json:"sources"`
	Total   int            `json:"total"`
}
```

**Step 2: Lint**

Run: `cd index-manager && golangci-lint run ./internal/domain/...`
Expected: No errors

**Step 3: Commit**

```bash
git add index-manager/internal/domain/aggregation.go
git commit -m "feat(index-manager): add SourceHealth domain type"
```

---

## Task 3: Implement GetSourceHealth Service Method (index-manager)

**Files:**
- Modify: `index-manager/internal/service/aggregation_service.go`

This is the core backend work. The method needs to:
1. Get all ES index names (raw + classified pairs)
2. Get doc counts per index via `_cat/indices`
3. For classified indexes, run a single multi-index aggregation for avg quality and 24h delta per source
4. Pair up raw/classified counts and return `[]SourceHealth`

**Step 1: Add the GetSourceHealth method**

Append to `index-manager/internal/service/aggregation_service.go` before `buildAggregationQuery` (before line 272):

```go
const (
	sourceHealthDeltaHours = 24
)

// GetSourceHealth returns per-source pipeline health metrics
func (s *AggregationService) GetSourceHealth(ctx context.Context) (*domain.SourceHealthResponse, error) {
	// Step 1: Get doc counts for all indexes via _cat/indices
	indexCounts, catErr := s.getIndexDocCounts(ctx)
	if catErr != nil {
		return nil, fmt.Errorf("failed to get index doc counts: %w", catErr)
	}

	// Step 2: Build source pairs from index names
	sources := s.buildSourcePairs(indexCounts)

	// Step 3: Get per-source quality and 24h delta from classified content
	qualityMap, deltaMap, aggErr := s.getClassifiedAggregations(ctx)
	if aggErr != nil {
		s.logger.Warn("Failed to get classified aggregations, continuing with counts only",
			infralogger.String("error", aggErr.Error()))
	}

	// Step 4: Build response
	result := make([]domain.SourceHealth, 0, len(sources))
	for source, pair := range sources {
		backlog := pair.rawCount - pair.classifiedCount
		if backlog < 0 {
			backlog = 0
		}
		result = append(result, domain.SourceHealth{
			Source:          source,
			RawCount:        pair.rawCount,
			ClassifiedCount: pair.classifiedCount,
			Backlog:         backlog,
			Delta24h:        deltaMap[source],
			AvgQuality:      qualityMap[source],
		})
	}

	return &domain.SourceHealthResponse{
		Sources: result,
		Total:   len(result),
	}, nil
}
```

**Step 2: Add helper types and methods**

Add these helpers after the `GetSourceHealth` method:

```go
type sourcePair struct {
	rawCount        int64
	classifiedCount int64
}

// getIndexDocCounts returns doc counts for all indexes via ES _cat/indices API
func (s *AggregationService) getIndexDocCounts(ctx context.Context) (map[string]int64, error) {
	return s.esClient.GetAllIndexDocCounts(ctx)
}

// buildSourcePairs extracts source names and pairs raw/classified counts
func (s *AggregationService) buildSourcePairs(indexCounts map[string]int64) map[string]*sourcePair {
	sources := make(map[string]*sourcePair)
	const (
		rawSuffix        = "_raw_content"
		classifiedSuffix = "_classified_content"
	)

	for indexName, count := range indexCounts {
		var source string
		switch {
		case len(indexName) > len(rawSuffix) && indexName[len(indexName)-len(rawSuffix):] == rawSuffix:
			source = indexName[:len(indexName)-len(rawSuffix)]
		case len(indexName) > len(classifiedSuffix) && indexName[len(indexName)-len(classifiedSuffix):] == classifiedSuffix:
			source = indexName[:len(indexName)-len(classifiedSuffix)]
		default:
			continue
		}

		if _, ok := sources[source]; !ok {
			sources[source] = &sourcePair{}
		}

		if indexName[len(indexName)-len(rawSuffix):] == rawSuffix {
			sources[source].rawCount = count
		} else {
			sources[source].classifiedCount = count
		}
	}

	return sources
}

// getClassifiedAggregations queries all classified content for per-source avg quality and 24h delta
func (s *AggregationService) getClassifiedAggregations(
	ctx context.Context,
) (qualityMap map[string]float64, deltaMap map[string]int64, err error) {
	qualityMap = make(map[string]float64)
	deltaMap = make(map[string]int64)

	query := map[string]any{
		"size":             0,
		"track_total_hits": true,
		"aggs": map[string]any{
			"by_source": map[string]any{
				"terms": map[string]any{
					"field": "source_name",
					"size":  200,
				},
				"aggs": map[string]any{
					"avg_quality": map[string]any{
						"avg": map[string]any{
							"field": "quality_score",
						},
					},
					"recent_24h": map[string]any{
						"filter": map[string]any{
							"range": map[string]any{
								"classified_at": map[string]any{
									"gte": "now-24h",
								},
							},
						},
					},
				},
			},
		},
	}

	res, searchErr := s.esClient.SearchAllClassifiedContent(ctx, query)
	if searchErr != nil {
		return qualityMap, deltaMap, fmt.Errorf("failed to execute source health aggregation: %w", searchErr)
	}
	defer func() { _ = res.Body.Close() }()

	var esResp sourceHealthAggResponse
	if decodeErr := json.NewDecoder(res.Body).Decode(&esResp); decodeErr != nil {
		return qualityMap, deltaMap, fmt.Errorf("failed to decode source health response: %w", decodeErr)
	}

	for _, bucket := range esResp.Aggregations.BySource.Buckets {
		qualityMap[bucket.Key] = bucket.AvgQuality.Value
		deltaMap[bucket.Key] = bucket.Recent24h.DocCount
	}

	return qualityMap, deltaMap, nil
}

// sourceHealthAggResponse is the ES response structure for source health aggregations
type sourceHealthAggResponse struct {
	Aggregations struct {
		BySource struct {
			Buckets []struct {
				Key        string `json:"key"`
				DocCount   int64  `json:"doc_count"`
				AvgQuality struct {
					Value float64 `json:"value"`
				} `json:"avg_quality"`
				Recent24h struct {
					DocCount int64 `json:"doc_count"`
				} `json:"recent_24h"`
			} `json:"buckets"`
		} `json:"by_source"`
	} `json:"aggregations"`
}
```

**Step 3: Add GetAllIndexDocCounts to ES client**

Check if `index-manager/internal/elasticsearch/client.go` has a method to get doc counts. If not, add:

```go
// GetAllIndexDocCounts returns document counts for all indexes using the _cat/indices API
func (c *Client) GetAllIndexDocCounts(ctx context.Context) (map[string]int64, error) {
	res, err := c.es.Cat.Indices(
		c.es.Cat.Indices.WithContext(ctx),
		c.es.Cat.Indices.WithFormat("json"),
		c.es.Cat.Indices.WithH("index", "docs.count"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get cat indices: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return nil, fmt.Errorf("cat indices returned error: %s", res.String())
	}

	var indices []struct {
		Index    string `json:"index"`
		DocsCount string `json:"docs.count"`
	}
	if decodeErr := json.NewDecoder(res.Body).Decode(&indices); decodeErr != nil {
		return nil, fmt.Errorf("failed to decode cat indices: %w", decodeErr)
	}

	result := make(map[string]int64, len(indices))
	for _, idx := range indices {
		count, _ := strconv.ParseInt(idx.DocsCount, 10, 64)
		result[idx.Index] = count
	}

	return result, nil
}
```

Make sure to add `"strconv"` to the imports in that file.

**Step 4: Lint**

Run: `cd index-manager && golangci-lint run ./internal/...`
Expected: No errors. Watch for funlen (100 line limit) and gocognit (complexity 20). If `GetSourceHealth` or helpers are too long, extract further.

**Step 5: Commit**

```bash
git add index-manager/internal/service/aggregation_service.go index-manager/internal/elasticsearch/client.go
git commit -m "feat(index-manager): implement source health aggregation

Queries all ES index pairs to return per-source doc counts, backlog,
24h delta, and avg quality score in a single endpoint."
```

---

## Task 4: Add Source Health Handler and Route (index-manager)

**Files:**
- Modify: `index-manager/internal/api/handlers.go`
- Modify: `index-manager/internal/api/routes.go:44-49`

**Step 1: Add handler method**

Add to `handlers.go` after `GetMiningAggregation` (after line ~677):

```go
// GetSourceHealth returns per-source pipeline health metrics
func (h *Handler) GetSourceHealth(c *gin.Context) {
	result, err := h.aggregationService.GetSourceHealth(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
```

**Step 2: Register the route**

In `routes.go`, add after line 49 (after the mining aggregation route):

```go
aggregations.GET("/source-health", handler.GetSourceHealth) // GET /api/v1/aggregations/source-health
```

**Step 3: Lint and build**

Run: `cd index-manager && golangci-lint run && go build -o /dev/null .`
Expected: No errors

**Step 4: Commit**

```bash
git add index-manager/internal/api/handlers.go index-manager/internal/api/routes.go
git commit -m "feat(index-manager): add source-health API endpoint

GET /api/v1/aggregations/source-health returns per-source pipeline
health with doc counts, backlog, delta, and quality."
```

---

## Task 5: Add Source Health to Dashboard API Client

**Files:**
- Modify: `dashboard/src/api/client.ts:524-543` (aggregations section)
- Modify: `dashboard/src/types/aggregation.ts`

**Step 1: Add TypeScript types**

Append to `dashboard/src/types/aggregation.ts`:

```typescript
// Source Health (pipeline health per source)
export interface SourceHealth {
  source: string
  raw_count: number
  classified_count: number
  backlog: number
  delta_24h: number
  avg_quality: number
}

export interface SourceHealthResponse {
  sources: SourceHealth[]
  total: number
}
```

**Step 2: Add API method**

In `dashboard/src/api/client.ts`, add the import for `SourceHealthResponse` to the aggregation imports (line ~36), then add to the `indexManagerApi.aggregations` object (after `getMining` around line 541):

```typescript
getSourceHealth: (): Promise<AxiosResponse<SourceHealthResponse>> =>
  indexManagerClient.get('/api/v1/aggregations/source-health'),
```

**Step 3: Lint**

Run: `cd dashboard && npm run lint`
Expected: No errors

**Step 4: Commit**

```bash
git add dashboard/src/types/aggregation.ts dashboard/src/api/client.ts
git commit -m "feat(dashboard): add source-health API client method"
```

---

## Task 6: Set Up Vitest for Dashboard

**Files:**
- Modify: `dashboard/package.json`
- Create: `dashboard/vitest.config.ts`

**Step 1: Install vitest**

Run: `cd dashboard && npm install -D vitest`

**Step 2: Create vitest config**

Create `dashboard/vitest.config.ts`:

```typescript
import { defineConfig } from 'vitest/config'
import { resolve } from 'path'

export default defineConfig({
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
})
```

**Step 3: Add test script to package.json**

In `dashboard/package.json` `scripts` section, add:

```json
"test": "vitest run",
"test:watch": "vitest"
```

**Step 4: Verify setup**

Run: `cd dashboard && npm run test`
Expected: `No test files found` (no tests yet, but vitest runs successfully)

**Step 5: Commit**

```bash
git add dashboard/package.json dashboard/package-lock.json dashboard/vitest.config.ts
git commit -m "chore(dashboard): add vitest test framework"
```

---

## Task 7: Create Problem Detection Types

**Files:**
- Create: `dashboard/src/features/intelligence/problems/types.ts`

**Step 1: Create the types file**

```typescript
// Problem detection types for the intelligence dashboard.
// All problem rules consume PipelineMetrics and produce Problem[].

export type ProblemKind = 'crawler' | 'publisher' | 'index' | 'system'
export type ProblemSeverity = 'error' | 'warning'

export interface Problem {
  id: string
  kind: ProblemKind
  severity: ProblemSeverity
  title: string
  action: string
  link?: string
  count?: number
  sourceIds?: string[]
}

// Metrics fetched from each service. null = service unreachable.
export interface CrawlerMetrics {
  failedJobs: number
  staleJobs: number // scheduled with next_run_at in the past
  failedJobUrls: string[]
}

export interface IndexMetrics {
  clusterHealth: 'green' | 'yellow' | 'red'
  sources: SourceMetrics[]
}

export interface SourceMetrics {
  source: string
  rawCount: number
  classifiedCount: number
  backlog: number
  delta24h: number
  avgQuality: number
  active: boolean
}

export interface PublisherMetrics {
  publishedToday: number
  inactiveChannels: number
  inactiveChannelNames: string[]
}

export interface PipelineMetrics {
  crawler: CrawlerMetrics | null
  indexes: IndexMetrics | null
  publisher: PublisherMetrics | null
}
```

**Step 2: Commit**

```bash
git add dashboard/src/features/intelligence/problems/types.ts
git commit -m "feat(dashboard): add problem detection type definitions"
```

---

## Task 8: Implement Problem Detection Rules with Tests (TDD)

**Files:**
- Create: `dashboard/src/features/intelligence/problems/rules.test.ts`
- Create: `dashboard/src/features/intelligence/problems/rules.ts`

**Step 1: Write the failing tests**

Create `dashboard/src/features/intelligence/problems/rules.test.ts`:

```typescript
import { describe, it, expect } from 'vitest'
import { detectProblems } from './rules'
import type { PipelineMetrics, CrawlerMetrics, IndexMetrics, PublisherMetrics } from './types'

// Factory for healthy defaults
function healthyMetrics(): PipelineMetrics {
  return {
    crawler: { failedJobs: 0, staleJobs: 0, failedJobUrls: [] },
    indexes: {
      clusterHealth: 'green',
      sources: [
        { source: 'example_com', rawCount: 100, classifiedCount: 95, backlog: 5, delta24h: 10, avgQuality: 72, active: true },
      ],
    },
    publisher: { publishedToday: 42, inactiveChannels: 0, inactiveChannelNames: [] },
  }
}

describe('detectProblems', () => {
  it('returns empty array when everything is healthy', () => {
    const problems = detectProblems(healthyMetrics())
    expect(problems).toEqual([])
  })

  it('detects failed crawl jobs', () => {
    const metrics = healthyMetrics()
    metrics.crawler!.failedJobs = 18
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'failed-crawls')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('error')
    expect(p!.kind).toBe('crawler')
    expect(p!.count).toBe(18)
  })

  it('detects stale scheduled jobs', () => {
    const metrics = healthyMetrics()
    metrics.crawler!.staleJobs = 3
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'stale-scheduled-jobs')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('error')
  })

  it('detects empty indexes for active sources', () => {
    const metrics = healthyMetrics()
    metrics.indexes!.sources = [
      { source: 'dead_source', rawCount: 0, classifiedCount: 0, backlog: 0, delta24h: 0, avgQuality: 0, active: true },
    ]
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'empty-indexes')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('warning')
    expect(p!.count).toBe(1)
  })

  it('ignores empty indexes for inactive sources', () => {
    const metrics = healthyMetrics()
    metrics.indexes!.sources = [
      { source: 'paused', rawCount: 0, classifiedCount: 0, backlog: 0, delta24h: 0, avgQuality: 0, active: false },
    ]
    const problems = detectProblems(metrics)
    expect(problems.find((p) => p.id === 'empty-indexes')).toBeUndefined()
  })

  it('detects classification backlog', () => {
    const metrics = healthyMetrics()
    metrics.indexes!.sources = [
      { source: 'backed_up', rawCount: 500, classifiedCount: 100, backlog: 400, delta24h: 0, avgQuality: 60, active: true },
    ]
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'classification-backlog')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('warning')
  })

  it('detects inactive channels', () => {
    const metrics = healthyMetrics()
    metrics.publisher!.inactiveChannels = 2
    metrics.publisher!.inactiveChannelNames = ['Crime Feed', 'Mining Feed']
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'inactive-channels')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('warning')
    expect(p!.count).toBe(2)
  })

  it('detects zero publishing', () => {
    const metrics = healthyMetrics()
    metrics.publisher!.publishedToday = 0
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'zero-publishing')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('error')
  })

  it('detects degraded cluster health', () => {
    const metrics = healthyMetrics()
    metrics.indexes!.clusterHealth = 'yellow'
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'cluster-health')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('warning')
  })

  it('detects red cluster health as error', () => {
    const metrics = healthyMetrics()
    metrics.indexes!.clusterHealth = 'red'
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'cluster-health')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('error')
  })

  it('detects unreachable crawler service', () => {
    const metrics = healthyMetrics()
    metrics.crawler = null
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'service-unreachable-crawler')
    expect(p).toBeDefined()
    expect(p!.severity).toBe('error')
    expect(p!.kind).toBe('system')
  })

  it('detects unreachable publisher service', () => {
    const metrics = healthyMetrics()
    metrics.publisher = null
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'service-unreachable-publisher')
    expect(p).toBeDefined()
  })

  it('detects unreachable index-manager service', () => {
    const metrics = healthyMetrics()
    metrics.indexes = null
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'service-unreachable-indexes')
    expect(p).toBeDefined()
  })
})
```

**Step 2: Run tests to verify they fail**

Run: `cd dashboard && npx vitest run src/features/intelligence/problems/rules.test.ts`
Expected: FAIL - `Cannot find module './rules'`

**Step 3: Implement the rules**

Create `dashboard/src/features/intelligence/problems/rules.ts`:

```typescript
import type { PipelineMetrics, Problem } from './types'

const BACKLOG_THRESHOLD = 100

export function detectProblems(metrics: PipelineMetrics): Problem[] {
  const problems: Problem[] = []

  detectServiceUnreachable(metrics, problems)

  if (metrics.crawler) {
    detectCrawlerProblems(metrics.crawler, problems)
  }
  if (metrics.indexes) {
    detectIndexProblems(metrics.indexes, problems)
  }
  if (metrics.publisher) {
    detectPublisherProblems(metrics.publisher, problems)
  }

  return problems
}

function detectServiceUnreachable(metrics: PipelineMetrics, problems: Problem[]): void {
  const services = [
    { key: 'crawler' as const, label: 'Crawler' },
    { key: 'indexes' as const, label: 'Index manager' },
    { key: 'publisher' as const, label: 'Publisher' },
  ]
  for (const svc of services) {
    if (metrics[svc.key] === null) {
      problems.push({
        id: `service-unreachable-${svc.key}`,
        kind: 'system',
        severity: 'error',
        title: `${svc.label} metrics unavailable`,
        action: `${svc.label} service may be down or auth misconfigured. Check service health and logs.`,
      })
    }
  }
}

function detectCrawlerProblems(
  crawler: NonNullable<PipelineMetrics['crawler']>,
  problems: Problem[],
): void {
  if (crawler.failedJobs > 0) {
    problems.push({
      id: 'failed-crawls',
      kind: 'crawler',
      severity: 'error',
      title: `${crawler.failedJobs} failed crawl job${crawler.failedJobs === 1 ? '' : 's'}`,
      action: 'Open job details, check last error, consider disabling or fixing source config.',
      link: '/jobs?status=failed',
      count: crawler.failedJobs,
    })
  }
  if (crawler.staleJobs > 0) {
    problems.push({
      id: 'stale-scheduled-jobs',
      kind: 'crawler',
      severity: 'error',
      title: `${crawler.staleJobs} stale scheduled job${crawler.staleJobs === 1 ? '' : 's'}`,
      action: 'Crawler scheduler may be down. Check service health.',
      link: '/jobs',
      count: crawler.staleJobs,
    })
  }
}

function detectIndexProblems(
  indexes: NonNullable<PipelineMetrics['indexes']>,
  problems: Problem[],
): void {
  if (indexes.clusterHealth !== 'green') {
    problems.push({
      id: 'cluster-health',
      kind: 'system',
      severity: indexes.clusterHealth === 'red' ? 'error' : 'warning',
      title: `Elasticsearch cluster health: ${indexes.clusterHealth}`,
      action: 'Check Elasticsearch cluster status and shard allocation.',
    })
  }

  const activeSources = indexes.sources.filter((s) => s.active)
  const emptySources = activeSources.filter((s) => s.classifiedCount === 0)
  if (emptySources.length > 0) {
    problems.push({
      id: 'empty-indexes',
      kind: 'index',
      severity: 'warning',
      title: `${emptySources.length} active source${emptySources.length === 1 ? '' : 's'} with no classified content`,
      action: 'Verify crawler is configured and running for these sources.',
      link: '/intelligence/indexes',
      count: emptySources.length,
      sourceIds: emptySources.map((s) => s.source),
    })
  }

  const backlogSources = activeSources.filter((s) => s.backlog > BACKLOG_THRESHOLD)
  if (backlogSources.length > 0) {
    problems.push({
      id: 'classification-backlog',
      kind: 'index',
      severity: 'warning',
      title: `${backlogSources.length} source${backlogSources.length === 1 ? '' : 's'} with classification backlog`,
      action: 'Classifier may be stalled or slow. Check service logs.',
      count: backlogSources.length,
      sourceIds: backlogSources.map((s) => s.source),
    })
  }
}

function detectPublisherProblems(
  publisher: NonNullable<PipelineMetrics['publisher']>,
  problems: Problem[],
): void {
  if (publisher.inactiveChannels > 0) {
    problems.push({
      id: 'inactive-channels',
      kind: 'publisher',
      severity: 'warning',
      title: `${publisher.inactiveChannels} inactive channel${publisher.inactiveChannels === 1 ? '' : 's'}`,
      action: `Enable or remove: ${publisher.inactiveChannelNames.join(', ')}.`,
      link: '/channels',
      count: publisher.inactiveChannels,
    })
  }
  if (publisher.publishedToday === 0) {
    problems.push({
      id: 'zero-publishing',
      kind: 'publisher',
      severity: 'error',
      title: 'No articles published today',
      action: 'Check channel status, route configuration, and classified content availability.',
      link: '/channels',
    })
  }
}
```

**Step 4: Run tests to verify they pass**

Run: `cd dashboard && npx vitest run src/features/intelligence/problems/rules.test.ts`
Expected: All 12 tests PASS

**Step 5: Commit**

```bash
git add dashboard/src/features/intelligence/problems/
git commit -m "feat(dashboard): implement problem detection rules with tests

Pure function detectProblems() analyzes pipeline metrics and returns
actionable problems. 12 unit tests cover all rules plus happy path."
```

---

## Task 9: Create ProblemsBanner Component

**Files:**
- Create: `dashboard/src/features/intelligence/problems/ProblemsBanner.vue`

**Step 1: Create the component**

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { AlertTriangle, XCircle } from 'lucide-vue-next'
import type { Problem } from './types'

const props = defineProps<{
  problems: Problem[]
}>()

const router = useRouter()

const errors = computed(() => props.problems.filter((p) => p.severity === 'error'))
const warnings = computed(() => props.problems.filter((p) => p.severity === 'warning'))

function handleClick(problem: Problem) {
  if (problem.link) {
    router.push(problem.link)
  }
}
</script>

<template>
  <div
    v-if="problems.length > 0"
    class="rounded-lg border p-4 space-y-2"
    :class="errors.length > 0 ? 'border-red-500/30 bg-red-500/5' : 'border-amber-500/30 bg-amber-500/5'"
  >
    <div class="flex items-center gap-2 text-sm font-medium">
      <XCircle v-if="errors.length > 0" class="h-4 w-4 text-red-500 shrink-0" />
      <AlertTriangle v-else class="h-4 w-4 text-amber-500 shrink-0" />
      <span>
        {{ problems.length }} issue{{ problems.length === 1 ? '' : 's' }} detected
      </span>
    </div>
    <div class="flex flex-wrap gap-2">
      <button
        v-for="problem in problems"
        :key="problem.id"
        class="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
        :class="
          problem.severity === 'error'
            ? 'bg-red-500/10 text-red-700 dark:text-red-400 hover:bg-red-500/20'
            : 'bg-amber-500/10 text-amber-700 dark:text-amber-400 hover:bg-amber-500/20'
        "
        :title="problem.action"
        @click="handleClick(problem)"
      >
        <span v-if="problem.count" class="font-semibold tabular-nums">{{ problem.count }}</span>
        {{ problem.title }}
      </button>
    </div>
  </div>
</template>
```

**Step 2: Lint**

Run: `cd dashboard && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add dashboard/src/features/intelligence/problems/ProblemsBanner.vue
git commit -m "feat(dashboard): add ProblemsBanner component

Renders detected problems as clickable chips with severity coloring.
Hidden when no problems exist."
```

---

## Task 10: Create usePipelineHealth Composable

**Files:**
- Create: `dashboard/src/features/intelligence/composables/usePipelineHealth.ts`

This composable fetches from all 3 services in parallel and assembles `PipelineMetrics`.

**Step 1: Create the composable**

```typescript
import { ref, onMounted, computed } from 'vue'
import { crawlerApi, publisherApi, indexManagerApi } from '@/api/client'
import { detectProblems } from '../problems/rules'
import type {
  PipelineMetrics,
  CrawlerMetrics,
  IndexMetrics,
  PublisherMetrics,
  SourceMetrics,
  Problem,
} from '../problems/types'
import type { SourceHealthResponse } from '@/types/aggregation'

export function usePipelineHealth() {
  const metrics = ref<PipelineMetrics>({ crawler: null, indexes: null, publisher: null })
  const loading = ref(true)
  const problems = computed<Problem[]>(() => detectProblems(metrics.value))

  async function fetchCrawlerMetrics(): Promise<CrawlerMetrics | null> {
    try {
      const [statusRes, failedRes] = await Promise.all([
        crawlerApi.jobs.statusCounts(),
        crawlerApi.jobs.list({ status: 'failed', limit: 100 }),
      ])
      const counts = statusRes.data as Record<string, number>
      const failedJobs = failedRes.data?.jobs ?? []
      // Stale = scheduled but next_run_at in the past
      const now = new Date()
      const staleJobs = failedJobs.filter((j: { next_run_at?: string }) => {
        if (!j.next_run_at) return false
        return new Date(j.next_run_at) < now
      }).length

      return {
        failedJobs: counts.failed ?? 0,
        staleJobs,
        failedJobUrls: failedJobs.map((j: { url: string }) => j.url),
      }
    } catch {
      return null
    }
  }

  async function fetchIndexMetrics(): Promise<IndexMetrics | null> {
    try {
      const [statsRes, healthRes] = await Promise.all([
        indexManagerApi.aggregations.getSourceHealth(),
        indexManagerApi.stats.get(),
      ])
      const healthData = healthRes.data as { cluster_health?: string }
      const sourceHealthData = statsRes.data as SourceHealthResponse

      const sources: SourceMetrics[] = (sourceHealthData.sources ?? []).map((s) => ({
        source: s.source,
        rawCount: s.raw_count,
        classifiedCount: s.classified_count,
        backlog: s.backlog,
        delta24h: s.delta_24h,
        avgQuality: s.avg_quality,
        active: true, // index-manager doesn't know active status; enriched by caller if needed
      }))

      return {
        clusterHealth: (healthData.cluster_health as 'green' | 'yellow' | 'red') ?? 'green',
        sources,
      }
    } catch {
      return null
    }
  }

  async function fetchPublisherMetrics(): Promise<PublisherMetrics | null> {
    try {
      const [statsRes, channelsRes] = await Promise.all([
        publisherApi.stats.overview('today'),
        publisherApi.channels.list(),
      ])
      const stats = statsRes.data
      const channels = channelsRes.data?.channels ?? []
      const inactive = channels.filter((c) => !c.enabled)

      return {
        publishedToday: stats?.total_articles ?? 0,
        inactiveChannels: inactive.length,
        inactiveChannelNames: inactive.map((c) => c.name),
      }
    } catch {
      return null
    }
  }

  async function fetch() {
    loading.value = true
    const [crawler, indexes, publisher] = await Promise.all([
      fetchCrawlerMetrics(),
      fetchIndexMetrics(),
      fetchPublisherMetrics(),
    ])
    metrics.value = { crawler, indexes, publisher }
    loading.value = false
  }

  onMounted(() => {
    fetch()
  })

  return { metrics, loading, problems, refresh: fetch }
}
```

**Step 2: Update the composables barrel export**

In `dashboard/src/features/intelligence/composables/index.ts`, add:

```typescript
export { usePipelineHealth } from './usePipelineHealth'
```

**Step 3: Lint**

Run: `cd dashboard && npm run lint`
Expected: No errors

**Step 4: Commit**

```bash
git add dashboard/src/features/intelligence/composables/
git commit -m "feat(dashboard): add usePipelineHealth composable

Fetches metrics from crawler, index-manager, and publisher in parallel.
Null-safe: service failures become system problems, not crashes."
```

---

## Task 11: Create PipelineKPIs Component

**Files:**
- Create: `dashboard/src/features/intelligence/components/PipelineKPIs.vue`

**Step 1: Create the component**

```vue
<script setup lang="ts">
import { computed } from 'vue'
import type { PipelineMetrics } from '../problems/types'

const props = defineProps<{
  metrics: PipelineMetrics
}>()

const crawled24h = computed(() => {
  if (!props.metrics.indexes) return 0
  return props.metrics.indexes.sources.reduce((sum, s) => sum + s.delta24h, 0)
})

const classified24h = computed(() => {
  if (!props.metrics.indexes) return 0
  return props.metrics.indexes.sources.reduce((sum, s) => sum + s.delta24h, 0)
})

const published24h = computed(() => props.metrics.publisher?.publishedToday ?? 0)
const failedJobs = computed(() => props.metrics.crawler?.failedJobs ?? 0)

const emptyIndexes = computed(() => {
  if (!props.metrics.indexes) return 0
  return props.metrics.indexes.sources.filter((s) => s.active && s.classifiedCount === 0).length
})

const pipelineYield = computed(() => {
  const crawled = crawled24h.value
  if (crawled === 0) return null
  return Math.round((published24h.value / crawled) * 100)
})

interface KPI {
  label: string
  value: string
  highlight: 'normal' | 'red' | 'amber'
  visible: boolean
}

const kpis = computed<KPI[]>(() => [
  {
    label: 'Crawled (24h)',
    value: crawled24h.value.toLocaleString(),
    highlight: crawled24h.value === 0 ? 'red' : 'normal',
    visible: true,
  },
  {
    label: 'Classified (24h)',
    value: classified24h.value.toLocaleString(),
    highlight: classified24h.value === 0 ? 'red' : 'normal',
    visible: true,
  },
  {
    label: 'Published (24h)',
    value: published24h.value.toLocaleString(),
    highlight: published24h.value === 0 ? 'red' : 'normal',
    visible: true,
  },
  {
    label: 'Failed Jobs',
    value: failedJobs.value.toLocaleString(),
    highlight: 'red',
    visible: failedJobs.value > 0,
  },
  {
    label: 'Empty Indexes',
    value: emptyIndexes.value.toLocaleString(),
    highlight: 'amber',
    visible: emptyIndexes.value > 0,
  },
  {
    label: 'Pipeline Yield',
    value: pipelineYield.value !== null ? `${pipelineYield.value}%` : '-',
    highlight: pipelineYield.value !== null && pipelineYield.value < 10 ? 'amber' : 'normal',
    visible: pipelineYield.value !== null,
  },
])

const visibleKpis = computed(() => kpis.value.filter((k) => k.visible))
</script>

<template>
  <div class="grid gap-3 grid-cols-3 lg:grid-cols-6">
    <div
      v-for="kpi in visibleKpis"
      :key="kpi.label"
      class="rounded-lg border bg-card px-4 py-3"
    >
      <p class="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
        {{ kpi.label }}
      </p>
      <p
        class="text-xl font-semibold tabular-nums mt-0.5"
        :class="{
          'text-red-500': kpi.highlight === 'red',
          'text-amber-500': kpi.highlight === 'amber',
        }"
      >
        {{ kpi.value }}
      </p>
    </div>
  </div>
</template>
```

**Step 2: Lint**

Run: `cd dashboard && npm run lint`

**Step 3: Commit**

```bash
git add dashboard/src/features/intelligence/components/PipelineKPIs.vue
git commit -m "feat(dashboard): add PipelineKPIs component

Horizontal strip of 4-6 key metrics. Failed jobs and empty indexes
only appear when non-zero. Values highlight red or amber on problems."
```

---

## Task 12: Create SourceHealthTable Component

**Files:**
- Create: `dashboard/src/features/intelligence/components/SourceHealthTable.vue`

**Step 1: Create the component**

```vue
<script setup lang="ts">
import { ref, computed } from 'vue'
import type { SourceMetrics } from '../problems/types'
import { Badge } from '@/components/ui/badge'

const props = defineProps<{
  sources: SourceMetrics[]
}>()

type ViewMode = 'ops' | 'dev'
type QuickFilter = 'all' | 'errors' | 'warnings' | 'no-docs' | 'backlog'

const viewMode = ref<ViewMode>(
  (localStorage.getItem('intelligence-view-mode') as ViewMode) ?? 'ops'
)
const quickFilter = ref<QuickFilter>('all')

function setViewMode(mode: ViewMode) {
  viewMode.value = mode
  localStorage.setItem('intelligence-view-mode', mode)
}

function getStatus(source: SourceMetrics): 'error' | 'warning' | 'healthy' {
  if (!source.active) return 'healthy'
  if (source.classifiedCount === 0) return 'error'
  if (source.backlog > 100) return 'warning'
  if (source.delta24h === 0 && source.classifiedCount > 0) return 'warning'
  return 'healthy'
}

const filteredSources = computed(() => {
  let result = [...props.sources]

  switch (quickFilter.value) {
    case 'errors':
      result = result.filter((s) => getStatus(s) === 'error')
      break
    case 'warnings':
      result = result.filter((s) => getStatus(s) === 'warning')
      break
    case 'no-docs':
      result = result.filter((s) => s.active && s.classifiedCount === 0)
      break
    case 'backlog':
      result = result.filter((s) => s.backlog > 0)
      break
  }

  // Sort: errors first, then warnings, then healthy
  result.sort((a, b) => {
    const order = { error: 0, warning: 1, healthy: 2 }
    return order[getStatus(a)] - order[getStatus(b)]
  })

  return result
})

const filters: { key: QuickFilter; label: string }[] = [
  { key: 'all', label: 'All' },
  { key: 'errors', label: 'Errors' },
  { key: 'warnings', label: 'Warnings' },
  { key: 'no-docs', label: 'No docs' },
  { key: 'backlog', label: 'Backlog' },
]

const statusDot: Record<string, string> = {
  error: 'bg-red-500',
  warning: 'bg-amber-500',
  healthy: 'bg-emerald-500',
}
</script>

<template>
  <div class="space-y-3">
    <!-- Controls -->
    <div class="flex items-center justify-between gap-3">
      <div class="flex gap-1.5">
        <button
          v-for="f in filters"
          :key="f.key"
          class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
          :class="
            quickFilter === f.key
              ? 'bg-primary text-primary-foreground'
              : 'bg-muted text-muted-foreground hover:bg-muted/80'
          "
          @click="quickFilter = f.key"
        >
          {{ f.label }}
        </button>
      </div>
      <div class="flex gap-1.5">
        <button
          class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
          :class="viewMode === 'ops' ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'"
          @click="setViewMode('ops')"
        >
          Ops
        </button>
        <button
          class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
          :class="viewMode === 'dev' ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'"
          @click="setViewMode('dev')"
        >
          Dev
        </button>
      </div>
    </div>

    <!-- Table -->
    <div class="rounded-lg border overflow-hidden">
      <table class="w-full text-sm">
        <thead class="border-b bg-muted/50">
          <tr>
            <th class="px-3 py-2 text-left font-medium text-muted-foreground">Source</th>
            <th class="px-3 py-2 text-left font-medium text-muted-foreground">Status</th>
            <!-- Ops columns -->
            <template v-if="viewMode === 'ops'">
              <th class="px-3 py-2 text-right font-medium text-muted-foreground">Raw</th>
              <th class="px-3 py-2 text-right font-medium text-muted-foreground">Classified</th>
              <th class="px-3 py-2 text-right font-medium text-muted-foreground">Backlog</th>
            </template>
            <!-- Dev columns -->
            <template v-else>
              <th class="px-3 py-2 text-right font-medium text-muted-foreground">Classified</th>
              <th class="px-3 py-2 text-right font-medium text-muted-foreground">Avg Quality</th>
              <th class="px-3 py-2 text-right font-medium text-muted-foreground">24h Delta</th>
            </template>
          </tr>
        </thead>
        <tbody class="divide-y">
          <tr
            v-for="source in filteredSources"
            :key="source.source"
            :class="{ 'opacity-50': !source.active }"
          >
            <td class="px-3 py-2 font-mono text-xs">
              {{ source.source.replaceAll('_', '.') }}
            </td>
            <td class="px-3 py-2">
              <span class="inline-block h-2 w-2 rounded-full" :class="statusDot[getStatus(source)]" />
            </td>
            <!-- Ops columns -->
            <template v-if="viewMode === 'ops'">
              <td class="px-3 py-2 text-right tabular-nums">{{ source.rawCount.toLocaleString() }}</td>
              <td class="px-3 py-2 text-right tabular-nums">{{ source.classifiedCount.toLocaleString() }}</td>
              <td class="px-3 py-2 text-right tabular-nums" :class="{ 'text-amber-500': source.backlog > 0 }">
                {{ source.backlog > 0 ? source.backlog.toLocaleString() : '-' }}
              </td>
            </template>
            <!-- Dev columns -->
            <template v-else>
              <td class="px-3 py-2 text-right tabular-nums">{{ source.classifiedCount.toLocaleString() }}</td>
              <td class="px-3 py-2 text-right tabular-nums">
                <Badge
                  v-if="source.avgQuality > 0"
                  :variant="source.avgQuality >= 70 ? 'success' : source.avgQuality >= 40 ? 'warning' : 'destructive'"
                >
                  {{ Math.round(source.avgQuality) }}
                </Badge>
                <span v-else class="text-muted-foreground">-</span>
              </td>
              <td class="px-3 py-2 text-right tabular-nums" :class="{ 'text-amber-500': source.delta24h === 0 && source.classifiedCount > 0 }">
                {{ source.delta24h > 0 ? `+${source.delta24h.toLocaleString()}` : source.delta24h === 0 && source.classifiedCount > 0 ? 'stale' : '-' }}
              </td>
            </template>
          </tr>
          <tr v-if="filteredSources.length === 0">
            <td :colspan="viewMode === 'ops' ? 5 : 5" class="px-3 py-8 text-center text-sm text-muted-foreground">
              No sources match the current filter.
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <p class="text-xs text-muted-foreground">
      {{ filteredSources.length }} of {{ sources.length }} sources
    </p>
  </div>
</template>
```

**Step 2: Lint**

Run: `cd dashboard && npm run lint`

**Step 3: Commit**

```bash
git add dashboard/src/features/intelligence/components/SourceHealthTable.vue
git commit -m "feat(dashboard): add SourceHealthTable component

Per-source table with Ops/Dev view toggle, quick filters (errors,
warnings, no docs, backlog), and status-sorted rows."
```

---

## Task 13: Create ContentSummaryCards Component

**Files:**
- Create: `dashboard/src/features/intelligence/components/ContentSummaryCards.vue`

**Step 1: Create the component**

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { AlertTriangle, Pickaxe, MapPin, BarChart3 } from 'lucide-vue-next'
import { Card, CardContent } from '@/components/ui/card'
import { indexManagerApi } from '@/api/client'

const router = useRouter()

interface SummaryCard {
  label: string
  icon: typeof AlertTriangle
  route?: string
  value: string
  sub: string
}

const cards = ref<SummaryCard[]>([
  { label: 'Crime', icon: AlertTriangle, route: '/intelligence/crime', value: '-', sub: '' },
  { label: 'Mining', icon: Pickaxe, route: '/intelligence/mining', value: '-', sub: '' },
  { label: 'Quality', icon: BarChart3, value: '-', sub: '' },
  { label: 'Location', icon: MapPin, route: '/intelligence/location', value: '-', sub: '' },
])

onMounted(async () => {
  const [crimeRes, miningRes, overviewRes, locationRes] = await Promise.allSettled([
    indexManagerApi.aggregations.getCrime(),
    indexManagerApi.aggregations.getMining(),
    indexManagerApi.aggregations.getOverview(),
    indexManagerApi.aggregations.getLocation(),
  ])

  if (crimeRes.status === 'fulfilled') {
    const d = crimeRes.value.data
    cards.value[0].value = (d?.total_crime_related ?? 0).toLocaleString()
    const top = Object.entries(d?.by_sub_label ?? {}).sort(([, a], [, b]) => b - a).slice(0, 3)
    cards.value[0].sub = top.map(([k]) => k.replace(/_/g, ' ')).join(', ')
  }

  if (miningRes.status === 'fulfilled') {
    const d = miningRes.value.data
    cards.value[1].value = (d?.total_mining ?? 0).toLocaleString()
    const top = Object.entries(d?.by_commodity ?? {}).sort(([, a], [, b]) => b - a).slice(0, 3)
    cards.value[1].sub = top.map(([k]) => k).join(', ')
  }

  if (overviewRes.status === 'fulfilled') {
    const d = overviewRes.value.data
    const q = d?.quality_distribution
    if (q) {
      const total = (q.high ?? 0) + (q.medium ?? 0) + (q.low ?? 0)
      cards.value[2].value = total.toLocaleString()
      cards.value[2].sub = `${q.high ?? 0} high / ${q.medium ?? 0} med / ${q.low ?? 0} low`
    }
  }

  if (locationRes.status === 'fulfilled') {
    const d = locationRes.value.data
    const top = Object.entries(d?.by_city ?? {}).sort(([, a], [, b]) => b - a).slice(0, 3)
    cards.value[3].value = top.length > 0 ? top[0][1].toLocaleString() : '-'
    cards.value[3].sub = top.map(([k]) => k).join(', ')
  }
})

function goTo(route?: string) {
  if (route) router.push(route)
}
</script>

<template>
  <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
    <Card
      v-for="card in cards"
      :key="card.label"
      class="transition-colors"
      :class="card.route ? 'cursor-pointer hover:bg-muted/50' : ''"
      @click="goTo(card.route)"
    >
      <CardContent class="pt-4 pb-3 px-4">
        <div class="flex items-center gap-2 mb-1">
          <component :is="card.icon" class="h-4 w-4 text-muted-foreground shrink-0" />
          <span class="text-xs font-medium uppercase tracking-wider text-muted-foreground">
            {{ card.label }}
          </span>
        </div>
        <p class="text-lg font-semibold tabular-nums">{{ card.value }}</p>
        <p v-if="card.sub" class="text-xs text-muted-foreground truncate mt-0.5">
          {{ card.sub }}
        </p>
      </CardContent>
    </Card>
  </div>
</template>
```

**Step 2: Lint**

Run: `cd dashboard && npm run lint`

**Step 3: Commit**

```bash
git add dashboard/src/features/intelligence/components/ContentSummaryCards.vue
git commit -m "feat(dashboard): add ContentSummaryCards component

Compact row of 4 cards showing crime, mining, quality, and location
summaries with top-3 sub-values. Links to drill-down pages."
```

---

## Task 14: Rewrite IntelligenceOverviewView

**Files:**
- Modify: `dashboard/src/views/intelligence/IntelligenceOverviewView.vue` (rewrite)

**Step 1: Rewrite the view**

Replace the entire contents of `IntelligenceOverviewView.vue`:

```vue
<script setup lang="ts">
import { Loader2, RefreshCw } from 'lucide-vue-next'
import { usePipelineHealth } from '@/features/intelligence/composables/usePipelineHealth'
import ProblemsBanner from '@/features/intelligence/problems/ProblemsBanner.vue'
import PipelineKPIs from '@/features/intelligence/components/PipelineKPIs.vue'
import SourceHealthTable from '@/features/intelligence/components/SourceHealthTable.vue'
import ContentSummaryCards from '@/features/intelligence/components/ContentSummaryCards.vue'

const { metrics, loading, problems, refresh } = usePipelineHealth()
</script>

<template>
  <div class="space-y-6 animate-fade-up">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-semibold tracking-tight">Intelligence</h1>
        <p class="mt-0.5 text-sm text-muted-foreground">
          Pipeline health and content intelligence.
        </p>
      </div>
      <button
        class="inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium bg-muted hover:bg-muted/80 transition-colors"
        :disabled="loading"
        @click="refresh"
      >
        <RefreshCw class="h-3.5 w-3.5" :class="{ 'animate-spin': loading }" />
        Refresh
      </button>
    </div>

    <!-- Loading state -->
    <div v-if="loading && !metrics.indexes" class="flex items-center justify-center py-16">
      <Loader2 class="h-6 w-6 animate-spin text-muted-foreground" />
    </div>

    <template v-else>
      <!-- Problems Banner -->
      <ProblemsBanner :problems="problems" />

      <!-- Pipeline KPIs -->
      <PipelineKPIs :metrics="metrics" />

      <!-- Source Health Table -->
      <div>
        <h2 class="text-sm font-medium uppercase tracking-wider text-muted-foreground mb-3">
          Source Health
        </h2>
        <SourceHealthTable :sources="metrics.indexes?.sources ?? []" />
      </div>

      <!-- Content Intelligence -->
      <div>
        <h2 class="text-sm font-medium uppercase tracking-wider text-muted-foreground mb-3">
          Content Intelligence
        </h2>
        <ContentSummaryCards />
      </div>
    </template>
  </div>
</template>
```

**Step 2: Lint**

Run: `cd dashboard && npm run lint`

**Step 3: Commit**

```bash
git add dashboard/src/views/intelligence/IntelligenceOverviewView.vue
git commit -m "feat(dashboard): rewrite Intelligence overview as unified dashboard

Replaces card-based navigation with: problems banner, pipeline KPIs,
source health table, and content summary cards."
```

---

## Task 15: Delete Old Files and Clean Up Imports

**Files:**
- Delete: `dashboard/src/components/intelligence/ContentIntelligenceSummary.vue`
- Delete: `dashboard/src/components/intelligence/index.ts`
- Delete: `dashboard/src/composables/useIntelligenceOverview.ts`
- Delete: `dashboard/src/config/intelligence.ts`
- Modify: `dashboard/src/composables/index.ts` (remove useIntelligenceOverview export)

**Step 1: Remove useIntelligenceOverview from composables barrel**

In `dashboard/src/composables/index.ts`, delete lines 24-27:

```typescript
// DELETE these lines:
export {
  useIntelligenceOverview,
  type IntelligenceOverviewData,
} from './useIntelligenceOverview'
```

**Step 2: Delete the old files**

Run:
```bash
rm dashboard/src/components/intelligence/ContentIntelligenceSummary.vue
rm dashboard/src/components/intelligence/index.ts
rm dashboard/src/composables/useIntelligenceOverview.ts
rm dashboard/src/config/intelligence.ts
```

If the `dashboard/src/components/intelligence/` directory is now empty, remove it:
```bash
rmdir dashboard/src/components/intelligence/
```

**Step 3: Search for any remaining imports of deleted files**

Run: `grep -r "useIntelligenceOverview\|ContentIntelligenceSummary\|config/intelligence" dashboard/src/`
Expected: No results (all imports should be gone since IntelligenceOverviewView was rewritten)

If any stale imports remain, fix them.

**Step 4: Lint and build**

Run: `cd dashboard && npm run lint && npm run build`
Expected: Both pass with no errors. The build step ensures there are no broken imports.

**Step 5: Run tests**

Run: `cd dashboard && npm run test`
Expected: All problem detection tests still pass.

**Step 6: Commit**

```bash
git add -A dashboard/src/components/intelligence/ dashboard/src/composables/ dashboard/src/config/
git commit -m "refactor(dashboard): remove old intelligence overview components

Delete ContentIntelligenceSummary, useIntelligenceOverview,
intelligence config. All replaced by unified dashboard."
```

---

## Task 16: Update Feature Barrel Export

**Files:**
- Modify: `dashboard/src/features/intelligence/index.ts`

**Step 1: Add new exports**

Update the barrel export to include new components:

```typescript
// API & data
export { fetchIndexes, fetchIndexStats, deleteIndex, indexesKeys } from './api/indexes'

// Composables
export { useIndexes } from './composables/useIndexes'
export { usePipelineHealth } from './composables/usePipelineHealth'

// Problem detection
export { detectProblems } from './problems/rules'
export type { Problem, PipelineMetrics } from './problems/types'

// Components
export { default as IndexesFilterBar } from './components/IndexesFilterBar.vue'
export { default as IndexesTable } from './components/IndexesTable.vue'
export { default as IndexStatsCards } from './components/IndexStatsCards.vue'
export { default as PipelineKPIs } from './components/PipelineKPIs.vue'
export { default as SourceHealthTable } from './components/SourceHealthTable.vue'
export { default as ContentSummaryCards } from './components/ContentSummaryCards.vue'
```

**Step 2: Lint and build**

Run: `cd dashboard && npm run lint && npm run build`

**Step 3: Commit**

```bash
git add dashboard/src/features/intelligence/index.ts
git commit -m "refactor(dashboard): update intelligence feature barrel exports"
```

---

## Task 17: Deploy and Verify Backend

**Step 1: Build and deploy index-manager**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d --build index-manager`

**Step 2: Test the new endpoint**

Use the MCP tool to verify:

```
Call: mcp__North_Cloud__Local___get_auth_token
Then: curl http://localhost:8090/api/v1/aggregations/source-health -H "Authorization: Bearer $TOKEN"
```

Expected: JSON response with `sources` array containing per-source health data.

**Step 3: Build and deploy MCP**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d --build mcp-north-cloud`

**Step 4: Verify MCP list_sources fix**

Call: `mcp__North_Cloud__Local___list_sources`
Expected: Returns sources array without unmarshal error.

---

## Task 18: Build and Verify Dashboard

**Step 1: Build the dashboard**

Run: `cd dashboard && npm run build`
Expected: Builds successfully with no errors.

**Step 2: Deploy**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d --build dashboard`

**Step 3: Verify in browser**

Navigate to `/dashboard/intelligence` and verify:
- Problems banner shows detected issues (failed crawls, inactive channels, zero publishing)
- KPI strip shows crawled/classified/published counts
- Source health table renders with data
- Ops/Dev view toggle works
- Quick filters work
- Content summary cards show crime/mining/quality/location
- Drill-down links still work (click Crime -> goes to /intelligence/crime)

---

## Notes

### Out of Scope (separate work)

1. **Publisher routes endpoints**: The `routes` table was dropped in migration 003_routing_v2. The MCP's `list_routes`, `create_route`, `delete_route`, `preview_route` tools call dead endpoints. This needs a separate decision: either re-implement routes or remove the stale MCP tools. Not blocking the dashboard.

2. **Operational fixes**: Investigating the 18 failed crawl jobs, inactive channels, and zero publishing. The new dashboard will make these visible; fixing them is operational work, not code changes.

3. **Crawler last-error tooltip**: The source-health endpoint returns ES data only. Enriching with crawler execution errors requires joining data from the crawler API client-side. This can be added as a follow-up by expanding `usePipelineHealth` to also fetch from `crawlerApi.jobs.list()` and joining by source URL.
