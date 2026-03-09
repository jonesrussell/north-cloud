# M-Indigenous-Backfill: Global Indigenous Re-Crawl

**Date**: 2026-03-14
**Status**: Proposed
**Milestone**: M-Indigenous-Backfill

## Purpose

Activate global indigenous content ingestion by re-crawling all 186 indigenous media
sources onboarded in M-Indigenous-Sources. These sources have `indigenous_region` set
but have not yet been crawled — this backfill triggers initial crawl jobs so content
flows through the full pipeline (crawler → classifier → publisher → Redis channels).

## Scope

All sources in the `sources` table where `indigenous_region IS NOT NULL` and `enabled = true`.

### Affected Services

| Service | Change |
|---------|--------|
| source-manager | New `GET /api/v1/sources/indigenous` endpoint to list indigenous sources with optional region filter |
| crawler | New `POST /api/v1/backfill/indigenous` endpoint to trigger staggered re-crawl jobs |
| publisher | Indigenous backfill metrics counters for monitoring routing throughput |

## Crawl Strategy

### Staggered Batches, Region-by-Region

To avoid overwhelming the crawler, proxy pool, and target sites:

1. **Region ordering**: canada → us → latin_america → oceania → europe → asia → africa
2. **Batch size**: 25 sources per batch (configurable via `--limit` / request param)
3. **Inter-batch delay**: 60 seconds between batches (allows rate limiters to reset)
4. **Dry-run mode**: Preview which sources would be crawled without dispatching jobs

### API Endpoint

```
POST /api/v1/backfill/indigenous
  ?region=canada        # Optional: filter to single region
  &limit=25             # Optional: max sources per batch (default: all)
  &dry_run=true         # Optional: preview without dispatching
```

Response:
```json
{
  "sources_found": 44,
  "jobs_dispatched": 44,
  "region": "canada",
  "dry_run": false,
  "sources": [
    {"id": "uuid", "name": "APTN News", "region": "canada", "render_mode": "static"}
  ]
}
```

## Render-Mode Considerations

| Mode | Sources | Rate Limit | Max Depth | Notes |
|------|---------|------------|-----------|-------|
| static | ~182 | 10s | 2 | Standard HTTP fetch, low resource cost |
| dynamic | ~4 | 12s | 1 | Playwright rendering, higher CPU/memory |

Dynamic sources (NITV, Māori Television, Te Karere TVNZ, Taiwan Indigenous TV) require
the render-worker container to be running. Monitor render-worker memory during backfill.

## Expected Extraction Metrics

### Baseline (Pre-Backfill)

- Indigenous `raw_content` documents: 0 (new sources, never crawled)
- Indigenous `classified_content` documents: 0
- `content:indigenous` Redis publishes: 0

### Post-Backfill Targets

- Raw content documents: ~5,000–15,000 (varies by source freshness and depth)
- Classified content with `indigenous.relevance != not_indigenous`: ~2,000–8,000
- Region channel distribution: proportional to source count per region
- Expected false-positive rate: < 5% (validated by M-Indigenous-Classifier v3)

## Monitoring Plan

### Elasticsearch Queries

```json
// Count raw content from indigenous sources
GET *_raw_content/_count
{ "query": { "exists": { "field": "meta.indigenous_region" } } }

// Count classified indigenous content by region
GET *_classified_content/_search
{
  "size": 0,
  "query": { "nested": { "path": "indigenous", "query": { "exists": { "field": "indigenous.relevance" } } } },
  "aggs": { "by_region": { "terms": { "field": "meta.indigenous_region.keyword" } } }
}
```

### Grafana Dashboards

- **Crawler**: monitor `crawled_today` and `indexed_today` counters during backfill
- **Publisher**: monitor `indigenous_backfill_total`, `indigenous_backfill_success`, `indigenous_backfill_failed` Redis counters
- **Scheduler metrics**: watch `jobs_running` count and `success_rate` for spikes/drops

### Alerting Thresholds

- Crawler error rate > 20% for any region → pause that region's batch
- Render-worker memory > 2GB → pause dynamic source batch
- Rate limit violations (429 responses) → increase inter-request delay

## Rollback Plan

If backfill causes issues (excessive errors, target site blocking, resource exhaustion):

1. **Pause active jobs**: `POST /api/v1/jobs/:id/pause` for each running backfill job
2. **Disable sources**: Set `enabled = false` for affected `indigenous_region` sources
3. **Cancel remaining batches**: Stop dispatching new backfill jobs
4. **Investigate**: Check crawler logs, ES error indices, proxy Squid logs
5. **Resume selectively**: Re-enable sources one region at a time after fixing issues

No data deletion is needed — raw/classified content documents are additive and harmless.

## Dependencies

| Dependency | Status |
|------------|--------|
| M-Indigenous-Sources (#228) | Merged — 186 sources onboarded |
| M-Indigenous-Classifier (#222) | Merged — multilingual classifier v3 |
| D1: Region Taxonomy (#216) | Merged — shared region validation |
| D2: Category Taxonomy (#218) | Merged — 10 global categories |

## Future Extensions

- **Scheduled re-crawl**: Convert one-time backfill jobs to recurring scheduled jobs
- **Feed health monitoring**: Track RSS feed availability and staleness per source
- **Adaptive scheduling**: Enable adaptive scheduling for indigenous sources based on content freshness
- **Regional dashboards**: Grafana dashboards per region showing content volume and quality
