# Publisher Service

A Go service that publishes classified articles from Elasticsearch to Redis pub/sub channels, enabling decoupled consumption by external services like Laravel, Node.js, Python applications, and more.

## Architecture

```
┌─────────────────┐
│  Classifier     │
│  (existing)     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐      ┌──────────────────┐
│ Elasticsearch   │◄─────┤   Publisher      │
│ Classified      │      │   Service        │
│ Content Indexes │      │  (this service)  │
└─────────────────┘      │                  │
                         │ - Query ES       │
                         │ - Filter by      │
                         │   content_type,  │
                         │   quality/topics │
                         │ - Publish to     │
                         │   Redis pub/sub  │
                         └────────┬─────────┘
                                  │
                                  ▼
                         ┌──────────────────────────┐
                         │  Redis Pub/Sub           │
                         │  Channels:               │
                         │  - articles:crime:violent│
                         │  - articles:crime:property│
                         │  - articles:crime:drug   │
                         │  - articles:crime:organized│
                         │  - articles:news         │
                         └────────┬─────────────────┘
                                  │
              ┌───────────────────┼───────────────────┐
              │                   │                   │
              ▼                   ▼                   ▼
    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
    │   Laravel    │    │   Node.js    │    │   Python     │
    │   Consumer   │    │   Consumer   │    │   Consumer   │
    └──────────────┘    └──────────────┘    └──────────────┘
```

## Features

- **Database-Backed Routing**: PostgreSQL stores sources, channels, and routes configuration
- **Dynamic Configuration**: No service restart needed to add/modify routes
- **Quality Filtering**: Routes support minimum quality score thresholds (0-100)
- **Topic-Based Channels**: Publish to topic-specific Redis channels (e.g., `articles:crime:violent`, `articles:crime:drug`, `articles:news`)
- **Redis Pub/Sub Publishing**: Standard JSON message format compatible with Laravel 12, Node.js, Python, and more
- **Deduplication**: Database-backed publish history prevents duplicate publications
- **Web UI**: Vue.js dashboard for managing sources, channels, and routes
- **REST API**: Full CRUD API for programmatic configuration
- **Publisher Statistics**: Real-time publishing metrics and history
- **Graceful Shutdown**: Handles SIGTERM/SIGINT for clean shutdowns

## Prerequisites

- Go 1.25 or later
- Task (taskfile.dev) - for running build tasks
- Elasticsearch 8.x (with classified content indexes)
- Redis 6.x or later
- PostgreSQL 16+ (for publisher database)

## Quick Start

### 1. Configuration

The publisher uses environment variables for configuration. Create a `.env` file or set them in your environment:

```bash
# Database
POSTGRES_PUBLISHER_HOST=localhost
POSTGRES_PUBLISHER_PORT=5432
POSTGRES_PUBLISHER_USER=postgres
POSTGRES_PUBLISHER_PASSWORD=your-password
POSTGRES_PUBLISHER_DB=publisher

# Elasticsearch
ELASTICSEARCH_URL=http://localhost:9200

# Redis
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=  # Optional

# API Server
PUBLISHER_PORT=8070

# Router Service
PUBLISHER_ROUTER_CHECK_INTERVAL=5m
PUBLISHER_ROUTER_BATCH_SIZE=100
```

### 2. Install Task (if not already installed)

```bash
# macOS
brew install go-task/tap/go-task

# Linux
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b ~/.local/bin

# Or via Go
go install github.com/go-task/task/v3/cmd/task@latest
```

### 3. Run Locally

```bash
# Install dependencies
task deps

# Run API server
publisher api

# Run router service (in separate terminal)
publisher router

# Or use task commands
task run:api
task run:router
```

### 4. Run with Docker Compose

```bash
# Start all services (uses docker-compose.base.yml)
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d

# View logs
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f publisher-api
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f publisher-router
```

## Service Components

The publisher service consists of two components:

### 1. API Server (`publisher api`)

REST API for managing sources, channels, and routes:

- `GET /health` - Health check
- `GET /api/v1/sources` - List sources
- `POST /api/v1/sources` - Create source
- `PUT /api/v1/sources/:id` - Update source
- `DELETE /api/v1/sources/:id` - Delete source
- `GET /api/v1/channels` - List channels
- `POST /api/v1/channels` - Create channel
- `PUT /api/v1/channels/:id` - Update channel
- `DELETE /api/v1/channels/:id` - Delete channel
- `GET /api/v1/routes` - List routes (with joined source/channel names)
- `POST /api/v1/routes` - Create route
- `PUT /api/v1/routes/:id` - Update route
- `DELETE /api/v1/routes/:id` - Delete route
- `GET /api/v1/stats/overview` - Publishing statistics
- `GET /api/v1/publish-history` - Paginated publish history

### 2. Router Service (`publisher router`)

Background worker that:
- Polls enabled routes at configured intervals
- Queries Elasticsearch classified_content indexes
- Filters by `content_type: "article"` to exclude pages/listings
- Filters articles by quality score and topics
- Publishes matching articles to Redis pub/sub channels
- Records publish history in database

## Configuration

### Environment Variables

#### Database Configuration
- `POSTGRES_PUBLISHER_HOST` - PostgreSQL host (default: `localhost`)
- `POSTGRES_PUBLISHER_PORT` - PostgreSQL port (default: `5432`)
- `POSTGRES_PUBLISHER_USER` - PostgreSQL user (default: `postgres`)
- `POSTGRES_PUBLISHER_PASSWORD` - PostgreSQL password (required)
- `POSTGRES_PUBLISHER_DB` - PostgreSQL database (default: `publisher`)

#### Elasticsearch Configuration
- `ELASTICSEARCH_URL` - Elasticsearch URL (default: `http://localhost:9200`)

#### Redis Configuration
- `REDIS_ADDR` - Redis address (default: `localhost:6379`)
- `REDIS_PASSWORD` - Redis password (optional)

#### API Server Configuration
- `PUBLISHER_PORT` - HTTP port (default: `8070`)
- `AUTH_JWT_SECRET` - JWT secret for authentication (optional)
- `GIN_MODE` - Gin mode: `debug` or `release` (default: `debug`)

#### Router Service Configuration
- `PUBLISHER_ROUTER_CHECK_INTERVAL` - How often to check routes (default: `5m`)
- `PUBLISHER_ROUTER_BATCH_SIZE` - Articles per route per check (default: `100`)

#### Application Configuration
- `APP_DEBUG` - Enable debug mode (`true`, `1`, `yes` for debug)

## Database Schema

The publisher uses four main tables:

### `sources`
Elasticsearch index patterns to monitor:
- `index_pattern` - Pattern like `example_com_classified_content`

### `channels`
Redis pub/sub channels:
- `name` - Channel name like `articles:crime:violent`, `articles:crime:drug`, `articles:news`
- `description` - Optional description

**Crime Sub-Category Channels** (as of Migration 007):
- `articles:crime:violent` - Violent crime (gang violence, murder, assault, shootings)
- `articles:crime:property` - Property crime (theft, burglary, auto theft, vandalism)
- `articles:crime:drug` - Drug crime (trafficking, possession, drug busts)
- `articles:crime:organized` - Organized crime (cartels, racketeering, money laundering)
- `articles:crime:justice` - Criminal justice process (court cases, arrests, trials)

### `routes`
Many-to-many source→channel mappings with filters:
- `source_id` - Reference to source
- `channel_id` - Reference to channel
- `min_quality_score` - Minimum quality threshold (0-100)
- `topics` - JSON array of required topics
- `enabled` - Whether route is active

### `publish_history`
Audit trail of all published articles:
- `article_id` - Elasticsearch document ID
- `channel_name` - Redis channel name
- `route_id` - Route that triggered publication
- `quality_score` - Article quality score
- `published_at` - Publication timestamp

## Redis Message Format

Articles are published as JSON messages to Redis pub/sub channels. See [REDIS_MESSAGE_FORMAT.md](./docs/REDIS_MESSAGE_FORMAT.md) for complete format specification.

**Example message:**
```json
{
  "publisher": {
    "route_id": "a1b2c3d4-e5f6-4789-a0b1-c2d3e4f5g6h7",
    "published_at": "2025-12-28T15:30:45Z",
    "channel": "articles:crime:property"
  },
  "id": "es-doc-id-12345",
  "title": "Local Police Investigate Break-In",
  "body": "Full article text...",
  "canonical_url": "https://example.com/article",
  "quality_score": 85,
  "topics": ["property_crime", "local_news"],
  ...
}
```

**Note**: The `channel` field in the publisher metadata reflects the specific crime sub-category (e.g., `articles:crime:property`) rather than the generic `articles:crime`.

## Consumer Integration

The publisher publishes to Redis pub/sub channels. Consumers subscribe to channels and process articles according to their business logic.

**Laravel 12 Example:**
```php
use Illuminate\Support\Facades\Redis;

Redis::subscribe(['articles:crime'], function ($message) {
    $article = json_decode($message, true);
    // Process article...
});
```

See [CONSUMER_GUIDE.md](./docs/CONSUMER_GUIDE.md) for complete integration examples for Laravel 12, Node.js, Python, and more.

## Development

### Available Tasks

```bash
# Show all available tasks
task

# Build the service
task build

# Run API server
task run:api

# Run router service
task run:router

# Run tests
task test

# Run tests with coverage
task test:coverage

# Format code
task fmt

# Run code quality checks (fmt, vet, lint)
task check

# Clean build artifacts
task clean

# Download and tidy dependencies
task deps
```

### Building

```bash
task build
# Binary will be in ./bin/publisher
```

### Testing

```bash
# Run all tests
task test

# Run with coverage (generates coverage.html)
task test:coverage
```

## Logging

The service uses structured logging with two modes:

**Development Mode (`APP_DEBUG=true`):**
- Human-readable, colorized output
- Stack traces for errors
- Pretty-printed structured fields

**Production Mode (`APP_DEBUG=false`):**
- JSON-formatted output
- Optimized for log aggregation
- Stack traces only for errors

## Monitoring

The API provides real-time access to:
- Publishing statistics (total published, skipped, errors)
- Per-route statistics
- Publish history with pagination
- Health check endpoint

## Troubleshooting

### No Articles Published

1. **Check routes are enabled:**
   ```bash
   curl http://localhost:8070/api/v1/routes
   ```

2. **Verify Elasticsearch indexes exist:**
   ```bash
   curl http://localhost:9200/_cat/indices?v | grep classified_content
   ```

3. **Check router service logs:**
   ```bash
   docker logs north-cloud-publisher-router
   ```

4. **Verify Redis connectivity:**
   ```bash
   redis-cli PING
   ```

### Messages Not Received in Consumer

1. **Check Redis channel:**
   ```bash
   redis-cli SUBSCRIBE articles:crime
   ```

2. **Verify channel name matches:**
   - Route channel name must match consumer subscription

3. **Check publish history:**
   ```bash
   curl http://localhost:8070/api/v1/publish-history?limit=10
   ```

## Future Enhancements

- [ ] Webhook notifications for published articles
- [ ] Prometheus metrics export
- [ ] Support for Redis Streams (alternative to pub/sub)
- [ ] Article content enrichment hooks
- [ ] Scheduled publishing (delay between posts)

## License

MIT

## Contributing

Contributions welcome! Please open an issue or submit a pull request.
