# North Cloud

A microservices content pipeline for crawling, classifying, and distributing articles via Redis Pub/Sub.

## Pipeline

```mermaid
flowchart LR
    SM[Source Manager] --> CR[Crawler]
    CR --> ES_RAW[(ES: raw_content)]
    ES_RAW --> CL[Classifier]
    CL --> ML{ML Sidecars}
    ML --> ES_CLASS[(ES: classified_content)]
    ES_CLASS --> PUB[Publisher]
    PUB --> REDIS[[Redis Pub/Sub]]
    REDIS --> EXT[External Consumers]

    SM -.-> DB_SM[(PostgreSQL)]
    CR -.-> DB_CR[(PostgreSQL)]
    CL -.-> DB_CL[(PostgreSQL)]
    PUB -.-> DB_PUB[(PostgreSQL)]
```

## Architecture

```mermaid
flowchart TB
    subgraph Frontend
        DASH[Dashboard :3002]
        SF[Search Frontend :3003]
    end

    subgraph Core Services
        SM[Source Manager :8050]
        CR[Crawler :8060]
        CL[Classifier :8071]
        PUB[Publisher :8070]
        PIPE[Pipeline :8075]
        IM[Index Manager :8090]
        SRCH[Search :8092]
        AUTH[Auth :8040]
        CT[Click Tracker :8093]
        MCP[MCP Server]
    end

    subgraph ML Sidecars
        CRIME[crime-ml :8076]
        MINING[mining-ml :8077]
        COFORGE[coforge-ml :8078]
        ENT[entertainment-ml :8079]
        ANISH[anishinaabe-ml :8080]
    end

    subgraph Infrastructure
        ES[(Elasticsearch)]
        REDIS[[Redis]]
        PG[(PostgreSQL x7)]
        MINIO[(MinIO)]
        NGINX[Nginx]
    end

    subgraph Observability
        LOKI[Loki]
        ALLOY[Grafana Alloy]
        GRAF[Grafana :3000]
        PYRO[Pyroscope :4040]
    end

    CL --> CRIME & MINING & COFORGE & ENT & ANISH
    CR --> ES
    CL --> ES
    PUB --> REDIS
    NGINX --> DASH & SF & AUTH & SRCH
    ALLOY --> LOKI --> GRAF
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| **source-manager** | 8050 | Manage content sources and crawl configurations |
| **crawler** | 8060 | Web crawler with interval-based job scheduling |
| **classifier** | 8071 | Hybrid rule + ML classification (quality, topics, crime) |
| **publisher** | 8070 | Multi-layer routing to Redis Pub/Sub channels |
| **pipeline** | 8075 | Ingest per-article stage-transition events and expose a funnel view across crawled → indexed → classified → routed → published |
| **index-manager** | 8090 | Elasticsearch index and document management |
| **search** | 8092 | Full-text search across classified content |
| **auth** | 8040 | JWT authentication (24h tokens) |
| **mcp-north-cloud** | stdio | MCP server for AI integration (27 tools) |
| **dashboard** | 3002 | Management UI (Vue.js 3) |
| **search-frontend** | 3003 | Public search UI |
| **nc-http-proxy** | 8055 | HTTP replay proxy for deterministic testing |
| **click-tracker** | 8093 | Click event tracking and analytics |

### ML Sidecars

| Sidecar | Port | Description |
|---------|------|-------------|
| **crime-ml** | 8076 | Crime content classification |
| **mining-ml** | 8077 | Mining content classification |
| **coforge-ml** | 8078 | Coforge content classification |
| **entertainment-ml** | 8079 | Entertainment content classification |
| **anishinaabe-ml** | 8080 | Anishinaabe/Indigenous content classification |

## Quick Start

```bash
# 1. Clone and configure
git clone <repository-url>
cd north-cloud
cp .env.example .env

# 2. Start development environment
task docker:dev:up

# 3. Access the dashboard
open http://localhost:3002
```

## Development

```bash
# Start core services
task docker:dev:up

# Include observability (Loki, Grafana, Pyroscope)
task docker:dev:up:observability

# View logs
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f SERVICE

# Rebuild a service
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d --build SERVICE

# Run tests and linting (all services)
task test
task lint

# Single service
task test:crawler
task lint:classifier

# Stop
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml down
```

## Production

```bash
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d --build
```

## Environment Variables

Key variables (see `.env.example` for full list):

```bash
# Authentication (required)
AUTH_USERNAME=admin
AUTH_PASSWORD=your-password
AUTH_JWT_SECRET=$(openssl rand -hex 32)

# ML classifiers (enable/disable per environment)
CRIME_ENABLED=true
MINING_ENABLED=true
COFORGE_ENABLED=true
ENTERTAINMENT_ENABLED=false
ANISHINAABE_ENABLED=false

# Debug mode
APP_DEBUG=true  # false in production
```

## Documentation

- **ARCHITECTURE.md** - Deep system architecture, routing layers, Redis channels, version history
- **CLAUDE.md** - Comprehensive architecture and development guide
- **DOCKER.md** - Docker reference
- **docs/PIPELINE.md** - Pipeline architecture deep-dive
- **docs/PROFILING.md** - Profiling and performance monitoring
- **Service READMEs** - Individual service docs in each directory
- **publisher/docs/** - Redis message format, consumer integration guide
- **crawler/docs/** - Interval scheduler guide
