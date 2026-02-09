# Mining-ML Pipeline End-to-End Design

**Date**: 2026-02-05
**Status**: Approved
**Consumer**: Orewire (orewire.ca) — Laravel mining news aggregator

## Overview

Get the mining-ML classification pipeline running end-to-end:
Crawl mining sites → Classify with hybrid rules+ML → Publish to Redis → Orewire consumes.

## Current State

| Component | Status | Notes |
|-----------|--------|-------|
| Classifier mining logic | Done | Rules + ML hybrid, decision matrix |
| Mining-ML sidecar code | Done | FastAPI, 4 classifiers, trained models |
| Index-Manager aggregations | Done | `/api/v1/aggregations/mining` endpoint |
| Dashboard mining view | Done | MiningBreakdownView with charts |
| Source-manager Excel import | Done | `POST /api/v1/sources/import-excel` |
| Orewire `mining:consume` command | Done | Subscribes to `articles:mining` |
| Orewire `northcloud` Redis config | Done | Separate Redis connection configured |
| Orewire systemd service definition | Done | `orewire-mining-consumer` |
| **Docker mining-ml service** | **Missing** | Not in docker-compose |
| **Publisher mining channels** | **Missing** | No mining channel generation |
| **Production Redis connection** | **Missing** | Orewire .env not configured |
| **Mining sources to crawl** | **Missing** | No mining sites imported yet |

## Architecture

```
Mining News Sites
       │
       ▼
   [Crawler] ──► {source}_raw_content (ES)
       │              │
       │              ▼
       │         [Classifier] + [mining-ml sidecar :8077]
       │              │
       │              ▼
       │         {source}_classified_content (ES)
       │              │
       │              ▼
       │         [Publisher]
       │              │
       │              ▼
       │         Redis: articles:mining
       │              │
       │              ▼
       │         [Orewire mining:consume]
       │              │
       │              ▼
       │         Orewire DB (mining_articles table)
       ▼
   Dashboard ◄── [Index-Manager] ◄── {source}_classified_content
```

## Design Decisions

1. **Single Redis channel**: `articles:mining` — Orewire filters by commodity/stage/location on its end
2. **Localhost port binding**: Orewire connects to North Cloud's Redis on 127.0.0.1:6379
3. **Mirror crime-ml pattern**: mining-ml added to docker-compose.base.yml like crime-ml
4. **Backup before import**: Always backup source-manager + crawler DBs before Excel import

## Section 1: mining-ml Sidecar (docker-compose)

Add to `docker-compose.base.yml` mirroring crime-ml:

```yaml
mining-ml:
  <<: *service-defaults
  build:
    context: ./mining-ml
    dockerfile: Dockerfile
  image: docker.io/jonesrussell/mining-ml:latest
  deploy:
    resources:
      limits:
        cpus: "0.5"
        memory: 512M
  environment:
    MODEL_PATH: /app/models
  healthcheck:
    test: ["CMD", "curl", "-f", "http://localhost:8077/health"]
    interval: 30s
    timeout: 10s
    retries: 3
    start_period: 15s
```

Add to `docker-compose.dev.yml`:
- Expose port 8077:8077
- Classifier `depends_on: mining-ml` with health condition

Set classifier env vars in both dev and prod:
- `MINING_ENABLED=true`
- `MINING_ML_SERVICE_URL=http://mining-ml:8077`

## Section 2: Publisher Mining Channel

**New file**: `publisher/internal/router/mining.go`

Logic (mirroring `crime.go`):
- Filter: `content_type: "article"` AND `mining.relevance IN [core_mining, peripheral_mining]`
- Publish to single channel: `articles:mining`

**Database records** (publisher DB):
- Channel: name=`articles:mining`, description="Mining content for downstream consumers"
- Route: source → channel with `min_quality_score: 50`

**Redis message payload**:
```json
{
  "id": "doc-uuid",
  "title": "Gold Exploration Results...",
  "body": "Full article text...",
  "source": "https://mining-news.com",
  "published_date": "2026-02-05T...",
  "quality_score": 85,
  "mining": {
    "relevance": "core_mining",
    "mining_stage": "exploration",
    "commodities": ["gold", "copper"],
    "location": "local_canada"
  },
  "publisher": {
    "route_id": "uuid",
    "channel": "articles:mining",
    "published_at": "2026-02-05T..."
  }
}
```

## Section 3: Orewire Production Redis

Orewire `.env` on production:
```
NORTHCLOUD_REDIS_HOST=127.0.0.1
NORTHCLOUD_REDIS_PORT=6379
```

North Cloud Redis already exposed on port 6379 to host (docker-compose.base.yml).

Enable systemd service:
```bash
systemctl --user enable orewire-mining-consumer
systemctl --user start orewire-mining-consumer
```

## Section 4: Import Workflow

1. Backup DBs: `task db:backup:source-manager && task db:backup:crawler`
2. Upload Excel via source-manager dashboard
3. Sources created and crawl jobs scheduled
4. Pipeline runs: crawl → classify (with mining-ml) → publish → Orewire consumes

## Execution Order

1. Add mining-ml to docker-compose (base + dev)
2. Add publisher mining channel generation code
3. Create `articles:mining` channel + routes in publisher DB
4. Deploy to production (`docker compose up -d --build`)
5. Set Orewire Redis env vars + start consumer service
6. Backup DBs, import Excel spreadsheet
7. Start crawl jobs, watch content flow through pipeline
