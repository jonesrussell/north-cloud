# Classifier Service

The classifier microservice for North Cloud processes raw crawled content and produces
enriched, classified articles ready for downstream routing by the publisher.

## Overview

The classifier reads raw content from Elasticsearch and applies a multi-step pipeline:

- **Content Type Detection** - Determines whether content is an article, page, video, image, or job
- **Quality Scoring** - Rates content 0-100 based on word count, metadata completeness, content richness, and readability
- **Topic Classification** - Categorises content using keyword rules stored in PostgreSQL
- **Source Reputation** - Tracks and scores source trustworthiness (0-100)
- **Hybrid ML Classifiers** - Five optional domain classifiers that combine keyword rules with ML sidecar calls: crime, mining, entertainment, anishinaabe, and coforge

## Architecture

```
Crawler → Elasticsearch: {source}_raw_content (classification_status=pending)
                              ↓
                      Classifier Service
              ┌─────────────────────────────────┐
              │ 1. Content Type Detection        │
              │ 2. Quality Scoring (0-100)       │
              │ 3. Topic Classification (rules)  │
              │ 4. Source Reputation             │
              │ 5-9. Hybrid ML Classifiers       │
              │    (crime, mining, entertainment,│
              │     anishinaabe, coforge)        │
              └─────────────────────────────────┘
                              ↓
              Elasticsearch: {source}_classified_content
                              ↓
                      Publisher (Consumes)
```

## Technology Stack

- **Go**: 1.25+ (container-aware GOMAXPROCS)
- **Database**: PostgreSQL (rules, source reputation, classification history)
- **Cache**: Redis (classification result caching)
- **Storage**: Elasticsearch (raw and classified content)
- **HTTP Framework**: Gin
- **Logging**: Zap (structured JSON logging, snake_case fields)

## Quick Start

### Development

```bash
# Start infrastructure services
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d postgres-classifier elasticsearch redis

# Start classifier service
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d classifier

# View logs
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f classifier
```

### Local Development (without Docker)

```bash
# Install dependencies
go mod download

# Run migrations
go run cmd/migrate/main.go up

# Run the service
go run main.go
```

## Configuration

Configuration is managed via `config.yml`. See `config.yml.example` for all available options.

Key configuration sections:

- **Service**: Port, concurrency, batch size, poll interval, quality thresholds
- **Database**: PostgreSQL connection settings
- **Elasticsearch**: ES connection and index suffix naming
- **Redis**: Redis connection and cache TTL settings
- **Classification**: Per-classifier enable/disable flags and ML sidecar URLs

## Environment Variables

```bash
# Service
CLASSIFIER_PORT=8071
CLASSIFIER_CONCURRENCY=10

# Database
POSTGRES_HOST=postgres-classifier
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=classifier

# Elasticsearch
ELASTICSEARCH_URL=http://elasticsearch:9200

# Redis
REDIS_URL=redis:6379

# Auth
AUTH_JWT_SECRET=<shared secret>

# Hybrid ML classifiers (each disabled by default)
CRIME_ENABLED=true
CRIME_ML_SERVICE_URL=http://crime-ml:8076

MINING_ENABLED=true
MINING_ML_SERVICE_URL=http://mining-ml:8077

COFORGE_ENABLED=true
COFORGE_ML_SERVICE_URL=http://coforge-ml:8078

ENTERTAINMENT_ENABLED=true
ENTERTAINMENT_ML_SERVICE_URL=http://entertainment-ml:8079

ANISHINAABE_ENABLED=true
ANISHINAABE_ML_SERVICE_URL=http://anishinaabe-ml:8080
```

## Database Schema

### Tables

1. **classification_rules** - Keyword rules for topic classification (topic, keywords JSON, priority, enabled)
2. **source_reputation** - Source trustworthiness metrics (reputation_score, total_articles, spam_count)
3. **classification_history** - Audit trail of classifications (content_id, quality_score, topics, classified_at)
4. **ml_models** - ML model metadata and version tracking
5. **dead_letter_queue** - Failed classifications for retry and analysis

### Migrations

```
001_create_rules.sql
002_create_source_reputation.sql
003_create_classification_history.sql
004_create_ml_models.sql
005_add_comprehensive_categories.sql
006_remove_is_crime_related.sql          # Deprecated boolean field
007_add_crime_subcategories.sql          # 5 crime sub-category rules
008_increase_content_url_size.sql
009_create_dead_letter_queue.sql
010_add_mining_topic.sql
011_tighten_mining_topic_rule.sql        # Removed ambiguous terms; ML handles nuance
```

## Classification Rules and Priority System

### Understanding Priority

Priority determines the evaluation order when processing content:

- **Higher priority rules are evaluated first** (integers 0-100)
- Specific, high-signal rules (crime sub-categories) run before general ones
- Current priority mapping:
  - **High (10)**: Crime sub-categories, breaking news, health emergencies
  - **Normal (5)**: Business, technology, health, entertainment, etc.
  - **Low (1-3)**: Pets, gaming, shopping, home/garden, recreation

### Topic Taxonomy

**High Priority**:
- `breaking_news` - Urgent, developing stories
- `health_emergency` - Pandemics, outbreaks, public health crises

**Crime Sub-Categories** (Migration 007):
- `violent_crime` (Priority 10) - Murder, assault, shooting, homicide
- `property_crime` (Priority 9) - Theft, burglary, vandalism, arson
- `drug_crime` (Priority 9) - Drug trafficking, narcotics, overdoses
- `organized_crime` (Priority 9) - Cartels, racketeering, money laundering
- `criminal_justice` (Priority 5) - Court cases, trials, sentencing

**Normal Priority**:
- `business`, `technology`, `health`, `entertainment`, `science`, `education`
- `weather`, `travel`, `food`, `lifestyle`, `automotive`, `real_estate`
- `finance`, `environment`, `arts`

**Low Priority**:
- `sports`, `politics`, `local_news`, `pets`, `gaming`, `shopping`
- `home_garden`, `recreation`

**Domain-Specific** (managed by hybrid ML classifiers):
- `mining` - Mining industry content (rules + mining-ml sidecar)

### Managing Rules

Classification rules can be managed via:

- **Dashboard UI**: `http://localhost:3002/classifier/rules`
- **REST API**: See API Endpoints section
- **Database**: Direct SQL access to `classification_rules` table

Each rule includes: topic name, keywords array, min confidence (0.0-1.0), priority, and enabled flag.

Rules are loaded from the database at startup and cached in memory. Changes require a service restart or an API call to reload.

## Hybrid ML Classifiers

The classifier runs five optional domain-specific classifiers that combine keyword rules with ML sidecar HTTP calls. Each is independently enabled by an environment flag.

| Classifier | Env Flag | ML Sidecar | Default URL |
|------------|----------|------------|-------------|
| Crime | `CRIME_ENABLED` | crime-ml | `http://crime-ml:8076` |
| Mining | `MINING_ENABLED` | mining-ml | `http://mining-ml:8077` |
| Coforge | `COFORGE_ENABLED` | coforge-ml | `http://coforge-ml:8078` |
| Entertainment | `ENTERTAINMENT_ENABLED` | entertainment-ml | `http://entertainment-ml:8079` |
| Anishinaabe | `ANISHINAABE_ENABLED` | anishinaabe-ml | `http://anishinaabe-ml:8080` |

Each hybrid classifier:
1. Evaluates keyword rules (precision/blocking signal)
2. Calls the ML sidecar via HTTP for confidence score (recall signal)
3. Merges results via a decision matrix (rules + ML → final relevance class)
4. Returns `nil` when disabled — the field is absent from classified content

Failure modes are non-blocking: if the ML sidecar is unreachable, the classifier falls back to rules-only mode and logs a warning. Classification continues for all other steps.

## Content Type Detection

The classifier assigns a `content_type` and optional `content_subtype` to each document.

**Content types**: `article`, `page`, `video`, `image`, `job`

**Article subtypes**: `press_release`, `blog_post`, `event`, `advisory`, `report`, `blotter`, `company_announcement`

Hybrid ML classifiers only run for articles. Subtype gates further narrow which classifiers run:
- `event` — location classifier only
- `blotter` — crime classifier only
- `report` — no optional classifiers
- all others (including standard articles) — full set of enabled optional classifiers

## API Endpoints

All `/api/v1/*` routes require a valid JWT (`Authorization: Bearer <token>`).

**Health** (public):
- `GET /health` - Liveness check
- `GET /ready` - Readiness check
- `GET /health/memory` - Memory stats
- `GET /metrics` - Prometheus metrics (when telemetry enabled)

**Classification**:
- `POST /api/v1/classify` - Classify a single content item
- `POST /api/v1/classify/batch` - Classify multiple items
- `POST /api/v1/classify/reclassify/:content_id` - Re-classify an existing document
- `GET /api/v1/classify/:content_id` - Get classification result for a document

**Rules Management**:
- `GET /api/v1/rules` - List classification rules
- `POST /api/v1/rules` - Create rule
- `PUT /api/v1/rules/:id` - Update rule
- `DELETE /api/v1/rules/:id` - Delete rule

**Source Reputation**:
- `GET /api/v1/sources` - List sources
- `GET /api/v1/sources/:name` - Get source details
- `PUT /api/v1/sources/:name` - Update source
- `GET /api/v1/sources/:name/stats` - Source statistics

**Statistics**:
- `GET /api/v1/stats` - Overall classification stats
- `GET /api/v1/stats/topics` - Topic distribution
- `GET /api/v1/stats/sources` - Source reputation distribution

**Metrics**:
- `GET /api/v1/metrics/ml-health` - ML sidecar health (reachability, latency, pipeline mode for all 5 sidecars)

## Project Structure

```
classifier/
├── cmd/
│   └── migrate/        # Database migration runner
├── internal/
│   ├── api/            # HTTP handlers, routes, server setup
│   ├── anishinaabemlclient/  # Anishinaabe ML sidecar HTTP client
│   ├── bootstrap/      # Service initialisation (config, DB, storage, classifier)
│   ├── classifier/     # Core classification logic
│   │   ├── classifier.go         # Orchestrator (Classify, BuildClassifiedContent)
│   │   ├── content_type.go       # Article vs page detection
│   │   ├── quality.go            # Quality scoring (0-100)
│   │   ├── topic.go              # Rule-based topic detection
│   │   ├── source_reputation.go  # Source trust scoring
│   │   ├── crime.go              # Hybrid crime classifier
│   │   ├── mining.go             # Hybrid mining classifier
│   │   ├── coforge.go            # Hybrid coforge classifier
│   │   ├── entertainment.go      # Hybrid entertainment classifier
│   │   ├── anishinaabe.go        # Hybrid anishinaabe classifier
│   │   └── location.go           # Location classifier
│   ├── coforgemlclient/    # Coforge ML sidecar HTTP client
│   ├── config/             # Configuration struct and loader
│   ├── data/               # Static data assets
│   ├── database/           # PostgreSQL repositories
│   ├── domain/             # RawContent, ClassifiedContent, Rule models
│   ├── elasticsearch/      # ES client and index mappings
│   ├── entertainmentmlclient/ # Entertainment ML sidecar HTTP client
│   ├── metrics/            # Metrics tracking
│   ├── mlclient/           # Shared ML client utilities
│   ├── mlhealth/           # ML sidecar health check helper
│   ├── mltransport/        # HTTP transport for ML sidecars
│   ├── processor/          # Background polling and batch processing
│   ├── server/             # Server lifecycle helpers
│   ├── storage/            # Elasticsearch read/write
│   ├── telemetry/          # OpenTelemetry/Prometheus provider
│   └── testhelpers/        # Shared test utilities
├── migrations/             # SQL migration files (001-011)
├── tests/
│   └── integration/        # Integration tests
├── Dockerfile
├── Dockerfile.dev
├── config.yml.example
├── go.mod
└── main.go
```

## Testing

```bash
# Run all tests
task test

# Run with coverage
task test:cover

# Run specific package
go test ./internal/classifier/...

# Run integration tests (requires services running)
go test ./tests/integration/...
```

## Related Documentation

- Main project: `/README.md`
- Architecture guide: `/CLAUDE.md`
- Docker guide: `/DOCKER.md`
- Classifier developer guide: `/classifier/CLAUDE.md`
