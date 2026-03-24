# Classifier Quality Gate Design

**Date**: 2026-03-24
**Closes**: #564, #565, #566

## Problem

Multiple sources are indexing non-article content (nav pages, share links, event listings, external site homepages) that the classifier identifies as low confidence but still indexes to `*_classified_content`. This inflates index sizes and reduces classifier metrics.

Evidence:
- Basque Tribune: 36% page content, 56% borderline confidence zone, external domains indexed (wa.me, fever.co)
- Battlefords News-Optimist: 14% pages + 11% events, avg confidence 0.552, 17% very-low zone

## Design

### Change 1: Quality Gate in Classifier Pipeline (#566, #565)

**Location**: `classifier/internal/processor/poller.go` — between batch classification and the `BulkIndexClassifiedContent` call. The gate filters the classified results before they reach ES.

**Gate logic**:
```
quality_score >= threshold (inclusive)                → index normally
quality_score < threshold AND content_type=article  → index with low_quality=true
quality_score < threshold AND content_type!=article → reject (log, skip indexing)
```

**Config additions** (`classifier/internal/config/config.go`):
- `CLASSIFIER_QUALITY_GATE_ENABLED` (bool, default `false`) — feature flag for safe rollout
- `CLASSIFIER_QUALITY_GATE_THRESHOLD` (int, default `40`) — minimum quality_score

**Domain model** (`classifier/internal/domain/classification.go`):
- Add `LowQuality bool` field to `ClassifiedContent` struct (JSON: `low_quality`, ES field: `low_quality`)

**Observability**: Rejected documents logged at `info` level with structured fields: `source`, `content_type`, `quality_score`, `url`, `reason`.

**Metrics tracking**: Add counters to the existing poller stats:
- `quality_gate_passed` — docs that passed the gate
- `quality_gate_flagged` — articles indexed with low_quality=true
- `quality_gate_rejected` — non-articles rejected

### Change 2: Basque Tribune Source Fix (#564)

**Root cause**: The Basque Tribune source (`naiz.eus/en`) crawler is following outbound links to external domains despite AllowedDomains auto-population. The frontier queue may be accepting external URLs before Colly's domain filter applies.

**Fix**: Update the Basque Tribune source config via source-manager API to explicitly set `allowed_domains: ["naiz.eus", "www.naiz.eus"]`. Verify via API that the restriction is in effect.

**Verification**: After update, confirm no new external-domain documents appear in the index.

### Change 3: ES Mapping for low_quality Field

Add `low_quality` as a `boolean` field to the classifier's ES mapping template so it's queryable. Dynamic mapping would handle this, but explicit is better for consistency.

**Location**: `classifier/internal/elasticsearch/mappings/` — add to classified_content template.

## Implementation Order

1. Add `LowQuality` field to domain model + ES mapping
2. Add config env vars (`CLASSIFIER_QUALITY_GATE_ENABLED`, `CLASSIFIER_QUALITY_GATE_THRESHOLD`)
3. Implement gate logic in classifier pipeline (between classify and index)
4. Add logging and counter metrics
5. Tests: unit tests for gate logic, integration test for reject/flag/pass paths
6. Update Basque Tribune source config via API
7. Update docker-compose env vars (gate disabled by default)
8. Update classifier CLAUDE.md with quality gate docs

## Rollout

1. Deploy with `CLASSIFIER_QUALITY_GATE_ENABLED=false` (no behavior change)
2. Enable on staging/dev, monitor rejection logs
3. Enable on prod, verify index sizes stabilize
4. Adjust threshold if needed via `CLASSIFIER_QUALITY_GATE_THRESHOLD`

## Not In Scope

- Retroactive cleanup of existing junk in indices (separate task)
- Publisher-side filtering on `low_quality` flag (future enhancement)
- Per-source threshold overrides (YAGNI for now)
