# Pipeline Service Integration Design

**Date**: 2026-02-12
**Status**: Approved

## Problem

The pipeline observability service exists with a complete API, database schema, and fire-and-forget client library, but no service emits events to it. This integration wires the existing `infrastructure/pipeline.Client` into the crawler, classifier, and publisher.

## Design Decisions

1. **3 stages, not 5**: Emit `indexed`, `classified`, `published`. Skip `crawled` (same function chain as indexed) and `routed` (internal step before publishing).
2. **One event per article**: The publisher emits a single `published` event with all matched channels in metadata, not one event per channel.
3. **Inline fire-and-forget**: Events are emitted synchronously. The client has a 2s timeout and circuit breaker. On error, log a warning and continue.
4. **Opt-in via env var**: `PIPELINE_URL` empty or unset = disabled (client no-ops).

## Integration Points

| Stage | Service | File | Function | After |
|-------|---------|------|----------|-------|
| `indexed` | Crawler | `content/rawcontent/service.go` | `Process()` | `IndexRawContent()` succeeds |
| `classified` | Classifier | `processor/poller.go` | `indexResults()` | `BulkIndexClassifiedContent()` + status update |
| `published` | Publisher | `router/service.go` | `routeArticle()` | All layers processed, channels collected |

## Metadata Per Stage

**indexed**: `{title, word_count, index_name, document_id}`
**classified**: `{quality_score, topics, content_type}`
**published**: `{channels, quality_score, topics}`

## Changes Per Service

Each service follows the same 3-file pattern:

1. **Config** (`config.go`): Add `PipelineURL string` field with `PIPELINE_URL` env tag
2. **Bootstrap**: Create `pipeline.NewClient(url, serviceName)`, inject into component
3. **Emission point**: Call `client.Emit(ctx, event)`, log warning on error

**Docker Compose**: Add `PIPELINE_URL: http://pipeline:8075` to crawler, classifier, publisher in both dev and prod compose files.

## What Doesn't Change

- Pipeline service (already complete)
- Infrastructure client library (already has circuit breaker)
- Dashboard (composite health view works independently; funnel widget is future work)
