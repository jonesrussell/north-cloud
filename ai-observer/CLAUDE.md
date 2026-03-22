# AI Observer — Developer Guide

## Quick Reference

```bash
# Run tests
cd ai-observer && GOWORK=off go test ./...

# Lint
cd ai-observer && GOWORK=off golangci-lint run --config ../.golangci.yml ./...

# Build
cd ai-observer && GOWORK=off go build ./...

# Dry-run smoke test (requires ES running)
cd ai-observer && AI_OBSERVER_ENABLED=true AI_OBSERVER_DRY_RUN=true \
  ANTHROPIC_API_KEY=dummy GOWORK=off go run .
```

---

## Layer Rules

The ai-observer's internal packages form a strict DAG organized into five layers.
A package may import from its own layer or any lower layer. Never from a higher layer.

| Layer | Packages | Role |
|-------|----------|------|
| L0 | `provider`, `drift` | Foundation — no internal imports |
| L1 | `category` | Core domain types — depends on L0 |
| L2 | `provider/anthropic`, `insights` | Domain implementations — depends on L0–L1 |
| L3 | `category/classifier`, `category/drift` | Processing / Analysis — depends on L0–L2 |
| L4 | `scheduler` | Orchestration — depends on L0–L3 |

**Rules:**
- `bootstrap/` is exempt — it assembles the full dependency graph
- `provider/` (L0) must not import any other ai-observer package (it is the leaf)
- All shared infrastructure imports go through `infrastructure/` (no cross-service imports)
- Lateral imports within the same layer are allowed

---

## Architecture

```
ai-observer/
├── main.go                      # Calls bootstrap.Start(); exits 1 on error
├── config.yml.example           # Reference config (env-var based, not YAML at runtime)
├── Dockerfile                   # Multi-stage alpine, uid 1000, no EXPOSE
└── internal/
    ├── bootstrap/               # Config -> logger -> ES -> provider -> categories -> scheduler
    ├── provider/                # LLMProvider interface + Anthropic implementation
    ├── category/                # Category interface, Event, Insight types
    │   └── classifier/          # Classifier drift category (ES sampling + LLM analysis)
    ├── insights/                # ai_insights ES index writer + dedup + retention cleanup
    ├── drift/               # Statistical drift metrics, baseline sampler, evaluator
    └── scheduler/               # Ticker loop + cost-ceiling token budget
```

## Key Design Decisions

- **Advisory only**: never writes to ingestion pipeline indices
- **`AI_OBSERVER_ENABLED=false` default**: zero production impact until explicitly enabled
- **Dry-run**: `AI_OBSERVER_DRY_RUN=true` short-circuits before any LLM call (safe to enable in prod)
- **Token budget**: pre-check estimate (`len(events) * 50`), not reconciled against actual API spend
- **Per-category timeout**: 5 minutes to prevent goroutine stalls on slow ES/API calls
- **`ANTHROPIC_API_KEY` only required when enabled**: service exits cleanly when disabled without API key
- **Insight dedup**: Writer filters duplicate summaries within a configurable cooldown window (default 24h) before indexing
- **Insight retention**: Cleaner runs on slow ticker, deleting insights older than retention period (default 30 days)
- **Dual-ticker**: Fast (30 min) for LLM-based classifier analysis, slow (6h) for statistical drift detection + cleanup
- **Statistical first**: KL, PSI, cross-matrix computed without LLM. LLM only invoked on breach for context.
- **Advisory + draft PRs**: Governor proposes changes via GitHub Actions, never auto-merges

## Config (environment variables)

| Variable | Default | Description |
|---|---|---|
| `AI_OBSERVER_ENABLED` | `false` | Enable the service |
| `AI_OBSERVER_DRY_RUN` | `false` | Log intent without LLM calls |
| `AI_OBSERVER_INTERVAL_SECONDS` | `1800` | Polling interval (30 min) |
| `AI_OBSERVER_MAX_TOKENS_PER_INTERVAL` | `25000` | Token budget ceiling per interval |
| `AI_OBSERVER_CATEGORY_CLASSIFIER_ENABLED` | `true` | Enable classifier drift category |
| `ANTHROPIC_API_KEY` | — | Required when enabled |
| `AI_OBSERVER_SUPPRESSED_SOURCES` | _(empty)_ | Comma-separated source domains to exclude from classifier analysis |
| `AI_OBSERVER_MIN_DOMAIN_SAMPLES` | `5` | Minimum articles per domain to include in LLM prompt |
| `AI_OBSERVER_DRIFT_ENABLED` | `false` | Enable drift governor |
| `AI_OBSERVER_DRIFT_INTERVAL_SECONDS` | `21600` | Drift check interval (6h) |
| `AI_OBSERVER_INSIGHT_COOLDOWN_HOURS` | `24` | Dedup window — suppress repeated summaries |
| `AI_OBSERVER_INSIGHT_RETENTION_DAYS` | `30` | Auto-delete insights older than this |
| `AI_OBSERVER_DRIFT_KL_THRESHOLD` | `0.30` | KL divergence alert threshold |
| `AI_OBSERVER_DRIFT_PSI_THRESHOLD` | `0.25` | PSI alert threshold |
| `AI_OBSERVER_DRIFT_MATRIX_THRESHOLD` | `0.20` | Cross-matrix deviation threshold |
| `AI_OBSERVER_DRIFT_BASELINE_WINDOW_DAYS` | `7` | Rolling baseline window |
| `AI_OBSERVER_DRIFT_BASELINE_RETENTION` | `30` | Baselines to retain |
| `ES_URL` | `http://localhost:9200` | Elasticsearch URL |

## Rollout Phases

| Phase | Action |
|---|---|
| 0 (current) | Merged with `AI_OBSERVER_ENABLED=false` — zero production impact |
| 1 | Enable with `DRY_RUN=true` — logs prompts + projected cost |
| 2 | Live calls in dev, review first insights manually |
| 3 | Production with classifier category only |
| 4 | Add sidecar/ingestion categories (requires upstream event emission) |

## Gotchas

- **ES mapping changes require manual index deletion**: `EnsureMapping` only creates the index if
  it doesn't exist. After changing `insightMapping` in `insights/mapping.go`, you must manually
  delete the index in production and restart ai-observer:
  ```bash
  ssh deployer@northcloud.one "docker exec north-cloud-elasticsearch-1 curl -s -X DELETE http://localhost:9200/ai_insights"
  # Then restart so it recreates with the new mapping
  docker compose -f docker-compose.base.yml -f docker-compose.prod.yml restart ai-observer
  ```
  Data loss is acceptable — insights are ephemeral advisory data.

- **`details` field uses flattened ES type**: LLM-generated details have inconsistent types across
  documents. The `flattened` mapping avoids dynamic type conflicts. All leaf values stored as strings.

- **Single-node ES: set replicas to 0**: `ai_insights` and `drift_baselines` default to `number_of_replicas: 0` in their mappings. On a single-node cluster, replicas can never be assigned, causing yellow cluster status and 503 errors on queries that hit unassigned shards. See #496 for the cluster-wide fix.

- **summary.keyword sub-field added for dedup**: The `summary` field now has a `keyword` sub-field
  (ignore_above=512) used by the dedup aggregation query. This mapping change requires deleting
  the `ai_insights` index in production (see above) on first deploy.

## Grafana Dashboard

The AI Insights dashboard is at `/d/north-cloud-ai-insights` and shows:
- Overview stats (total insights, severity counts, token usage, error count)
- Trends (insights over time by severity, token usage over time)
- Severity/category/model breakdowns (pie charts)
- Service logs (Loki) and recent insights table (ES)

Datasource: `ai-insights` (uid: `ai-insights`) pointing to `ai_insights` index with `created_at` time field.

## Deferred (not in v0)

- Sidecar anomaly category — needs operational events on Redis Streams
- Ingestion failure category — needs Loki HTTP query client in infrastructure
