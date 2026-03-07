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
    ├── insights/                # ai_insights ES index writer
    └── scheduler/               # Ticker loop + cost-ceiling token budget
```

## Key Design Decisions

- **Advisory only**: never writes to ingestion pipeline indices
- **`AI_OBSERVER_ENABLED=false` default**: zero production impact until explicitly enabled
- **Dry-run**: `AI_OBSERVER_DRY_RUN=true` short-circuits before any LLM call (safe to enable in prod)
- **Token budget**: pre-check estimate (`len(events) * 50`), not reconciled against actual API spend
- **Per-category timeout**: 5 minutes to prevent goroutine stalls on slow ES/API calls
- **`ANTHROPIC_API_KEY` only required when enabled**: service exits cleanly when disabled without API key

## Config (environment variables)

| Variable | Default | Description |
|---|---|---|
| `AI_OBSERVER_ENABLED` | `false` | Enable the service |
| `AI_OBSERVER_DRY_RUN` | `false` | Log intent without LLM calls |
| `AI_OBSERVER_INTERVAL_SECONDS` | `1800` | Polling interval (30 min) |
| `AI_OBSERVER_MAX_TOKENS_PER_INTERVAL` | `25000` | Token budget ceiling per interval |
| `AI_OBSERVER_CATEGORY_CLASSIFIER_ENABLED` | `true` | Enable classifier drift category |
| `ANTHROPIC_API_KEY` | — | Required when enabled |
| `ES_URL` | `http://localhost:9200` | Elasticsearch URL |

## Rollout Phases

| Phase | Action |
|---|---|
| 0 (current) | Merged with `AI_OBSERVER_ENABLED=false` — zero production impact |
| 1 | Enable with `DRY_RUN=true` — logs prompts + projected cost |
| 2 | Live calls in dev, review first insights manually |
| 3 | Production with classifier category only |
| 4 | Add sidecar/ingestion categories (requires upstream event emission) |

## Deferred (not in v0)

- Sidecar anomaly category — needs operational events on Redis Streams
- Ingestion failure category — needs Loki HTTP query client in infrastructure
- Dashboard UI for `ai_insights` index
