# CLAUDE.md - AI Assistant Guide for North Cloud

This document provides a comprehensive guide for AI assistants working with the North Cloud codebase. It explains the multi-service architecture, conventions, and development workflows to help AI assistants make informed decisions when modifying or extending the system.

## Table of Contents

1. [Project Overview](#project-overview)
2. [Architecture & Services](#architecture--services)
3. [Directory Structure](#directory-structure)
4. [Key Conventions](#key-conventions)
5. [Development Workflow](#development-workflow)
6. [Docker Environment](#docker-environment)
7. [Common Tasks](#common-tasks)
8. [Important Guidelines for AI Assistants](#important-guidelines-for-ai-assistants)

---

## Project Overview

**North Cloud** is a microservices-based content management and publishing platform built with Go and Drupal. It crawls news content, manages sources, filters articles, and publishes them to a Drupal CMS.

### Purpose
- Crawl news websites for articles
- Manage content sources via web interface
- Filter and categorize articles (e.g., crime news)
- Publish filtered content to Drupal 11 CMS
- Provide a scalable, distributed architecture

### Tech Stack
- **Languages**: Go 1.24+ (crawler, source-manager), Go 1.25+ (publisher), PHP 8.2+, JavaScript (Vue.js)
- **Frameworks**: Gin (Go), Drupal 11 (PHP), Vue.js 3
- **Infrastructure**: Docker, PostgreSQL, Redis, Elasticsearch, Nginx
- **Build Tools**: Task (taskfile.dev), Composer, npm/Vite

### Key Features
- Multi-service microservices architecture
- Independent service databases (PostgreSQL)
- Shared infrastructure (Redis, Elasticsearch)
- Docker-based development and production environments
- Hot-reloading for development
- REST APIs for service communication
- Drupal JSON:API integration

---

## Architecture & Services

### System Overview

```
┌────────────────────────────────────────────────────────────────────────┐
│                         North Cloud Platform                            │
├────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────┐         ┌──────────────┐        ┌──────────────┐        │
│  │ Crawler  │────────▶│    Source    │        │  Classifier  │        │
│  │(crawler) │         │   Manager    │        │  (Go 1.25)   │        │
│  │          │         │  (Go + Vue)  │        │              │        │
│  └────┬─────┘         └──────────────┘        └──────▲───────┘        │
│       │                                                │                │
│       │ raw_content                                    │                │
│       │ (pending)                                      │ classified     │
│       ▼                                                │ _content       │
│  ┌─────────────────────────────────────────────────────┼──────────┐   │
│  │              Elasticsearch (Content Pipeline)       │          │   │
│  │  ┌────────────────┐  ┌──────────────┐  ┌──────────▼────────┐ │   │
│  │  │ {source}_raw   │─▶│  Classifier  │─▶│ {source}_classified│ │   │
│  │  │ _content       │  │   Service    │  │ _content          │ │   │
│  │  │ (pending)      │  │              │  │ (crime filtered)  │ │   │
│  │  └────────────────┘  └──────────────┘  └──────────┬────────┘ │   │
│  └─────────────────────────────────────────────────────┼──────────┘   │
│                                                         │               │
│                                                         ▼               │
│                                              ┌──────────────┐          │
│                                              │  Publisher   │          │
│                                              │  (gopost)    │          │
│                                              └──────┬───────┘          │
│                                                     │                   │
│                                                     ▼                   │
│                                            ┌──────────────┐            │
│                                            │  Streetcode  │            │
│                                            │ (Drupal 11)  │            │
│                                            └──────────────┘            │
│                                                                          │
│  Infrastructure:                                                        │
│  ┌─────────────┐  ┌─────────────┐  ┌──────────────────┐               │
│  │ PostgreSQL  │  │    Redis    │  │      Nginx       │               │
│  │  (3 DBs)    │  │   (Cache)   │  │  (Reverse Proxy) │               │
│  └─────────────┘  └─────────────┘  └──────────────────┘               │
└────────────────────────────────────────────────────────────────────────┘
```

### Content Processing Pipeline

The platform uses a **three-stage content pipeline** for intelligent article processing:

1. **Raw Content Indexing** (Crawler → Elasticsearch)
   - Crawler extracts articles and stores minimally-processed content
   - Indexed to `{source}_raw_content` with `classification_status=pending`
   - Preserves original HTML, text, metadata, and Open Graph tags

2. **Classification** (Classifier → Elasticsearch)
   - Classifier service processes raw content for:
     - **Content type detection** (article, page, video, etc.)
     - **Quality scoring** (0-100 based on completeness, metadata, etc.)
     - **Topic classification** (crime detection, category tagging)
     - **Source reputation** scoring
   - Enriched content indexed to `{source}_classified_content`
   - Includes all original fields plus classification metadata

3. **Publishing** (Publisher → Drupal)
   - Publisher queries classified_content indexes
   - Filters by `is_crime_related=true` and `quality_score >= threshold`
   - Posts high-quality, crime-related articles to Drupal via JSON:API
   - Redis deduplication prevents duplicate posts

### Service Descriptions

#### 1. **crawler** (crawler)
- **Location**: `/crawler`
- **Language**: Go 1.25+ (Backend), Vue.js 3 (Frontend)
- **Purpose**: Web crawler for scraping news articles
- **Database**: `postgres-crawler` (crawler database)
- **Ports**: 8060 (API), 3001 (Frontend - development)
- **Key Features**:
  - Configurable crawling rules
  - Article extraction and parsing
  - **Raw content indexing** to `{source}_raw_content` Elasticsearch indexes
  - Minimal processing: preserves HTML, text, metadata for downstream classification
  - Source management integration
  - Vue.js dashboard interface for monitoring
  - **Database-backed job scheduler** for dynamic crawling
  - REST API for job management (create, update, delete, list)
  - Support for immediate and scheduled (cron) jobs
  - Job status tracking (pending → processing → completed/failed)
- **Indexing Strategy**:
  - One index per source: `{source_name}_raw_content` (e.g., `example_com_raw_content`)
  - Documents marked with `classification_status=pending` for classifier pickup
  - Extracts: title, raw_text, raw_html, OG tags, metadata, published_date
- **Documentation**: See `/crawler/README.md`, `/crawler/frontend/README.md`, `/crawler/docs/DATABASE_SCHEDULER.md`

#### 2. **source-manager**
- **Location**: `/source-manager`
- **Language**: Go 1.24+ (Backend), Vue.js 3 (Frontend)
- **Purpose**: Manage content sources and crawling configurations
- **Database**: `postgres-source-manager` (gosources database)
- **Ports**: 8050 (API)
- **Key Features**:
  - REST API for source management
  - Vue.js web interface
  - Source CRUD operations
  - Crawling schedule configuration
- **Documentation**: See `/source-manager/README.md`, `/source-manager/DEVELOPMENT.md`

#### 3. **classifier**
- **Location**: `/classifier`
- **Language**: Go 1.25+
- **Purpose**: Classify and enrich raw content with metadata and quality scores
- **Dependencies**: Elasticsearch (reads raw_content, writes classified_content)
- **Ports**: 8070 (HTTP API)
- **Key Features**:
  - **Content type classification**: Identifies articles, pages, videos, jobs, etc.
  - **Quality scoring**: 0-100 score based on completeness, metadata, word count
  - **Topic classification**: Crime detection, category tagging with confidence scores
  - **Source reputation**: Tracks and scores source quality over time
  - **REST API**: `/api/v1/classify` for on-demand classification
  - HTTP server for real-time classification requests
  - Batch processing support for bulk classification
- **Processing Pipeline**:
  1. Polls `{source}_raw_content` indexes for `classification_status=pending`
  2. Applies classification algorithms (content type, quality, topics, reputation)
  3. Indexes enriched content to `{source}_classified_content`
  4. Updates `classification_status=classified` on success
- **Output Fields**:
  - All original RawContent fields (title, raw_text, url, metadata)
  - `content_type`, `quality_score`, `is_crime_related`, `topics`
  - `source_reputation`, `source_category`, `confidence`
  - **Alias fields for publisher**: `body` (alias for raw_text), `source` (alias for url)
- **Documentation**: See `/classifier/README.md`, `/classifier/CLAUDE.md`

#### 4. **publisher** (gopost)
- **Location**: `/publisher`
- **Language**: Go 1.25+
- **Purpose**: Filter and publish articles from Elasticsearch to Drupal
- **Dependencies**: Elasticsearch, Redis, Drupal
- **Key Features**:
  - **Dual-mode operation**: Legacy keyword-based OR classifier-based filtering
  - **Classification-aware filtering** (when enabled):
    - Queries `{source}_classified_content` indexes
    - Filters by `is_crime_related=true` flag
    - Quality threshold filtering (`quality_score >= min_quality_score`)
    - Trusts classifier determinations for accuracy
  - **Legacy mode** (when disabled):
    - Queries `{source}_articles` indexes
    - Keyword-based crime detection
  - Drupal JSON:API integration
  - Redis-based deduplication
  - Multi-city support
  - Rate limiting
- **Configuration**:
  - `use_classified_content: true/false` - Enable classifier-based filtering
  - `min_quality_score: 50` - Minimum quality threshold (0-100)
  - `index_suffix: "_classified_content"` - Index naming pattern
- **Documentation**: See `/publisher/CLAUDE.md`, `/publisher/README.md`

#### 5. **streetcode** (Drupal 11)
- **Location**: `/streetcode`
- **Language**: PHP 8.2+
- **Purpose**: Content management and public website
- **Database**: `postgres-streetcode` (streetcode database)
- **Ports**: 8080 (Web interface)
- **Key Features**:
  - Drupal 11 CMS
  - JSON:API for content ingestion
  - Group-based content organization
  - Custom content types (articles, crime news)
- **Documentation**: See `/streetcode/docs/`

### Infrastructure Services

#### PostgreSQL Databases
- **postgres-source-manager**: Source manager database (gosources)
- **postgres-crawler**: Crawler database (crawler)
- **postgres-streetcode**: Drupal database (streetcode)
- Each service has its own isolated database

#### Elasticsearch
- **Purpose**: Content pipeline storage and search
- **Port**: 9200
- **Index Patterns**:
  - **Raw content**: `{source}_raw_content` (e.g., `example_com_raw_content`)
    - Minimally-processed crawled content with `classification_status=pending`
  - **Classified content**: `{source}_classified_content` (e.g., `example_com_classified_content`)
    - Enriched content with quality scores, topics, crime detection
  - **Legacy articles** (deprecated): `{source}_articles`
    - Old article format, being phased out in favor of classified_content
- **Content Flow**: raw_content (pending) → Classifier → classified_content → Publisher → Drupal

#### Redis
- **Purpose**: Deduplication, caching, queue management
- **Port**: 6379
- **Usage**: Publisher service for tracking posted articles

#### Nginx
- **Purpose**: Reverse proxy and load balancer with SSL/TLS termination
- **Ports**: 80 (HTTP), 443 (HTTPS)
- **Configuration**: `/infrastructure/nginx/`
- **SSL/TLS**: Let's Encrypt certificates with automatic renewal
- **Features**:
  - HTTP to HTTPS redirect (301 Permanent)
  - Modern TLS protocols (TLSv1.2, TLSv1.3)
  - Security headers (HSTS, X-Frame-Options, etc.)
  - HTTP/2 support
  - ACME challenge endpoint for certificate validation

#### SSL/TLS Certificate Management
- **Certificate Authority**: Let's Encrypt
- **Domain**: northcloud.biz
- **Validation Method**: HTTP-01 (webroot)
- **Renewal**: Automatic (certbot service checks every 12 hours)
- **Certificate Validity**: 90 days
- **Storage**: Docker volumes (`certbot_etc`, `certbot_www`)
- **Documentation**: `/infrastructure/certbot/README.md`
- **Monitoring**: Certificate expiry check script available
- **Manual Renewal**: Use scripts in `/infrastructure/certbot/scripts/`

---

## Directory Structure

```
north-cloud/
├── docker-compose.base.yml       # Base infrastructure services
├── docker-compose.dev.yml        # Development overrides
├── docker-compose.prod.yml       # Production overrides
├── .env.example                  # Environment variables template
├── .env                          # Environment variables (not committed)
├── README.md                     # User documentation
├── DOCKER.md                     # Docker documentation
├── CLAUDE.md                     # This file (AI assistant guide)
│
├── crawler/                      # Web crawler service
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   ├── cmd/
│   │   ├── httpd/               # HTTP API server with job scheduler
│   │   ├── crawl/               # Manual crawl command
│   │   └── scheduler/           # Legacy scheduler (deprecated)
│   ├── internal/
│   │   ├── job/                 # Job scheduler implementation
│   │   │   └── db_scheduler.go  # Database-backed scheduler
│   │   ├── database/            # Database layer (PostgreSQL)
│   │   ├── domain/              # Domain models (Job, Item)
│   │   └── api/                 # REST API handlers
│   ├── frontend/                # Vue.js dashboard
│   │   └── src/
│   │       └── views/
│   │           └── CrawlJobsView.vue  # Job management UI
│   ├── docs/
│   │   └── DATABASE_SCHEDULER.md  # Scheduler documentation
│   ├── tests/
│   └── README.md
│
├── source-manager/               # Source management service
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   ├── internal/
│   ├── frontend/                 # Vue.js application
│   │   ├── src/
│   │   ├── package.json
│   │   └── vite.config.js
│   ├── migrations/
│   ├── README.md
│   └── DEVELOPMENT.md
│
├── classifier/                   # Content classification service
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   ├── cmd/
│   │   ├── httpd/               # HTTP API server
│   │   └── processor/           # Batch processor
│   ├── internal/
│   │   ├── classifier/          # Classification algorithms
│   │   │   ├── classifier.go
│   │   │   ├── content_type.go
│   │   │   ├── quality.go
│   │   │   ├── topics.go
│   │   │   └── reputation.go
│   │   ├── domain/              # Domain models
│   │   │   ├── raw_content.go
│   │   │   └── classification.go
│   │   ├── storage/             # Elasticsearch integration
│   │   └── api/                 # REST API handlers
│   ├── tests/
│   ├── README.md
│   └── CLAUDE.md                # Classifier-specific AI guide
│
├── publisher/                    # Article publishing service
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   ├── internal/
│   ├── cmd/
│   ├── README.md
│   └── CLAUDE.md                # Publisher-specific AI guide
│
├── streetcode/                   # Drupal 11 CMS
│   ├── Dockerfile
│   ├── composer.json
│   ├── web/
│   ├── config/
│   └── docs/                    # Drupal documentation
│       ├── API_SECURITY_GUIDE.md
│       ├── PAYLOAD_TO_DRUPAL_MAPPING.md
│       └── ARTICLE_FIELDS_GUIDE.md
│
└── infrastructure/               # Shared infrastructure configs
    ├── nginx/
    │   └── nginx.conf           # Main nginx configuration with SSL/TLS
    ├── elasticsearch/
    ├── postgres/
    └── certbot/                 # SSL/TLS certificate management
        ├── README.md            # Comprehensive SSL documentation
        ├── QUICK_REFERENCE.md   # Quick command reference
        └── scripts/
            ├── check-cert-expiry.sh      # Monitor certificate expiration
            ├── renew-and-reload.sh       # Manual renewal with nginx reload
            └── reload-nginx.sh           # Simple nginx reload script
```

---

## Key Conventions

### 1. Code Style

#### Go Services (crawler, source-manager, classifier, publisher)
- **Standards**: Follow standard Go formatting (`gofmt`, `goimports`)
- **Go Version**: 1.24+ (crawler, source-manager), 1.25+ (classifier, publisher)
- **Error Handling**: Always wrap errors with context using `fmt.Errorf("context: %w", err)`
- **Logging**: Use structured logging (zap for publisher, configure per service)
- **Testing**: Unit tests with 80%+ coverage target
- **Linting**: Use `golangci-lint` with service-specific configurations

#### Go 1.25 Features
The codebase leverages Go 1.25 improvements:

- **Container-Aware GOMAXPROCS**: Go 1.25 automatically adjusts `GOMAXPROCS` based on container CPU quotas. This means:
  - No manual `GOMAXPROCS` configuration needed in Docker containers
  - Automatic CPU utilization optimization
  - Better resource utilization in containerized environments
  - Works seamlessly with Docker CPU limits and Kubernetes resource requests

- **Built-in CSRF Protection**: The `net/http` package now includes Cross-Site Request Forgery (CSRF) protection:
  - Available for HTTP servers using the standard library
  - Can be enabled for additional security in web services
  - Consider evaluating for services exposing web interfaces
  - See [Go 1.25 release notes](https://go.dev/doc/go1.25) for implementation details

**Note**: These features are automatic and require no code changes. The container-aware GOMAXPROCS is particularly beneficial for microservices running in Docker/Kubernetes environments.

#### PHP Service (streetcode/Drupal)
- **Standards**: Follow Drupal coding standards
- **PHP Version**: 8.2+
- **Composer**: Use for dependency management
- **Configuration**: Export config to `/config` directory
- **Custom Modules**: Place in `/web/modules/custom`

#### Frontend (source-manager/frontend)
- **Framework**: Vue.js 3 with Composition API
- **Build Tool**: Vite
- **Styling**: Component-scoped CSS or Tailwind
- **State Management**: Pinia (if needed)

### 2. Environment Variables

#### Naming Convention
- Uppercase with underscores (e.g., `DRUPAL_TOKEN`)
- Service-specific prefixes (e.g., `SOURCE_MANAGER_PORT`)
- Common variables:
  - `APP_DEBUG`: Enable debug mode (true/false)
  - `*_PORT`: Service ports
  - `POSTGRES_*_USER`: Database users
  - `POSTGRES_*_PASSWORD`: Database passwords
  - `DRUPAL_TOKEN`: Drupal API authentication

#### Configuration Priority
1. Environment variables (highest priority)
2. `.env` file
3. Service config files (config.yml, etc.)
4. Hardcoded defaults (lowest priority)

### 3. Docker Configuration

#### File Naming
- `docker-compose.base.yml`: Shared infrastructure
- `docker-compose.dev.yml`: Development overrides
- `docker-compose.prod.yml`: Production overrides
- Service Dockerfiles: `{service}/Dockerfile`

#### Container Naming
- Format: `north-cloud-{service-name}`
- Examples: `north-cloud-crawler`, `north-cloud-source-manager`

#### Volume Mounts (Development)
- Source code mounted for hot-reloading
- Database volumes for persistence
- Configuration files mounted as needed

### 4. Database Conventions

#### Naming
- Database names: `gosources`, `crawler`, `streetcode`
- Container names: `postgres-{service}`
- Port exposure: 5432 (internal), mapped externally if needed

#### Migrations
- Go services: Use golang-migrate or custom migration system
- Drupal: Use config export/import
- Place migrations in service-specific directories

### 5. API Conventions

#### REST APIs (Go services)
- **Framework**: Gin (recommended)
- **Format**: JSON
- **Versioning**: `/api/v1/` prefix
- **Authentication**: Token-based or Basic Auth
- **Error Responses**: Consistent JSON format

#### Drupal JSON:API
- **Endpoint**: `/jsonapi`
- **Content Type**: `application/vnd.api+json`
- **Authentication**: Multiple methods supported (API-KEY, Basic, miniOrange)
- **Format**: Follow JSON:API specification
- See `/publisher/CLAUDE.md` for detailed JSON:API conventions

### 6. Logging Conventions

#### Log Levels
- **Debug**: Detailed troubleshooting (queries, payloads)
- **Info**: Important business events (service start, operations completed)
- **Warn**: Non-critical issues (deprecations, fallbacks)
- **Error**: Failures requiring attention (API errors, connection failures)

#### Field Naming
- **Always use snake_case** for structured log fields
- Common fields:
  - `service`: Service name
  - `method`: Function/method name
  - `duration`: Operation duration
  - `error`: Error message
  - `status_code`: HTTP status

#### Debug Mode
- Development: `APP_DEBUG=true` (human-readable logs)
- Production: `APP_DEBUG=false` (JSON logs)

---

## Development Workflow

### 1. Initial Setup

```bash
# Clone repository
git clone <repository-url>
cd north-cloud

# Copy environment template
cp .env.example .env

# Edit environment variables
nano .env

# Start development environment
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d
```

### 2. Development Mode

#### Starting Services

```bash
# Start all services (development)
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d

# Start specific service
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d source-manager

# Start only infrastructure
docker-compose -f docker-compose.base.yml up -d
```

#### Viewing Logs

```bash
# All services
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f

# Specific service
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f publisher

# Tail last 100 lines
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml logs --tail=100 -f
```

#### Stopping Services

```bash
# Stop all services
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml down

# Stop and remove volumes (⚠️ deletes data)
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml down -v
```

### 3. Working on Individual Services

#### Crawler
```bash
cd crawler

# Start HTTP server with job scheduler
go run main.go

# Frontend (separate terminal)
cd frontend
npm install
npm run dev

# Run tests
go test ./...
cd frontend && npm test

# Build
go build -o bin/crawler main.go
```

**Job Scheduler Notes**:
- The service automatically starts the database-backed job scheduler on startup
- Jobs can be managed via REST API at `http://localhost:8060/api/v1/jobs`
- The scheduler processes both immediate and scheduled (cron) jobs
- See `/crawler/docs/DATABASE_SCHEDULER.md` for detailed usage

#### Source Manager
```bash
cd source-manager

# Backend (API)
go run main.go

# Frontend (separate terminal)
cd frontend
npm install
npm run dev

# Run tests
go test ./...
cd frontend && npm test
```

#### Classifier
```bash
cd classifier

# Run HTTP server (REST API)
go run main.go httpd

# Run batch processor (polls raw_content indexes)
go run main.go processor

# Run tests
go test ./...

# Run with coverage
go test -cover ./...

# Lint
golangci-lint run

# Build
go build -o bin/classifier main.go
```

**Classification Notes**:
- The `httpd` command starts the REST API server on port 8070
- The `processor` command runs continuous batch classification
- Processes raw_content with `classification_status=pending`
- Outputs to classified_content indexes
- See `/classifier/README.md` for API usage examples

#### Publisher
```bash
cd publisher

# Run locally
task run

# Run tests
task test

# Run with coverage
task test:coverage

# Lint
task lint

# See publisher/CLAUDE.md for detailed commands
```

#### Streetcode (Drupal)
```bash
# Access container
docker exec -it north-cloud-streetcode bash

# Drupal commands (inside container)
drush cr                    # Clear cache
drush cex                   # Export configuration
drush cim                   # Import configuration
drush updb                  # Update database
```

### 4. Database Access

```bash
# Source manager database
docker exec -it north-cloud-postgres-source-manager psql -U postgres -d gosources

# Crawler database
docker exec -it north-cloud-postgres-crawler psql -U postgres -d crawler

# Streetcode database
docker exec -it north-cloud-postgres-streetcode psql -U postgres -d streetcode
```

### 5. Building Images

```bash
# Build all services (development)
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml build

# Build specific service
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml build publisher

# Build for production
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml build
```

---

## Docker Environment

### Development vs Production

#### Development Mode (`docker-compose.dev.yml`)
- **Purpose**: Local development with hot-reloading
- **Features**:
  - Source code mounted as volumes
  - `APP_DEBUG=true` for detailed logs
  - All ports exposed for local access
  - Development dependencies included
  - Fast iteration cycles
- **Usage**:
  ```bash
  docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d
  ```

#### Production Mode (`docker-compose.prod.yml`)
- **Purpose**: Production deployment
- **Features**:
  - Code baked into images (no volume mounts)
  - `APP_DEBUG=false` for optimized logging
  - Resource limits configured
  - Security hardening enabled
  - `restart: always` policy
  - SSL/TLS ready
- **Usage**:
  ```bash
  docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d
  ```

### Service Ports (Development)

| Service | Internal Port | External Port | Description |
|---------|---------------|---------------|-------------|
| crawler | 8060 | 8060 | Crawler API |
| crawler-frontend | 3000 | 3001 | Crawler Dashboard UI |
| source-manager | 8050 | 8050 | Source Manager API |
| source-manager-frontend | 3000 | 3000 | Source Manager UI |
| classifier | 8070 | 8070 | Classifier HTTP API |
| publisher | 8080 | 8080 | Publisher API (if enabled) |
| streetcode | 80 | 8090 | Drupal web interface |
| nginx | 80 | 80 | Reverse proxy |
| elasticsearch | 9200 | 9200 | Elasticsearch API |
| redis | 6379 | 6379 | Redis cache |
| postgres-* | 5432 | - | PostgreSQL (internal only) |

### Docker Compose Commands

```bash
# Use shorter alias for development
alias dc-dev='docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml'
alias dc-prod='docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml'

# Then use:
dc-dev up -d
dc-dev logs -f
dc-dev down
```

---

## Common Tasks

### Adding a New Service

1. **Create Service Directory**:
   ```bash
   mkdir new-service
   cd new-service
   ```

2. **Create Dockerfile**:
   ```dockerfile
   FROM golang:1.24-alpine AS builder
   WORKDIR /app
   COPY . .
   RUN go build -o bin/app main.go

   FROM alpine:latest
   COPY --from=builder /app/bin/app /app
   CMD ["/app"]
   ```

3. **Add to docker-compose.base.yml**:
   ```yaml
   new-service:
     build: ./new-service
     depends_on:
       - postgres-new-service
     environment:
       - DATABASE_URL=${NEW_SERVICE_DB_URL}
   ```

4. **Add Database** (if needed):
   ```yaml
   postgres-new-service:
     image: postgres:16-alpine
     environment:
       POSTGRES_DB: new_service
       POSTGRES_USER: ${POSTGRES_NEW_SERVICE_USER}
       POSTGRES_PASSWORD: ${POSTGRES_NEW_SERVICE_PASSWORD}
   ```

5. **Update .env.example**:
   ```bash
   # New Service
   NEW_SERVICE_PORT=8060
   POSTGRES_NEW_SERVICE_USER=postgres
   POSTGRES_NEW_SERVICE_PASSWORD=changeme
   ```

### Updating a Service

1. **Make Code Changes**: Edit files in service directory
2. **Test Locally** (if applicable): `go test ./...` or service-specific tests
3. **Rebuild Container**:
   ```bash
   docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml build service-name
   ```
4. **Restart Service**:
   ```bash
   docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d service-name
   ```
5. **Check Logs**:
   ```bash
   docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f service-name
   ```

### Managing Crawler Jobs

The crawler service includes a database-backed job scheduler for dynamic crawling. Jobs can be managed via REST API or the Vue.js frontend.

#### Creating Jobs via API

**Immediate Job (Run Once, Now)**:
```bash
curl -X POST http://localhost:8060/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "source_id": "news-site",
    "source_name": "example.com",
    "url": "https://example.com",
    "schedule_enabled": false
  }'
```

**Scheduled Job (Cron)**:
```bash
curl -X POST http://localhost:8060/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "source_id": "news-site",
    "source_name": "example.com",
    "url": "https://example.com",
    "schedule_time": "0 */6 * * *",
    "schedule_enabled": true
  }'
```

#### Listing Jobs
```bash
# All jobs
curl http://localhost:8060/api/v1/jobs

# Filter by status
curl http://localhost:8060/api/v1/jobs?status=pending
curl http://localhost:8060/api/v1/jobs?status=completed
curl http://localhost:8060/api/v1/jobs?status=processing
curl http://localhost:8060/api/v1/jobs?status=failed
```

#### Updating Jobs
```bash
curl -X PUT http://localhost:8060/api/v1/jobs/{job-id} \
  -H "Content-Type: application/json" \
  -d '{
    "schedule_time": "0 * * * *",
    "schedule_enabled": true
  }'
```

#### Deleting Jobs
```bash
curl -X DELETE http://localhost:8060/api/v1/jobs/{job-id}
```

#### Common Cron Expressions
- `0 * * * *` - Every hour
- `*/30 * * * *` - Every 30 minutes
- `0 0 * * *` - Daily at midnight
- `0 9,17 * * 1-5` - 9 AM and 5 PM on weekdays
- `0 */6 * * *` - Every 6 hours

**Important Notes**:
- Jobs require `source_name` field to match an existing source configuration
- Immediate jobs (`schedule_enabled: false`) execute within 10 seconds
- Scheduled jobs reload every 5 minutes or immediately after creation/update
- Job status transitions: `pending` → `processing` → `completed`/`failed`
- See `/crawler/docs/DATABASE_SCHEDULER.md` for comprehensive documentation

### Running Database Migrations

#### Go Services
```bash
# Inside service directory
go run cmd/migrate/main.go up

# Or via Docker
docker exec -it north-cloud-service-name /app/migrate up
```

#### Drupal
```bash
docker exec -it north-cloud-streetcode drush updb
docker exec -it north-cloud-streetcode drush cex
```

### Debugging Issues

#### Service Won't Start
1. Check logs: `docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml logs service-name`
2. Check environment variables: `docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml config`
3. Verify dependencies: Check `depends_on` and ensure dependent services are healthy
4. Check port conflicts: `netstat -tulpn | grep PORT`

#### Database Connection Issues
1. Verify database is running: `docker ps | grep postgres`
2. Check connection string: Review `.env` file
3. Test connection: `docker exec -it north-cloud-postgres-service psql -U user -d database`
4. Check health: `docker inspect north-cloud-postgres-service`

#### Cannot Access Service
1. Verify port mapping: `docker ps | grep service-name`
2. Check firewall: `sudo ufw status`
3. Verify service is listening: `docker exec -it north-cloud-service-name netstat -tulpn`
4. Check nginx configuration (if using reverse proxy)

### Managing SSL/TLS Certificates

#### Certificate Status Check
```bash
# Quick expiry check
bash infrastructure/certbot/scripts/check-cert-expiry.sh

# View detailed certificate information
docker run --rm -v north-cloud_certbot_etc:/etc/letsencrypt \
  certbot/certbot certificates
```

#### Manual Certificate Renewal
```bash
# Recommended: Renew and reload nginx automatically
bash infrastructure/certbot/scripts/renew-and-reload.sh

# Alternative: Manual steps
docker run --rm --network north-cloud_north-cloud-network \
  -v north-cloud_certbot_etc:/etc/letsencrypt \
  -v north-cloud_certbot_www:/var/www/certbot \
  certbot/certbot renew

# Then reload nginx
docker exec north-cloud-nginx nginx -s reload
```

#### Test Certificate Renewal
```bash
# Dry-run to test renewal without actually renewing
docker run --rm --network north-cloud_north-cloud-network \
  -v north-cloud_certbot_etc:/etc/letsencrypt \
  -v north-cloud_certbot_www:/var/www/certbot \
  certbot/certbot renew --dry-run
```

#### View Certbot Logs
```bash
# View renewal service logs
docker logs north-cloud-certbot

# Check certbot service status
docker ps | grep certbot
```

**Important Notes**:
- Certbot service automatically checks for renewal every 12 hours
- Certificates renew when 30 days or less until expiration
- After automatic renewal, manually reload nginx: `docker exec north-cloud-nginx nginx -s reload`
- Email alerts sent to jonesrussell42@gmail.com at 20 days before expiry
- See `/infrastructure/certbot/README.md` for comprehensive documentation

### Cleaning Up

```bash
# Stop all services
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml down

# Remove volumes (⚠️ deletes all data)
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml down -v

# Remove images
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml down --rmi all

# Clean up Docker system
docker system prune -a --volumes
```

---

## Important Guidelines for AI Assistants

### When Working with This Codebase

#### 1. Understand Service Boundaries
- **Each service is independent**: Don't make cross-service changes without understanding dependencies
- **Check service-specific documentation**: Look for README.md or CLAUDE.md in each service directory
- **Respect API contracts**: Don't break existing APIs without migration plans
- **Database isolation**: Each service has its own database; avoid cross-database queries

#### 2. Before Making Changes

**Always Read First**:
1. Read the service's README.md or CLAUDE.md
2. Read the specific files you'll modify
3. Understand existing patterns and conventions
4. Check for related tests

**Check Dependencies**:
1. Review `depends_on` in docker-compose files
2. Check API integrations between services
3. Verify database schema dependencies
4. Review environment variable requirements

**Plan Multi-Service Changes**:
1. Identify all affected services
2. Determine change order (dependencies first)
3. Plan backward compatibility
4. Consider migration paths

#### 3. Service-Specific Guidelines

**For Crawler**:
- Follow crawler-specific patterns
- Test crawling logic thoroughly
- Validate Elasticsearch indexing
- Check source manager integration
- **Job Scheduler**:
  - Use database-backed scheduler (httpd command) for dynamic job management
  - Jobs require `source_name` field to match existing source
  - Support both immediate (`schedule_enabled: false`) and cron-scheduled jobs
  - Job status tracked: `pending` → `processing` → `completed`/`failed`
  - Scheduler automatically reloads jobs every 5 minutes
  - See `/crawler/docs/DATABASE_SCHEDULER.md` for implementation details

**For Source Manager**:
- Backend: Follow Go REST API conventions
- Frontend: Use Vue 3 Composition API
- Test API endpoints
- Validate database migrations

**For Classifier**:
- **IMPORTANT**: Read `/classifier/CLAUDE.md` for detailed guidelines
- Processes raw_content → classified_content pipeline
- **Index Requirements**:
  - Input: `{source}_raw_content` with `classification_status=pending`
  - Output: `{source}_classified_content` with enriched fields
- **Classification Components**:
  - Content type: Use OG tags, selectors, heuristics
  - Quality: Score 0-100 based on completeness, metadata, word count
  - Topics: Crime detection via keywords, patterns, ML models
  - Reputation: Track source quality over time
- **Publisher Compatibility**:
  - Must populate `Body` and `Source` alias fields
  - Ensures `is_crime_related` flag is accurate
  - Quality scores must be consistent (0-100 scale)
- Test classification accuracy with known articles
- Validate Elasticsearch indexing for both input and output

**For Publisher**:
- **IMPORTANT**: Read `/publisher/CLAUDE.md` for detailed guidelines
- **Dual-mode operation**: Legacy keyword-based OR classifier-based
- **When using classified_content** (`use_classified_content: true`):
  - Query `{source}_classified_content` indexes
  - Filter by `is_crime_related=true` and `quality_score >= threshold`
  - Trust classifier's determinations (don't re-check keywords)
  - Use configured `index_suffix` for index naming
- **Legacy mode** (`use_classified_content: false`):
  - Query `{source}_articles` indexes
  - Use keyword-based crime detection
- Follow structured logging conventions (snake_case fields)
- Test Drupal JSON:API integration
- Verify Redis deduplication
- Respect rate limits

**For Streetcode (Drupal)**:
- Follow Drupal coding standards
- Export configuration changes: `drush cex`
- Test JSON:API endpoints
- Validate content types and fields
- Check permissions and access control

#### 4. Docker and Environment

**Development**:
- Always use `docker-compose.base.yml` + `docker-compose.dev.yml`
- Mount source code for hot-reloading
- Set `APP_DEBUG=true`
- Expose ports for testing

**Production** (if deploying):
- Use `docker-compose.base.yml` + `docker-compose.prod.yml`
- Build images with code baked in
- Set `APP_DEBUG=false`
- Configure resource limits
- Enable SSL/TLS
- Set strong passwords

**Environment Variables**:
- Always update `.env.example` when adding new variables
- Use sensible defaults where possible
- Document required vs optional variables
- Never commit secrets to `.env`

#### 5. Testing Requirements

**Before Committing**:
1. Run service-specific tests: `go test ./...` or `npm test`
2. Rebuild affected Docker images
3. Test service in Docker environment
4. Check logs for errors
5. Verify integration with dependent services

**Integration Testing**:
1. Start all required services
2. Test API endpoints
3. Verify database operations
4. Check Elasticsearch indexing (if applicable)
5. Validate Drupal posting (if applicable)

#### 6. Git Workflow

**Branch Naming**:
- Must start with `claude/`
- Must end with session ID
- Format: `claude/{description}-{session-id}`
- Example: `claude/create-claude-md-01YMXWZpqv3utVH69jyNnLaE`

**Committing Changes**:
1. Clear, descriptive commit messages
2. Explain "why" not just "what"
3. Reference related issues or tasks
4. Group related changes
5. Commit often, push when complete

**Pushing**:
- Always use: `git push -u origin {branch-name}`
- Retry on network failures (up to 4 times, exponential backoff: 2s, 4s, 8s, 16s)
- Never force push to main/master

#### 7. Documentation Updates

**When Making Changes, Update**:
1. This CLAUDE.md for architectural changes
2. README.md for user-facing changes
3. Service-specific CLAUDE.md or README.md
4. Docker documentation (DOCKER.md)
5. API documentation
6. Code comments for complex logic
7. `.env.example` for new environment variables

**Documentation Standards**:
- Use clear, concise language
- Include code examples
- Document gotchas and common issues
- Keep table of contents updated
- Use proper markdown formatting

#### 8. Common Pitfalls to Avoid

**Multi-Service Changes**:
- Don't modify multiple services without testing each
- Maintain backward compatibility during transitions
- Use feature flags for gradual rollouts
- Document service dependencies

**Database Changes**:
- Create migrations, don't modify schema directly
- Test migrations both up and down
- Backup data before destructive changes
- Coordinate schema changes with code changes

**Docker Issues**:
- Don't mix dev and prod configurations
- Always specify both base and environment-specific compose files
- Clean up volumes when schema changes
- Check for port conflicts

**Environment Variables**:
- Don't hardcode values that should be configurable
- Provide defaults for optional settings
- Validate required variables on startup
- Document all variables

**Authentication and Security**:
- Never commit credentials
- Use environment variables for secrets
- Validate authentication in all services
- Follow security best practices (see service docs)

### 9. Service Communication Patterns

**REST APIs**:
- Use consistent error responses
- Include request IDs for tracing
- Implement timeouts
- Handle partial failures gracefully

**Elasticsearch**:
- Use appropriate index names (per city)
- Validate documents before indexing
- Handle search errors gracefully
- Use bulk operations where possible

**Redis**:
- Use consistent key naming (`service:type:id`)
- Set appropriate TTLs
- Handle cache misses
- Don't rely on cache for critical data

**Databases**:
- Use connection pooling
- Implement retries with backoff
- Handle deadlocks and conflicts
- Use transactions for multi-step operations

### 10. Performance Considerations

**Go Services**:
- Use appropriate timeouts
- Implement rate limiting
- Pool connections (HTTP, DB, Redis)
- Use context for cancellation
- Profile before optimizing

**Drupal**:
- Use caching appropriately
- Optimize database queries
- Use JSON:API filtering
- Consider pagination for large datasets

**Docker**:
- Set appropriate resource limits
- Use multi-stage builds for smaller images
- Minimize layers
- Use .dockerignore

---

## Additional Resources

### External Documentation
- [Docker Compose](https://docs.docker.com/compose/)
- [Go Documentation](https://go.dev/doc/)
- [Drupal JSON:API](https://www.drupal.org/docs/core-modules-and-themes/core-modules/jsonapi-module)
- [Elasticsearch Documentation](https://www.elastic.co/guide/index.html)
- [Vue.js Documentation](https://vuejs.org/)

### Internal Documentation
- `/README.md`: User-facing overview
- `/DOCKER.md`: Docker setup and configuration
- `/infrastructure/certbot/README.md`: SSL/TLS certificate management guide
- `/infrastructure/certbot/QUICK_REFERENCE.md`: SSL quick reference
- `/publisher/CLAUDE.md`: Publisher service AI guide
- `/source-manager/README.md`: Source manager documentation
- `/source-manager/DEVELOPMENT.md`: Source manager development guide
- `/streetcode/docs/`: Drupal-specific documentation

### Service Documentation by Feature

**Content Crawling**:
- `/crawler/README.md`
- `/source-manager/README.md`
- `/crawler/docs/DATABASE_SCHEDULER.md` - Database-backed job scheduler

**Job Scheduling & Management**:
- `/crawler/docs/DATABASE_SCHEDULER.md` - Comprehensive scheduler guide
  - Architecture and implementation details
  - API usage examples (create, update, delete, list jobs)
  - Cron expression syntax and examples
  - Job status tracking and lifecycle
  - Immediate vs scheduled jobs
  - Troubleshooting and best practices

**Content Classification**:
- `/classifier/README.md`
- `/classifier/CLAUDE.md` - Classifier-specific AI guide
  - Classification algorithms and strategies
  - Quality scoring methodology
  - Topic detection and crime classification
  - Publisher compatibility requirements

**Content Publishing**:
- `/publisher/CLAUDE.md`
- `/publisher/README.md`
- `/streetcode/docs/API_SECURITY_GUIDE.md`
- `/streetcode/docs/PAYLOAD_TO_DRUPAL_MAPPING.md`

**SSL/TLS & Security**:
- `/infrastructure/certbot/README.md` - Comprehensive SSL certificate management guide
  - Initial certificate setup
  - Automatic renewal configuration
  - Manual operations and troubleshooting
  - Certificate monitoring and alerts
  - Security best practices
- `/infrastructure/certbot/QUICK_REFERENCE.md` - Quick command reference
- `/infrastructure/nginx/nginx.conf` - Nginx SSL/TLS configuration

**Development Setup**:
- `/DOCKER.md`
- `/source-manager/DEVELOPMENT.md`

---

## Questions or Clarifications?

When encountering scenarios not covered in this guide:

1. **Check service-specific documentation**: Each service may have its own CLAUDE.md or README.md
2. **Review existing code patterns**: Look at similar functionality in the same service
3. **Examine test files**: Tests often show usage examples
4. **Check git history**: `git log -p path/to/file` for context on changes
5. **Look for comments**: Code comments often explain "why" not just "what"
6. **Ask for clarification**: When assumptions are needed, ask the user

**Remember**:
- Consistency with existing codebase is critical
- Each service may have different conventions
- Read before modifying
- Test thoroughly across service boundaries
- Document your changes

---

## Version History

- **SSL/TLS Implementation** (2025-12-25): Production SSL/TLS setup for northcloud.biz
  - **Let's Encrypt integration**: Automated certificate management with certbot
  - **Nginx SSL/TLS configuration**:
    - HTTPS on port 443 with HTTP/2 support
    - HTTP to HTTPS redirect (301 Permanent)
    - Modern TLS protocols (TLSv1.2, TLSv1.3)
    - Security headers (HSTS, X-Frame-Options, X-Content-Type-Options, X-XSS-Protection)
    - ACME challenge endpoint for certificate validation
  - **Certbot service**:
    - Automatic renewal checks every 12 hours
    - Certificates renew 30 days before expiration
    - Email alerts at 20 days before expiry
    - Docker volume-based certificate storage
  - **Certificate management tools**:
    - Certificate expiry monitoring script (`check-cert-expiry.sh`)
    - Automated renewal with nginx reload (`renew-and-reload.sh`)
    - Comprehensive documentation (`/infrastructure/certbot/README.md`)
    - Quick reference guide (`/infrastructure/certbot/QUICK_REFERENCE.md`)
  - **Production deployment**:
    - Certificate obtained for northcloud.biz
    - Valid until March 25, 2026 (90 days)
    - HTTP-01 (webroot) validation method
    - Shared Docker volumes for nginx/certbot integration
  - **Documentation updates**: SSL/TLS section added to CLAUDE.md and infrastructure docs

- **Raw Content Pipeline Implementation** (2025-12-23): Replaced article indexing with raw_content pipeline
  - **Classifier service integration**: New microservice for content classification
  - **Three-stage pipeline**: Crawler → raw_content → Classifier → classified_content → Publisher
  - **Crawler updates**:
    - Raw content indexing to `{source}_raw_content` indexes
    - Minimal processing, preserves HTML/text/metadata for classifier
    - `classification_status=pending` marking for classifier pickup
  - **Classifier service** (new):
    - Content type detection (article, page, video, job, etc.)
    - Quality scoring (0-100) based on completeness and metadata
    - Topic classification with crime detection
    - Source reputation tracking
    - Publisher compatibility alias fields (`Body`, `Source`)
  - **Publisher enhancements**:
    - Dual-mode operation: classifier-based OR legacy keyword-based
    - Configuration: `use_classified_content`, `min_quality_score`, `index_suffix`
    - Classification-aware filtering (`is_crime_related=true`, quality threshold)
    - Trusts classifier determinations for improved accuracy
  - **Updated system architecture diagram** with content pipeline visualization
  - **Updated Elasticsearch index patterns** documentation
  - **Migration strategy**: Phased rollout with feature flags and rollback support

- **Database Scheduler Update** (2025-12-15): Added database-backed job scheduler documentation
  - Comprehensive job scheduler guide (`/crawler/docs/DATABASE_SCHEDULER.md`)
  - Database-backed scheduler implementation (`internal/job/db_scheduler.go`)
  - REST API for job management (create, update, delete, list)
  - Support for immediate and cron-scheduled jobs
  - Job status tracking and lifecycle management
  - Vue.js frontend integration for job management
  - Scheduler auto-starts with httpd command
  - Updated crawler service documentation with scheduler features

- **Initial Version** (2025-12-13): Created comprehensive AI assistant guide
  - Multi-service architecture overview
  - Docker environment documentation
  - Service-specific guidelines
  - Cross-service integration patterns
  - Git workflow and conventions

---

*This document is maintained for AI assistants working with the North Cloud codebase. Keep it updated as the architecture evolves.*
