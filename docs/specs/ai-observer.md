# AI Observer Spec

> Last verified: 2026-03-08

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
      classifier/                  # Classifier drift category (ES sampling + LLM analysis)
    insights/                      # ai_insights ES index writer + mapping
    scheduler/                     # Ticker loop + cost-ceiling token budget
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
| `AI_OBSERVER_CATEGORY_CLASSIFIER_ENABLED` | `true` | Enable classifier drift category |
| `ANTHROPIC_API_KEY` | — | Required when enabled |
| `ES_URL` | `http://localhost:9200` | Elasticsearch URL |

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
