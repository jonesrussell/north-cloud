# AI Observer Spec

> Last verified: 2026-03-22 (add layer rules to service CLAUDE.md and .layers config)

## Overview

Advisory-only service that periodically samples content from Elasticsearch, sends it to an LLM (Anthropic Claude) for analysis, and writes insights to a dedicated `ai_insights` ES index. Never modifies ingestion pipeline indices. Disabled by default.

---

## File Map

```
ai-observer/
  main.go                          # Calls bootstrap.Start()
  Dockerfile                       # Multi-stage alpine, uid 1000
  internal/
    bootstrap/                     # Config -> logger -> ES -> provider -> categories -> scheduler
    provider/                      # LLMProvider interface + Anthropic implementation
    category/                      # Category interface, Event, Insight types
      classifier/                  # Classifier category (ES sampling + LLM analysis)
      drift/                       # Statistical drift category (KL, PSI, cross-matrix)
    drift/                         # Drift metrics, baseline sampler, evaluator, store
    insights/                      # ai_insights ES index writer + dedup + retention cleanup
    scheduler/                     # Dual-ticker loop + cost-ceiling token budget
```

---

## API Reference

No HTTP API. The service runs as a background scheduler that writes to Elasticsearch.

### Grafana Dashboard

Available at `/d/north-cloud-ai-insights`:
- Overview stats (total insights, severity counts, token usage, error count)
- Trends (insights over time by severity, token usage over time)
- Severity/category/model breakdowns
- Service logs (Loki) and recent insights table (ES)

Datasource: `ai-insights` (uid: `ai-insights`) pointing to `ai_insights` index with `created_at` time field.

---

## Data Model

### ai_insights ES index

| Field | ES Type | Description |
|-------|---------|-------------|
| `category` | keyword | Category of insight (e.g., `classifier_drift`) |
| `severity` | keyword | `info`, `warning`, `critical` |
| `title` | text | Human-readable insight title |
| `description` | text | Detailed insight description |
| `details` | flattened | LLM-generated structured details (inconsistent types, stored as strings) |
| `source_name` | keyword | Source name being analyzed |
| `model` | keyword | LLM model used |
| `tokens_used` | integer | Tokens consumed for this insight |
| `created_at` | date | When the insight was generated |

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `AI_OBSERVER_ENABLED` | `false` | Enable the service |
| `AI_OBSERVER_DRY_RUN` | `false` | Log intent without LLM calls |
| `AI_OBSERVER_INTERVAL_SECONDS` | `1800` | Polling interval (30 min) |
| `AI_OBSERVER_MAX_TOKENS_PER_INTERVAL` | `25000` | Token budget ceiling per interval |
| `AI_OBSERVER_SUPPRESSED_SOURCES` | _(empty)_ | Comma-separated source domains to exclude from classifier analysis |
| `AI_OBSERVER_MIN_DOMAIN_SAMPLES` | `5` | Minimum articles per domain to include in LLM prompt |
| `AI_OBSERVER_CATEGORY_CLASSIFIER_ENABLED` | `true` | Enable classifier drift category |
| `ANTHROPIC_API_KEY` | — | Required when enabled |
| `AI_OBSERVER_DRIFT_ENABLED` | `false` | Enable drift governor |
| `AI_OBSERVER_DRIFT_INTERVAL_SECONDS` | `21600` | Drift check interval (6h) |
| `AI_OBSERVER_INSIGHT_COOLDOWN_HOURS` | `24` | Dedup window for repeated summaries |
| `AI_OBSERVER_INSIGHT_RETENTION_DAYS` | `30` | Auto-delete insights older than this |
| `AI_OBSERVER_DRIFT_KL_THRESHOLD` | `0.30` | KL divergence alert threshold |
| `AI_OBSERVER_DRIFT_PSI_THRESHOLD` | `0.25` | PSI alert threshold |
| `AI_OBSERVER_DRIFT_MATRIX_THRESHOLD` | `0.20` | Cross-matrix deviation threshold |
| `AI_OBSERVER_DRIFT_BASELINE_WINDOW_DAYS` | `7` | Rolling baseline window |
| `ES_URL` | `http://localhost:9200` | Elasticsearch URL |

---

## Drift Detection

**Dual-ticker architecture**: Fast ticker (30 min) runs classifier category. Slow ticker (6h) runs drift category + insight cleanup.

**Statistical metrics** (computed without LLM):
- **KL divergence**: measures category distribution shift
- **PSI (Population Stability Index)**: measures confidence histogram stability
- **Cross-matrix deviation**: measures region×category co-occurrence changes

**Flow**: `CollectCurrentWindow` → `LoadLatestBaseline` → `Evaluate(baseline, current, thresholds)` → signals. If any signal breaches threshold, LLM is invoked for contextual analysis. Otherwise, insight is written with `"No drift detected"` summary.

**Logging**: Scheduler logs `"Drift check started"` and `"Drift check completed"` with duration. Drift category logs `"Drift evaluation complete"` with signal_count and breach_count.

### drift_baselines ES index

| Field | ES Type | Description |
|-------|---------|-------------|
| `computed_at` | date | When baseline was computed |
| `window_days` | integer | Baseline window size |
| `sample_count` | integer | Documents sampled |
| `category_distribution` | flattened | Category frequency distribution |
| `confidence_histograms` | flattened | Confidence score histograms |
| `cross_matrix` | flattened | Region×category co-occurrence matrix |
| `cross_matrix_counts` | flattened | Raw counts for cross-matrix |

### ES Index Settings

Both `ai_insights` and `drift_baselines` use `number_of_replicas: 0` (single-node cluster). See #496 for cluster-wide replica fix.

---

## Known Constraints

- **Disabled by default**: `AI_OBSERVER_ENABLED=false` means zero production impact until explicitly enabled.
- **Advisory only**: never writes to ingestion pipeline indices (`*_raw_content`, `*_classified_content`).
- **Dry-run mode**: `AI_OBSERVER_DRY_RUN=true` short-circuits before any LLM call.
- **Token budget is pre-estimated**: `len(events) * 50`, not reconciled against actual API spend.
- **Per-category timeout**: 5 minutes to prevent goroutine stalls.
- **ES mapping changes require manual index deletion**: `EnsureMapping` only creates the index if it doesn't exist. After changing the mapping, manually delete the index and restart.
- **`details` field uses flattened ES type**: avoids dynamic type conflicts from inconsistent LLM output.
- **`ANTHROPIC_API_KEY` only required when enabled**: service exits cleanly when disabled without API key.

### Rollout Phases

| Phase | Action |
|-------|--------|
| 0 (current) | Merged with `AI_OBSERVER_ENABLED=false` |
| 1 | Enable with `DRY_RUN=true` (logs prompts + projected cost) |
| 2 | Live calls in dev, review first insights manually |
| 3 | Production with classifier category only |
| 4 | Add sidecar/ingestion categories (requires upstream event emission) |

<\!-- Reviewed: 2026-03-18 — go.mod dependency update only, no spec changes needed -->
