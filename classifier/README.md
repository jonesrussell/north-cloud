# Classifier Service

The classifier microservice for North Cloud that separates content classification from crawling, enabling ML-based classification in the future.

## Overview

The classifier processes raw content from Elasticsearch and applies multi-strategy classification:

- **Content Type Detection** - Determines if content is article, page, video, image, or job
- **Quality Scoring** - Rates content 0-100 based on completeness, readability, metadata richness
- **Topic Classification** - Categorizes content (crime, sports, politics, etc.)
- **Source Reputation** - Tracks and scores source trustworthiness (0-100)

## Architecture

```
Crawler (Raw Ingestion) → Elasticsearch (Raw Content)
                              ↓
                      Classifier Service
                      - Content Type
                      - Quality Scoring
                      - Topic Classification
                      - Source Reputation
                              ↓
                   Elasticsearch (Classified Content)
                              ↓
                      Publisher (Consumes)
```

## Technology Stack

- **Go**: 1.25+ (container-aware GOMAXPROCS)
- **Database**: PostgreSQL 16 (rules, source reputation, history)
- **Cache/Queue**: Redis (caching, job queue)
- **Storage**: Elasticsearch 9.x (raw & classified content)
- **HTTP Framework**: Gin
- **Logging**: Zap (structured logging, snake_case fields)

## Quick Start

### Development

```bash
# Copy environment variables
cp .env.example .env

# Start infrastructure services (PostgreSQL, Elasticsearch, Redis)
docker-compose -f docker-compose.base.yml up -d

# Start classifier service
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d classifier

# View logs
docker logs -f north-cloud-classifier-dev
```

### Local Development (without Docker)

```bash
# Install dependencies
go mod download

# Run migrations (requires PostgreSQL running)
psql -h localhost -p 5435 -U postgres -d classifier -f migrations/001_create_rules.sql
psql -h localhost -p 5435 -U postgres -d classifier -f migrations/002_create_source_reputation.sql
psql -h localhost -p 5435 -U postgres -d classifier -f migrations/003_create_classification_history.sql
psql -h localhost -p 5435 -U postgres -d classifier -f migrations/004_create_ml_models.sql
psql -h localhost -p 5435 -U postgres -d classifier -f migrations/005_add_comprehensive_categories.sql

# Run the service
go run main.go
```

## Configuration

Configuration is managed via `config.yml`. See `config.yml.example` for all available options.

Key configuration sections:
- **Service**: Port, concurrency, batch size, thresholds
- **Database**: PostgreSQL connection settings
- **Elasticsearch**: ES connection and index naming
- **Redis**: Redis connection and pub/sub channels
- **Classification**: Enable/disable classifiers, confidence thresholds
- **ML**: ML model integration settings (future)

## Environment Variables

See `.env.example` in the root directory for all classifier-related variables:

```bash
CLASSIFIER_PORT=8071
CLASSIFIER_ENABLED=true
CLASSIFIER_CONCURRENCY=10
CLASSIFIER_BATCH_SIZE=100
CLASSIFIER_MIN_QUALITY_SCORE=50

POSTGRES_CLASSIFIER_USER=postgres
POSTGRES_CLASSIFIER_PASSWORD=postgres
POSTGRES_CLASSIFIER_DB=classifier
POSTGRES_CLASSIFIER_PORT=5435
```

## Database Schema

### Tables

1. **classification_rules** - Rules for content type and topic classification
2. **source_reputation** - Source trustworthiness and quality metrics
3. **classification_history** - Audit trail for ML training data
4. **ml_models** - ML model metadata and performance metrics

### Migrations

Migrations are located in `migrations/` and should be run in order:

```bash
001_create_rules.sql
002_create_source_reputation.sql
003_create_classification_history.sql
004_create_ml_models.sql
005_add_comprehensive_categories.sql
```

## Classification Rules & Priority System

### Understanding Priority

**Priority** determines the evaluation order of classification rules when processing content:

- **Higher priority rules are evaluated first** (stored as integers 0-100, displayed as "high"/"normal"/"low" in the UI)
- This ensures more specific or critical rules (like "crime") are checked before general ones
- Helps manage conflicts when content matches multiple rules
- Current priority mapping:
  - **High (10)**: Critical/time-sensitive categories (crime, breaking_news, health_emergency)
  - **Normal (5)**: Common news categories (business, technology, health, entertainment, etc.)
  - **Low (1-3)**: Niche categories (pets, gaming, shopping, etc.)

The system orders rules by `priority DESC` when loading them from the database, so high-priority rules get the first chance to match content. This is important for accurate classification, especially when content could match multiple categories.

### Comprehensive Category Taxonomy

The classifier uses a comprehensive Microsoft/Bing-style taxonomy with 25+ topic categories:

**High Priority Categories**:
- `crime` - Criminal activity, law enforcement, legal proceedings
- `breaking_news` - Urgent, developing stories, news alerts
- `health_emergency` - Pandemics, outbreaks, public health crises

**Normal Priority Categories**:
- `business` - Companies, markets, economy, trade, finance
- `technology` - Software, hardware, AI, innovation, digital
- `health` - Medical, healthcare, wellness, fitness
- `entertainment` - Movies, TV, music, celebrities, arts
- `science` - Research, discoveries, experiments, studies
- `education` - Schools, universities, learning, academic
- `weather` - Forecasts, storms, climate, meteorology
- `travel` - Tourism, destinations, hotels, flights
- `food` - Restaurants, recipes, cooking, cuisine
- `lifestyle` - Fashion, culture, trends, personal
- `automotive` - Cars, vehicles, driving, traffic
- `real_estate` - Property, housing, mortgages, real estate
- `finance` - Banking, investments, credit, loans
- `environment` - Climate, pollution, sustainability, nature
- `arts` - Art, galleries, museums, creative works

**Low Priority Categories**:
- `sports` - Games, teams, tournaments, athletics
- `politics` - Elections, government, policy, legislation
- `local_news` - Community, neighborhood, municipal news
- `pets` - Animals, veterinary, pet care
- `gaming` - Video games, esports, consoles
- `shopping` - Retail, stores, purchases, deals
- `home_garden` - Home improvement, gardening, landscaping
- `recreation` - Hobbies, leisure activities, outdoor fun

### Managing Rules

Classification rules can be managed via:
- **Dashboard UI**: `http://localhost:3002/classifier/rules` - Visual interface for creating, editing, and deleting rules
- **REST API**: See API Endpoints section below
- **Database**: Direct SQL access to `classification_rules` table

Each rule includes:
- **Topic name**: The category identifier (e.g., "crime", "technology")
- **Keywords**: Array of keywords used for matching (content is matched against title + text)
- **Min confidence**: Minimum score (0.0-1.0) required for classification
- **Priority**: Evaluation order (higher = evaluated first)
- **Enabled**: Whether the rule is active

Rules are automatically loaded from the database when the classifier service starts.

## API Endpoints

### Classification

- `POST /api/v1/classify` - Classify single content item
- `POST /api/v1/classify/batch` - Classify multiple items
- `GET /api/v1/classify/:content_id` - Get classification result

### Rules Management

- `GET /api/v1/rules` - List classification rules
- `POST /api/v1/rules` - Create rule
- `PUT /api/v1/rules/:id` - Update rule
- `DELETE /api/v1/rules/:id` - Delete rule

### Source Reputation

- `GET /api/v1/sources` - List sources
- `GET /api/v1/sources/:name` - Get source details
- `PUT /api/v1/sources/:name` - Update source
- `GET /api/v1/sources/:name/stats` - Source statistics

### Statistics

- `GET /api/v1/stats` - Overall classification stats
- `GET /api/v1/stats/topics` - Topic distribution
- `GET /api/v1/stats/sources` - Source reputation distribution

## Development Status

**Week 1 (Complete)**:
- ✅ Directory structure
- ✅ Go module initialization
- ✅ Domain models (RawContent, ClassifiedContent, rules)
- ✅ Elasticsearch mappings (raw_content, classified_content)
- ✅ Database migrations
- ✅ Docker integration
- ✅ Environment configuration

**Week 2 (Planned)**:
- ContentTypeClassifier implementation
- QualityScorer implementation
- TopicClassifier implementation
- SourceReputationScorer implementation
- Unit tests

**Week 3 (Planned)**:
- Processing pipeline with worker pool
- Polling mechanism
- Rate limiting
- Integration tests

**Week 4 (Planned)**:
- REST API implementation
- Crawler dual indexing update
- Service deployment
- End-to-end validation

## Project Structure

```
classifier/
├── cmd/
│   ├── httpd/          # HTTP API server
│   └── worker/         # Background worker
├── internal/
│   ├── api/            # REST API handlers
│   ├── classifier/     # Core classification logic
│   ├── config/         # Configuration
│   ├── database/       # PostgreSQL operations
│   ├── domain/         # Domain models
│   ├── elasticsearch/  # ES client & mappings
│   ├── logger/         # Structured logging
│   ├── metrics/        # Metrics tracking
│   ├── ml/             # ML integration (future)
│   ├── processor/      # Processing pipeline
│   └── redis/          # Redis caching/queuing
├── migrations/         # Database migrations
├── tests/
│   ├── integration/    # Integration tests
│   └── unit/           # Unit tests
├── Dockerfile          # Production Dockerfile
├── Dockerfile.dev      # Development Dockerfile
├── config.yml.example  # Configuration template
├── go.mod              # Go module definition
└── main.go             # Main entry point
```

## Testing

```bash
# Run unit tests
go test ./internal/...

# Run with coverage
go test -cover ./internal/...

# Run integration tests (requires services running)
go test ./tests/integration/...
```

## Performance Targets

- Classification latency: <100ms per item (p95)
- Throughput: 1000 items/minute (single instance)
- Batch size: 100 items
- Concurrency: 10 workers
- Poll interval: 30 seconds

## ML Integration (Future)

The service is designed for future ML model integration:

- Interface-based design for swapping rule-based with ML models
- A/B testing framework for gradual rollout
- Classification history for training data
- Support for embedded (TensorFlow Lite, ONNX) and API-based models

## Contributing

Follow North Cloud conventions:
- Go 1.25+ features (container-aware GOMAXPROCS)
- Structured logging with snake_case fields
- REST API with Gin framework
- PostgreSQL for persistence
- Elasticsearch for content storage
- Redis for caching/queuing

## License

See LICENSE file in the root directory.

## Related Documentation

- Main project: `/README.md`
- Architecture guide: `/CLAUDE.md`
- Docker guide: `/DOCKER.md`
- Implementation plan: `~/.claude/plans/elegant-exploring-trinket.md`
