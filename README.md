# North Cloud

A microservices-based content pipeline for crawling, classifying, and distributing news articles.

## Pipeline

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           NORTH CLOUD PIPELINE                              │
└─────────────────────────────────────────────────────────────────────────────┘

  ┌──────────┐      ┌───────────────┐      ┌────────────┐      ┌───────────┐
  │  Source  │      │               │      │            │      │           │
  │  Manager │─────▶│    Crawler    │─────▶│ Classifier │─────▶│ Publisher │
  │          │      │               │      │            │      │           │
  └──────────┘      └───────────────┘      └────────────┘      └─────┬─────┘
       │                    │                     │                   │
       │                    ▼                     ▼                   ▼
       │            ┌───────────────────────────────────┐      ┌───────────┐
       │            │         ELASTICSEARCH             │      │   REDIS   │
       │            │  ┌─────────────┐ ┌─────────────┐  │      │  Pub/Sub  │
       │            │  │ raw_content │ │ classified  │  │      └─────┬─────┘
       │            │  │   indexes   │ │   indexes   │  │            │
       │            └──┴─────────────┴─┴─────────────┴──┘            │
       │                                                             │
       ▼                                                             ▼
  ┌──────────┐                                          ┌────────────────────┐
  │PostgreSQL│                                          │ EXTERNAL CONSUMERS │
  │ (5 DBs)  │                                          │  Drupal, Laravel,  │
  └──────────┘                                          │  Node.js, Python   │
                                                        └────────────────────┘
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| **crawler** | 8060 | Web crawler with interval-based job scheduling |
| **source-manager** | 8050 | Manage content sources and crawl configurations |
| **classifier** | 8071 | Classify content with quality scores and topics |
| **publisher** | 8070 | Route articles to Redis pub/sub channels |
| **index-manager** | 8090 | Elasticsearch index management |
| **search** | 8092 | Full-text search across classified content |
| **auth** | 8040 | JWT authentication |
| **dashboard** | 3002 | Unified management UI |

## Quick Start

```bash
# 1. Clone and configure
git clone <repository-url>
cd north-cloud
cp .env.example .env

# 2. Start development environment
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d

# 3. Access the dashboard
open http://localhost:3002
```

## Development

```bash
# Start dev environment
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d

# View logs
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f

# Stop
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml down
```

## Production

```bash
# Build and start
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d --build

# Stop
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml down
```

## Environment Variables

Key variables (see `.env.example` for full list):

```bash
# Authentication (required)
AUTH_USERNAME=admin
AUTH_PASSWORD=your-password
AUTH_JWT_SECRET=$(openssl rand -hex 32)

# Debug mode
APP_DEBUG=true  # false in production
```

## Documentation

- **CLAUDE.md** - Comprehensive architecture guide
- **DOCKER.md** - Docker reference
- **Service READMEs** - Individual service docs in each directory
