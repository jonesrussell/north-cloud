# AI Observer Source Suppression & Minimum Sample Size

**Date**: 2026-03-20
**Status**: Approved
**Issues**: Closes #491 (working as designed), Closes #492 (working as designed)

## Problem

The AI Observer generates recurring medium-severity insights for sources that are genuinely low-confidence by design. Battlefords News-Optimist consistently triggers alerts with 100% borderline rate because:

- Thin wire stories (150-200 words, quality score ~55 vs population ~78)
- 52% of articles have no topic matches (topic confidence falls to 0.3 minimum)
- Average confidence ~0.53, well below the 0.6 borderline threshold
- 68% of articles fall below borderline threshold (vs 2.4% for Global News)

The classifier is correct — these are genuinely low-signal articles outside the domain focus. The Observer is also correct to detect the anomaly. But the insights are not actionable and create noise.

Additionally, domains with 1-2 documents in a sample window can trigger medium-severity insights despite insufficient statistical evidence.

## Decision

**Result-time exclusion**: Suppressed sources remain in ES queries and population stats (preserving statistical integrity), but the analyzer filters them from the domain stats sent to the LLM. This follows the same pattern as Prometheus alertmanager, Datadog monitors, and other production anomaly detection systems.

## Design

### Config Layer

Two new environment variables in `bootstrap/config.go`:

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `AI_OBSERVER_SUPPRESSED_SOURCES` | comma-separated string | `""` (empty) | Source names excluded from insight generation. Stored as `map[string]bool`. |
| `AI_OBSERVER_MIN_DOMAIN_SAMPLES` | int | `5` | Minimum docs per domain+label pair to include in LLM analysis. |

Both are added to the existing config struct and passed through the classifier category constructor to the analyzer.

The `minSamples` default of 5 is chosen because below 5 samples, a single borderline document causes a 20%+ swing in borderline rate, making the metric unreliable for insight generation.

### Constructor Changes

`category.go` constructor gains two parameters, stored on the `Category` struct:

```go
func New(esClient *es.Client, maxEvents int, modelTier string, suppressedSources map[string]bool, minDomainSamples int) *Category
```

These are passed through to `analyze()` which gains the same parameters:

```go
func (c *Category) analyze(events []category.Event, pop populationStats) ([]category.Insight, error)
// analyze() reads c.suppressedSources and c.minDomainSamples internally
```

### Analyzer Filtering

New function in `analyzer.go`, called after `aggregateStats()` and before LLM prompt construction:

```go
// filterDomainStats removes suppressed sources and low-sample pairs from the
// domain stats sent to the LLM. Note: domainStats.Domain corresponds to
// source_name in Elasticsearch (not URL domain).
func filterDomainStats(stats []domainStats, suppressed map[string]bool, minSamples int) []domainStats {
    filtered := make([]domainStats, 0, len(stats))
    for _, s := range stats {
        if suppressed[s.Domain] {
            continue
        }
        if s.Count < minSamples {
            continue
        }
        filtered = append(filtered, s)
    }
    return filtered
}
```

If filtering removes all domain stats, skip the LLM call entirely (no insights this cycle). Log filtered counts with structured fields:

```go
log.Info("filtered domain stats",
    infralogger.Int("suppressed_count", suppressedCount),
    infralogger.Int("below_min_samples", belowMinCount),
    infralogger.Int("remaining", len(filtered)),
)
```

### Data Flow

```
Sampler (unchanged)
  ├── Queries *_classified_content (all sources, including suppressed)
  └── Returns []Event + PopulationStats (truthful, all sources)

Analyzer
  ├── aggregateStats(events) → []domainStats (all sources)
  ├── filterDomainStats(stats, suppressed, minSamples) → []domainStats (filtered)  ← NEW
  ├── If len(filtered) == 0 → return nil (no LLM call)
  ├── Build LLM prompt with filtered stats + unchanged PopulationStats
  └── Return []Insight

Writer (unchanged)
  └── Dedup + write to ai_insights index
```

**Key invariant**: `PopulationStats` always reflects the full dataset. Only per-domain breakdown is filtered.

### Files Changed

1. `ai-observer/internal/bootstrap/config.go` — add 2 env vars to config struct
2. `ai-observer/internal/category/classifier/category.go` — update `New()` signature, store suppression config on struct
3. `ai-observer/internal/category/classifier/analyzer.go` — add `filterDomainStats()`, call before LLM prompt
4. `ai-observer/config.yml.example` — document new options
5. `ai-observer/CLAUDE.md` — update config table with new env vars

### Tests

- Unit test `filterDomainStats`: suppressed sources excluded, low-sample pairs excluded, empty result returns nil
- Unit test: population stats unchanged when sources are suppressed
- Integration test: verify no insights generated for suppressed domains

## What This Does NOT Change

- Sampler ES queries (all sources still queried)
- Population stats (all sources contribute)
- Writer/dedup logic
- Drift detection (separate category, unaffected — if suppressed sources also trigger drift alerts, extend suppression there in a future iteration)
- Classifier service (no changes)
