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
- Publish filtered content to PubSub
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
┌───────────────────────────────────────────────────────────────────────────┐
│                         North Cloud Platform                               │
├───────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌──────────┐         ┌──────────────┐        ┌──────────────┐           │
│  │ Crawler  │────────▶│    Source    │        │  Classifier  │           │
│  │(crawler) │         │   Manager    │        │  (Go 1.25)   │           │
│  │          │         │  (Go + Vue)  │        │              │           │
│  └────┬─────┘         └──────────────┘        └──────▲───────┘           │
│       │                                                │                   │
│       │ raw_content                                    │                   │
│       │ (pending)                                      │ classified        │
│       ▼                                                │ _content          │
│  ┌─────────────────────────────────────────────────────┼──────────┐      │
│  │              Elasticsearch (Content Pipeline)       │          │      │
│  │  ┌────────────────┐  ┌──────────────┐  ┌──────────▼────────┐ │      │
│  │  │ {source}_raw   │─▶│  Classifier  │─▶│ {source}_classified│ │      │
│  │  │ _content       │  │   Service    │  │ _content          │ │      │
│  │  │ (pending)      │  │              │  │ (crime filtered)  │ │      │
│  │  └────────────────┘  └──────────────┘  └──────────┬────────┘ │      │
│  └─────────────────────────────────────────────────────┼──────────┘      │
│                                                         │                  │
│                                                         ▼                  │
│              ┌───────────────────────────────────────────────────┐       │
│              │          Publisher Service (Hub)                   │       │
│              │  ┌──────────────┐  ┌──────────────┐  ┌─────────┐ │       │
│              │  │ PostgreSQL   │  │  API Server  │  │  Vue.js │ │       │
│              │  │  Database    │◄─┤  (REST API)  │◄─┤Dashboard│ │       │
│              │  │              │  │              │  │         │ │       │
│              │  │ - Routes     │  │ - Sources    │  │ - CRUD  │ │       │
│              │  │ - Sources    │  │ - Channels   │  │ - Stats │ │       │
│              │  │ - Channels   │  │ - Routes     │  │         │ │       │
│              │  │ - History    │  │ - Stats      │  │         │ │       │
│              │  └──────▲───────┘  └──────────────┘  └─────────┘ │       │
│              │         │                                          │       │
│              │         │          ┌──────────────┐               │       │
│              │         └──────────┤Router Service│               │       │
│              │                    │(Background)  │               │       │
│              │                    └──────┬───────┘               │       │
│              └───────────────────────────┼───────────────────────┘       │
│                                          │                                │
│                                          │ Publishes to Redis            │
│                                          ▼                                │
│                              ┌────────────────────┐                      │
│                              │  Redis Pub/Sub     │                      │
│                              │  Channels:         │                      │
│                              │  - articles:crime  │                      │
│                              │  - articles:news   │                      │
│                              │  - articles:local  │                      │
│                              └─────────┬──────────┘                      │
│                                        │                                  │
│              ┌─────────────────────────┴──────────────────┐             │
│              │                                             │             │
│              ▼                                             ▼             │
│    ┌──────────────────┐                         ┌──────────────────┐   │
│    │ External Service │                         │ External Service │   │
│    │  (Drupal Site)   │                         │  (Laravel Site)  │   │
│    │                  │                         │                  │   │
│    │ - Subscribes to  │                         │ - Subscribes to  │   │
│    │   articles:crime │                         │   articles:news  │   │
│    │ - Own dedup      │                         │ - Own filters    │   │
│    │ - Own storage    │                         │ - Own storage    │   │
│    └──────────────────┘                         └──────────────────┘   │
│                                                                           │
│  Infrastructure:                                                         │
│  ┌─────────────┐  ┌─────────────┐  ┌──────────────────┐                │
│  │ PostgreSQL  │  │    Redis    │  │      Nginx       │                │
│  │  (4 DBs)    │  │  (Pub/Sub)  │  │  (Reverse Proxy) │                │
│  └─────────────┘  └─────────────┘  └──────────────────┘                │
└───────────────────────────────────────────────────────────────────────────┘
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

3. **Publishing** (Publisher → Redis → Consumers)
   - **Publisher Router** queries classified_content indexes based on active routes
   - Filters by `is_crime_related`, `quality_score >= threshold`, and `topics`
   - Publishes full article payload to topic-based Redis pub/sub channels
   - Records publish history in PostgreSQL (prevents re-publishing)
   - **External Consumers** (Drupal, Laravel, etc.) subscribe to Redis channels
   - Consumers handle own filtering, deduplication, and storage
   - Complete decoupling: publisher doesn't manage destinations

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
  - **Interval-based job scheduler** for dynamic crawling (NEW - Dec 2025)
  - REST API for job management with 8 new endpoints (pause/resume/cancel/retry)
  - Support for immediate and interval-based scheduling (every N minutes/hours/days)
  - Job status tracking with 7 states (pending → scheduled → running → completed/failed/paused/cancelled)
  - Execution history tracking in `job_executions` table
  - Distributed locking for multi-instance deployment
  - Exponential backoff retry with configurable max retries
  - Real-time scheduler metrics and job statistics
- **Scheduler Architecture**:
  - **IntervalScheduler**: Modern replacement for cron-based scheduler
  - Polls database every 10 seconds for jobs ready to run
  - Atomic lock acquisition using PostgreSQL CAS operations
  - Stale lock cleanup every 1 minute (5-minute timeout)
  - Thread-safe metrics collection every 30 seconds
  - Graceful shutdown with context cancellation
- **Indexing Strategy**:
  - One index per source: `{source_name}_raw_content` (e.g., `example_com_raw_content`)
  - Documents marked with `classification_status=pending` for classifier pickup
  - Extracts: title, raw_text, raw_html, OG tags, metadata, published_date
- **Documentation**:
  - `/crawler/README.md` - General crawler documentation
  - `/crawler/frontend/README.md` - Frontend documentation
  - `/crawler/docs/INTERVAL_SCHEDULER.md` - **NEW** Interval-based scheduler guide (recommended)
  - `/crawler/docs/DATABASE_SCHEDULER.md` - Legacy cron-based scheduler (deprecated)

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

#### 4. **publisher**
- **Location**: `/publisher`
- **Language**: Go 1.25+
- **Purpose**: Database-backed routing hub that filters articles and publishes to Redis pub/sub channels
- **Database**: `postgres-publisher` (publisher database)
- **Dependencies**: Elasticsearch (classified_content indexes), Redis (pub/sub), PostgreSQL
- **Ports**: 8070 (API)
- **Architecture**: Two-component system
  1. **API Server** (`/app/publisher api`):
     - REST API for managing sources, channels, and routes
     - JWT authentication integration
     - Publishing statistics and history
  2. **Router Service** (`/app/publisher router`):
     - Background worker that processes routes
     - Queries Elasticsearch classified_content indexes
     - Filters by quality_score and topics
     - Publishes to Redis pub/sub channels
     - Records publish history in database
  3. **Frontend Dashboard** (part of unified dashboard):
     - Vue.js 3 interface for managing publisher configuration
     - CRUD operations for sources, channels, routes
     - Real-time statistics and publish history
     - Accessible at `/dashboard/publisher` via nginx
- **Key Features**:
  - **Database-backed configuration**: PostgreSQL stores sources, channels, routes, publish_history
  - **Dynamic routing**: Many-to-many routes (sources → channels) with configurable filters
  - **Topic-based channels**: Redis channels like `articles:crime`, `articles:news`
  - **Complete decoupling**: No destination-specific logic; consumers subscribe independently
  - **Quality filtering**: Routes specify min_quality_score (0-100) and topics
  - **Publish history**: Persistent audit trail of all published articles
  - **Deduplication**: Database-backed (publish_history table prevents re-publishing)
  - **Web UI**: Full CRUD interface for sources, channels, routes
- **Database Schema**:
  - `sources`: Elasticsearch index patterns to monitor (e.g., `example_com_classified_content`)
  - `channels`: Redis pub/sub channels (e.g., `articles:crime`)
  - `routes`: Many-to-many mapping with filters (source_id, channel_id, min_quality_score, topics)
  - `publish_history`: Audit trail (article_id, channel_name, quality_score, published_at)
- **API Endpoints**:
  - `/api/v1/sources` - CRUD for sources
  - `/api/v1/channels` - CRUD for channels
  - `/api/v1/routes` - CRUD for routes (with joined source/channel names)
  - `/api/v1/stats/overview` - Publishing statistics
  - `/api/v1/publish-history` - Paginated publish history
  - `/health` - Health check
- **Redis Message Format**: Full Elasticsearch article payload with publisher metadata
  - All classified_content fields (title, body, canonical_url, quality_score, topics, etc.)
  - Publisher metadata (route_id, published_at, channel)
  - See `/publisher/docs/REDIS_MESSAGE_FORMAT.md` for details
- **Consumer Integration**: External services (Drupal, Laravel, etc.) subscribe to Redis channels
  - Consumers handle own filtering, deduplication, storage
  - No publisher dependency required
  - See `/publisher/docs/CONSUMER_GUIDE.md` for implementation examples
- **Configuration**:
  - `POSTGRES_PUBLISHER_*`: Database connection
  - `PUBLISHER_PORT`: API server port (default: 8070)
  - `PUBLISHER_ROUTER_CHECK_INTERVAL`: Polling interval (default: 5m)
  - `PUBLISHER_ROUTER_BATCH_SIZE`: Articles per route per check (default: 100)
- **Documentation**:
  - `/publisher/README.md` - User guide
  - `/publisher/CLAUDE.md` - Technical architecture
  - `/publisher/docs/REDIS_MESSAGE_FORMAT.md` - Message specification
  - `/publisher/docs/CONSUMER_GUIDE.md` - Integration guide
  - `/publisher/docs/TESTING.md` - Testing procedures
  - `/publisher/docs/DEPLOYMENT.md` - Deployment guide

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

#### 6. **search-service**
- **Location**: `/search`
- **Language**: Go 1.25
- **Purpose**: Full-text search across all classified content
- **Dependencies**: Elasticsearch
- **Ports**: 8090 (internal), 8092 (development), accessible via nginx at `/api/search`
- **Key Features**:
  - Google-like full-text search with relevance ranking
  - Advanced filtering (topics, content type, quality, dates)
  - Faceted search with aggregations
  - Search highlighting and snippets
  - Pagination and multi-field sorting
  - Query across all `*_classified_content` indexes
- **API Endpoints**:
  - `POST /api/v1/search` - Execute search with filters
  - `GET /api/v1/search` - Simple search via query params
  - `GET /api/v1/health` - Health check
- **Search Features**:
  - Multi-match across title, body (raw_text), OG tags, metadata
  - Field boosting: title (3x), OG title (2x), body (1x)
  - Fuzzy matching with typo tolerance
  - Recency and quality score boosting
  - Configurable pagination (max 100 per page)
- **Configuration**:
  - `max_page_size: 100` - Maximum results per page
  - `default_page_size: 20` - Default page size
  - `search_timeout: 5s` - Elasticsearch query timeout
- **Documentation**: See [/search/README.md](search/README.md)

#### 7. **auth**
- **Location**: `/auth`
- **Language**: Go 1.25+
- **Purpose**: Authentication service for dashboard and API access
- **Ports**: 8040
- **Key Features**:
  - Username/password authentication
  - JWT token generation and validation
  - Token-based API protection
  - Simple credential management via environment variables
  - REST API for login
- **API Endpoints**:
  - `POST /api/v1/auth/login` - Authenticate and receive JWT token
  - `GET /health` - Health check (public)
- **Authentication Flow**:
  1. User submits username/password to `/api/v1/auth/login`
  2. Service validates credentials against environment variables
  3. On success, returns JWT token with 24h expiration
  4. Frontend stores token in localStorage
  5. All API requests include token in `Authorization: Bearer <token>` header
  6. Backend services validate token using shared JWT secret
- **Configuration**:
  - `AUTH_USERNAME` - Dashboard username (required)
  - `AUTH_PASSWORD` - Dashboard password (required)
  - `AUTH_JWT_SECRET` - Secret key for JWT signing/validation (required in production)
  - `AUTH_PORT` - Service port (default: 8040)
- **Security**:
  - Tokens expire after 24 hours
  - HS256 algorithm for token signing
  - Shared secret across all services for token validation
  - Health endpoints remain public (no authentication required)

### Infrastructure Services

#### PostgreSQL Databases
- **postgres-source-manager**: Source manager database (gosources)
- **postgres-crawler**: Crawler database (crawler)
- **postgres-publisher**: Publisher database (publisher) - stores sources, channels, routes, publish_history
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
- **Content Flow**: raw_content (pending) → Classifier → classified_content → Publisher → Redis pub/sub → External consumers

#### Redis
- **Purpose**: Pub/sub messaging, deduplication (historical), caching
- **Port**: 6379
- **Usage**:
  - **Pub/Sub Channels**: Publisher publishes articles to topic-based channels (e.g., `articles:crime`, `articles:news`)
  - **Channel Pattern**: `articles:{topic}` - external services subscribe to relevant channels
  - **Message Format**: Full Elasticsearch article payload with publisher metadata (see `/publisher/docs/REDIS_MESSAGE_FORMAT.md`)
  - **Consumers**: External services (Drupal, Laravel, etc.) subscribe and handle own storage/deduplication

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
│   │   ├── httpd/               # HTTP API server with interval scheduler
│   │   ├── crawl/               # Manual crawl command
│   │   └── scheduler/           # Legacy scheduler (deprecated)
│   ├── internal/
│   │   ├── scheduler/           # Interval-based scheduler (NEW)
│   │   │   ├── interval_scheduler.go  # Main scheduler implementation
│   │   │   ├── state_machine.go       # Job state validation
│   │   │   ├── metrics.go             # Thread-safe metrics
│   │   │   └── options.go             # Functional options pattern
│   │   ├── database/            # Database layer (PostgreSQL)
│   │   │   ├── job_repository.go        # Job CRUD + locking
│   │   │   ├── execution_repository.go  # Execution history CRUD
│   │   │   └── interfaces.go            # Repository interfaces
│   │   ├── domain/              # Domain models
│   │   │   ├── job.go           # Job model (13 new fields)
│   │   │   └── execution.go     # JobExecution model (NEW)
│   │   └── api/                 # REST API handlers
│   │       └── jobs_handler.go  # Job control endpoints (8 new)
│   ├── migrations/
│   │   ├── 003_refactor_to_interval_scheduler.up.sql    # Migration (NEW)
│   │   └── 003_refactor_to_interval_scheduler.down.sql  # Rollback (NEW)
│   ├── frontend/                # Vue.js dashboard
│   │   └── src/
│   │       └── views/
│   │           └── CrawlJobsView.vue  # Job management UI
│   ├── scripts/
│   │   └── test-migration.sh    # Migration test script (NEW)
│   ├── docs/
│   │   ├── INTERVAL_SCHEDULER.md  # Interval scheduler guide (NEW)
│   │   └── DATABASE_SCHEDULER.md  # Legacy cron scheduler (deprecated)
│   ├── tests/
│   └── README.md
│   └── SCHEDULER_REFACTOR_SUMMARY.md  # Implementation summary (NEW)
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
├── auth/                         # Authentication service
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   ├── internal/
│   │   ├── api/                 # HTTP API handlers
│   │   │   ├── auth_handler.go  # Login endpoint handler
│   │   │   └── server.go        # HTTP server setup
│   │   ├── auth/                # JWT token management
│   │   │   └── jwt.go           # Token generation and validation
│   │   └── config/              # Configuration management
│   │       └── config.go        # Environment variable loading
│   └── Dockerfile.dev           # Development Dockerfile
│
├── dashboard/                    # Unified dashboard frontend
│   ├── Dockerfile
│   ├── Dockerfile.dev
│   ├── src/
│   │   ├── views/
│   │   │   └── LoginView.vue    # Login page component
│   │   ├── composables/
│   │   │   ├── useAuth.js       # Authentication state management
│   │   │   └── useFormValidation.ts  # Form validation with TypeScript types
│   │   ├── api/
│   │   │   ├── auth.js          # Auth API client
│   │   │   └── client.ts       # API clients with JWT interceptors (TypeScript)
│   │   ├── types/
│   │   │   ├── publisher.ts     # Publisher API types (Source, Channel, Route, PreviewArticle, etc.)
│   │   │   ├── indexManager.ts # Index manager types
│   │   │   └── common.ts       # Shared types (ApiError, etc.)
│   │   ├── router/
│   │   │   └── index.js         # Router with auth guards
│   │   └── App.vue              # Main app component with auth-aware layout
│   └── package.json
│
└── infrastructure/               # Shared infrastructure configs
    ├── jwt/                     # Shared JWT authentication middleware
    │   └── middleware.go        # JWT validation middleware for Gin
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
- **Type Safety**: 
  - Use `any` instead of `interface{}` (Go 1.18+)
  - Avoid magic numbers: define constants for numeric literals
  - Use integer range syntax (`for i := range n`) when possible (Go 1.22+)
  - Avoid copying large structs in loops: use pointers or indexing (`for i := range items { item := &items[i] }`)
  - Use compound assignment operators (`/=`, `*=`, etc.) instead of `x = x / y`

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

#### Frontend (dashboard, source-manager/frontend)
- **Framework**: Vue.js 3 with Composition API
- **Build Tool**: Vite
- **Styling**: Component-scoped CSS or Tailwind
- **State Management**: Pinia (if needed)
- **TypeScript**: Use strict typing, avoid `any` types
  - Prefer `unknown` for generic values (form fields, error handling)
  - Use specific interfaces for known types (`Source`, `Channel`, `Route`, etc.)
  - Define shared types in `/dashboard/src/types/` directory
  - Common types: `PreviewArticle`, `TestCrawlArticle`, `ApiError` (see `/dashboard/src/types/`)
  - Error handling: Use `ApiError` interface with type assertions (`err as ApiError`)

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
  - `AUTH_USERNAME`: Dashboard username (required)
  - `AUTH_PASSWORD`: Dashboard password (required)
  - `AUTH_JWT_SECRET`: Shared JWT secret for token signing/validation (required in production)

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
- **Authentication**: JWT token-based authentication
  - Token obtained via `POST /api/auth/api/v1/auth/login`
  - Include in `Authorization: Bearer <token>` header
  - All `/api/v1/*` routes protected (except `/health` endpoints)
  - Shared JWT secret (`AUTH_JWT_SECRET`) across all services
  - Tokens expire after 24 hours
- **Error Responses**: Consistent JSON format
- **Middleware**: Use `infrastructure/jwt` package for JWT validation

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
| auth | 8040 | 8040 | Authentication service |
| streetcode | 80 | 8090 | Drupal web interface |
| nginx | 80 | 80 | Reverse proxy |
| dashboard | 3002 | 3002 | Unified dashboard frontend |
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

### Profiling and Performance Monitoring

North Cloud includes comprehensive profiling and performance monitoring infrastructure for detecting memory leaks, analyzing CPU usage, and benchmarking performance across all services.

#### Available Tools

1. **pprof Profiling**: Go's built-in profiler for CPU, heap, goroutine, allocation, block, and mutex profiling
2. **Benchmarks**: 44 comprehensive benchmarks across all 6 services for performance regression detection
3. **Memory Health Endpoints**: HTTP endpoints exposing runtime memory statistics
4. **Memory Leak Detection**: Automated tools for detecting heap and goroutine leaks
5. **Helper Scripts**: Automation scripts for common profiling workflows

#### Quick Start

**Enable Profiling in Development**:
Profiling is automatically enabled when services run. pprof endpoints are exposed on dedicated ports:
- Crawler: 6060
- Source Manager: 6061
- Classifier: 6062
- Publisher API: 6063
- Publisher Router: 6064
- Auth: 6065
- Search: 6066

**Capture a Heap Profile**:
```bash
./scripts/profile.sh crawler heap
```

**Run All Benchmarks**:
```bash
./scripts/run-benchmarks.sh
```

**Check for Memory Leaks**:
```bash
./scripts/check-memory-leaks.sh -s crawler -i 600 -c 5
```

**Monitor Memory Health**:
```bash
curl http://localhost:6060/health/memory
```

#### Helper Scripts

Located in `/scripts/` directory:

1. **profile.sh** - Automated profile capture
   ```bash
   # Capture heap profile
   ./scripts/profile.sh crawler heap

   # Capture 60s CPU profile
   ./scripts/profile.sh publisher-api cpu 60

   # Capture goroutine profile
   ./scripts/profile.sh classifier goroutine
   ```

2. **compare-heap.sh** - Memory leak detection via heap comparison
   ```bash
   # Compare heap profiles 10 minutes apart
   ./scripts/compare-heap.sh crawler 600

   # Shows growth analysis and leak warnings
   ```

3. **run-benchmarks.sh** - Run benchmarks across all services
   ```bash
   # Run all benchmarks
   ./scripts/run-benchmarks.sh

   # Run specific service with memory stats
   ./scripts/run-benchmarks.sh -s crawler -m

   # Save as baseline
   ./scripts/run-benchmarks.sh -b

   # Compare against baseline
   ./scripts/run-benchmarks.sh -c baselines/baseline_20260104.txt
   ```

4. **check-memory-leaks.sh** - Automated leak detection
   ```bash
   # Check all services (3 checks at 5min intervals)
   ./scripts/check-memory-leaks.sh

   # Extended check for specific service
   ./scripts/check-memory-leaks.sh -s publisher-api -i 900 -c 10

   # With log-based alerts
   ./scripts/check-memory-leaks.sh -a
   ```

#### Profiling Workflows

**Investigating Memory Leaks**:
1. Monitor service memory health over time:
   ```bash
   watch -n 30 'curl -s http://localhost:6060/health/memory | jq'
   ```

2. Run automated leak detection:
   ```bash
   ./scripts/check-memory-leaks.sh -s crawler -i 600 -c 5 -a
   ```

3. If leak detected, compare heap profiles:
   ```bash
   ./scripts/compare-heap.sh crawler 600
   ```

4. Analyze with pprof:
   ```bash
   go tool pprof -http=:8080 profiles/crawler_heap_*.pb.gz
   ```

**Performance Regression Detection**:
1. Establish baseline:
   ```bash
   ./scripts/run-benchmarks.sh -b
   ```

2. After code changes, run benchmarks:
   ```bash
   ./scripts/run-benchmarks.sh -c baselines/baseline_TIMESTAMP.txt
   ```

3. Use `benchstat` for statistical analysis:
   ```bash
   go install golang.org/x/perf/cmd/benchstat@latest
   benchstat baseline.txt current.txt
   ```

**CPU Profiling for Optimization**:
1. Capture CPU profile during load:
   ```bash
   ./scripts/profile.sh crawler cpu 60
   ```

2. Analyze with pprof:
   ```bash
   go tool pprof -http=:8080 profiles/crawler_cpu_*.pb.gz
   ```

3. Focus on flame graphs and top functions

#### Memory Health Endpoints

All services expose `GET /health/memory` with runtime statistics:

```bash
curl http://localhost:6060/health/memory | jq
```

Response:
```json
{
  "timestamp": "2026-01-04T10:30:00Z",
  "heap_alloc_mb": 12.5,
  "heap_inuse_mb": 15.2,
  "heap_idle_mb": 8.3,
  "stack_inuse_mb": 1.2,
  "num_gc": 42,
  "num_goroutine": 127,
  "gomaxprocs": 8,
  "last_gc_pause_ms": 0.5
}
```

**Service Port Mapping**:
- Crawler: `http://localhost:6060/health/memory`
- Source Manager: `http://localhost:6061/health/memory`
- Classifier: `http://localhost:6062/health/memory`
- Publisher API: `http://localhost:6063/health/memory`
- Publisher Router: `http://localhost:6064/health/memory`
- Auth: `http://localhost:6065/health/memory`
- Search: `http://localhost:6066/health/memory`

#### Benchmark Coverage

**44 Total Benchmarks** across all services:

- **Crawler** (18 benchmarks): Job processing, scheduler, indexing, extraction
- **Source Manager** (7 benchmarks): Source CRUD, validation, URL parsing
- **Classifier** (6 benchmarks): Classification, quality scoring, topic detection
- **Publisher** (6 benchmarks): Filtering, formatting, JSON:API posting
- **Auth** (10 benchmarks): JWT operations, hashing, token validation
- **Search** (7 benchmarks): Full-text search, faceted search, pagination

Run benchmarks with:
```bash
# All services
./scripts/run-benchmarks.sh -m -v

# Specific service
./scripts/run-benchmarks.sh -s crawler -m -t 5s
```

#### Best Practices

1. **Development Profiling**:
   - Profile before and after significant changes
   - Establish baselines for performance-critical code paths
   - Use benchmarks to detect regressions early

2. **Memory Leak Detection**:
   - Run leak detection weekly in long-running environments
   - Monitor memory health metrics in production
   - Investigate any heap growth >50% over 10 minutes

3. **Performance Optimization**:
   - Profile first, optimize second (don't guess)
   - Focus on the top 3-5 functions in CPU profiles
   - Use benchmarks to verify improvements

4. **Zero-Overhead Design**:
   - Profiling infrastructure has no overhead when disabled
   - pprof endpoints only activate when accessed
   - Memory health checks use <1ms of CPU time

#### Troubleshooting

**pprof endpoints not accessible**:
- Verify service is running: `docker ps | grep service-name`
- Check port mapping: Service may not expose profiling port
- In production, ensure `PPROF_ENABLED=true` in environment

**Benchmarks fail**:
- Check service dependencies (databases, Elasticsearch)
- Verify test data exists
- Run with `-v` flag for detailed output

**Memory stats show 0 values**:
- Service may not have started monitoring yet
- Wait 30 seconds after service startup
- Check logs for memory monitor initialization

#### Documentation

For comprehensive profiling documentation, see:
- `/docs/PROFILING.md` - Complete profiling guide
- Service-specific benchmark documentation in `*_bench_test.go` files
- Infrastructure monitoring package: `/infrastructure/monitoring/`

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
- **Interval-Based Job Scheduler** (NEW - December 2025):
  - **IMPORTANT**: Read `/crawler/docs/INTERVAL_SCHEDULER.md` for comprehensive guide
  - Use **interval-based scheduling**: `{"interval_minutes": 30, "interval_type": "minutes"}` instead of cron
  - **7 job states**: pending, scheduled, running, paused, completed, failed, cancelled
  - **State validation**: Use state machine to validate transitions before updates
  - **Job control**: Support pause, resume, cancel, manual retry operations
  - **Execution history**: Track every job run in `job_executions` table
  - **Distributed locking**: Use PostgreSQL CAS locks for multi-instance safety
  - **Exponential backoff**: `base × 2^(attempt-1)` capped at 1 hour
  - **Metrics**: Real-time job counts, success rates, average duration
  - Jobs require `source_name` field to match existing source
  - Scheduler polls database every 10 seconds (automatic, no manual reload needed)
  - **8 new API endpoints**: `/pause`, `/resume`, `/cancel`, `/retry`, `/executions`, `/stats`, `/scheduler/metrics`
  - **Migration**: Run migration 003 to upgrade from cron-based scheduler
  - See `/crawler/docs/INTERVAL_SCHEDULER.md` for complete documentation
  - **Legacy**: `/crawler/docs/DATABASE_SCHEDULER.md` (cron-based scheduler, deprecated)

**For Source Manager**:
- Backend: Follow Go REST API conventions
- Frontend: Use Vue 3 Composition API
- Test API endpoints
- Validate database migrations
- **Test Crawl Endpoint**: `POST /api/v1/sources/test-crawl` for previewing extraction without saving
  - Returns simulated response with articles found, success rate, warnings, and sample articles
  - Use constants for magic numbers in test responses

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
- **Test/Preview Endpoints**:
  - `GET /api/v1/routes/preview` - Preview articles matching route filters
  - `GET /api/v1/channels/:id/test-publish` - Simulate publishing to a channel
  - Use constants for magic numbers in simulation responses
  - Avoid copying large structs in loops (use pointers or indexing)
  - Trust classifier's determinations (don't re-check keywords)
  - Use configured `index_suffix` for index naming
- **Legacy mode** (`use_classified_content: false`):
  - Query `{source}_articles` indexes
  - Use keyword-based crime detection
- Follow structured logging conventions (snake_case fields)
- Test Drupal JSON:API integration
- Verify Redis deduplication
- Respect rate limits

**For Auth Service**:
- Simple username/password authentication via environment variables
- JWT token generation with 24h expiration
- No database required (credentials in environment)
- Shared JWT secret must match across all services
- Health endpoint remains public
- For production, generate strong JWT secret: `openssl rand -hex 32`

**For Dashboard Frontend**:
- Vue.js 3 with Composition API and TypeScript
- Authentication state managed via `useAuth` composable
- Route guards redirect unauthenticated users to `/login`
- JWT tokens stored in localStorage
- API clients automatically inject tokens in Authorization header
- Handles 401 responses by redirecting to login and clearing token
- Login page styled with Tailwind CSS
- **Type Safety**:
  - All components use proper TypeScript types (no `any` types)
  - Shared types defined in `/dashboard/src/types/` directory
  - Use `unknown` for generic values (form fields, error handling)
  - Use `ApiError` interface for error handling with type assertions
  - Type definitions: `PreviewArticle`, `TestCrawlArticle`, `Source`, `Channel`, `Route`, etc.

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
- **JWT Authentication**: All dashboard API routes require JWT tokens
  - Token obtained from `/api/auth/api/v1/auth/login` endpoint
  - Shared `AUTH_JWT_SECRET` environment variable across all services
  - Tokens expire after 24 hours
  - Health endpoints remain public (no authentication)
- Validate authentication in all services using `infrastructure/jwt` middleware
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

**Job Scheduling & Management** (NEW - December 2025):
- **`/crawler/docs/INTERVAL_SCHEDULER.md`** - **Interval-based scheduler guide (RECOMMENDED)**
  - Modern interval-based scheduling (every N minutes/hours/days)
  - Complete API reference for 8 new endpoints
  - Job lifecycle with 7 states and state machine validation
  - Execution history tracking and analytics
  - Distributed locking for multi-instance deployment
  - Exponential backoff retry with configurable max retries
  - Real-time metrics and job statistics
  - Migration guide from cron-based scheduler
  - Troubleshooting, best practices, and security
  - Performance optimization and monitoring
- `/crawler/SCHEDULER_REFACTOR_SUMMARY.md` - Implementation summary (December 2025)
  - Complete refactor overview (~3,500 lines of code)
  - File-by-file changes and statistics
  - Key technical decisions and trade-offs
  - Testing checklist and deployment plan
  - Next steps and future enhancements
- `/crawler/docs/DATABASE_SCHEDULER.md` - **Legacy cron-based scheduler (DEPRECATED)**
  - Original cron expression-based scheduler
  - Maintained for reference and rollback only
  - Use INTERVAL_SCHEDULER.md for new implementations

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

**Authentication & Security**:
- `/auth/` - Authentication service implementation
- `/infrastructure/jwt/` - Shared JWT middleware for backend services
- `/dashboard/src/composables/useAuth.js` - Frontend authentication state management
- `/dashboard/src/views/LoginView.vue` - Login page component

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

- **Type Safety and Code Quality Improvements** (2025-12-29): Enhanced type safety and linting compliance
  - **TypeScript Type Safety**:
    - Replaced all 15 `any` types with proper TypeScript types across dashboard
    - Created shared type definitions: `PreviewArticle`, `TestCrawlArticle`, `ApiError`
    - Used `unknown` for generic form validation and error handling (safer than `any`)
    - All components now use specific interfaces (`Source`, `Channel`, `Route`, etc.)
    - Type definitions located in `/dashboard/src/types/` directory
  - **Go Linting Improvements**:
    - Replaced magic numbers with named constants in all services
    - Fixed range value copy issues (use pointers/indexing instead of copying)
    - Replaced `interface{}` with `any` (Go 1.18+)
    - Used compound assignment operators (`/=`, `*=`, etc.)
    - Applied Go 1.22+ integer range syntax where applicable
    - Removed unused nolint directives
  - **Services Updated**:
    - Crawler: Function length refactoring, magic number constants
    - Publisher: Test/preview endpoint constants, range loop optimizations
    - Source Manager: Test crawl endpoint constants, `interface{}` → `any`
  - **Documentation**: Updated CLAUDE.md with TypeScript and Go type safety best practices

- **Crawler Scheduler Refactor: Interval-Based Job Scheduling** (2025-12-29): Complete modernization of job scheduler
  - **Architecture Change**: From cron-based to interval-based scheduling for improved user experience
  - **Database Schema**: Migration 003 adds `job_executions` table and 13 new columns to `jobs` table
    - New fields: `interval_minutes`, `interval_type`, `next_run_at`, `is_paused`, `max_retries`, `retry_backoff_seconds`, `current_retry_count`, `lock_token`, `lock_acquired_at`, `paused_at`, `cancelled_at`, `metadata`
    - `job_executions` table: Complete execution history with 17 columns (duration, items crawled/indexed, resource usage, error details)
  - **IntervalScheduler**: Modern replacement for DBScheduler (617 lines)
    - Polls database every 10 seconds for jobs ready to run
    - Distributed locking using PostgreSQL atomic CAS operations
    - Stale lock cleanup every 1 minute (5-minute timeout)
    - Thread-safe metrics collection every 30 seconds
    - Graceful shutdown with context cancellation
  - **7 Job States**: pending, scheduled, running, paused, completed, failed, cancelled
    - State machine validation prevents invalid transitions
    - Helper functions: `CanPause`, `CanResume`, `CanCancel`, `CanRetry`
  - **8 New API Endpoints**:
    - Job control: `POST /jobs/:id/pause|resume|cancel|retry`
    - History: `GET /jobs/:id/executions`, `GET /executions/:id`
    - Statistics: `GET /jobs/:id/stats`, `GET /scheduler/metrics`
  - **Key Features**:
    - Interval-based scheduling: `{"interval_minutes": 30, "interval_type": "minutes"}`
    - Execution history tracking with 100 executions OR 30 days retention
    - Exponential backoff retry: `base × 2^(attempt-1)` capped at 1 hour
    - Real-time metrics: job counts, success rates, average duration
    - Multi-instance safe with atomic distributed locking
  - **Code Changes**: ~3,500 lines of code
    - 15 files created (migrations, scheduler core, tests, documentation)
    - 10 files modified (domain models, repositories, API handlers, main integration)
    - 75+ unit tests (state machine, metrics, concurrency)
    - Migration test script for schema validation
  - **Documentation**:
    - `/crawler/docs/INTERVAL_SCHEDULER.md` - Complete 600+ line user guide
    - `/crawler/SCHEDULER_REFACTOR_SUMMARY.md` - Implementation summary
    - Migration guide from cron-based scheduler with rollback procedures
  - **Backward Compatibility**: Legacy `schedule_time` (cron) field maintained for rollback safety
  - **Testing**: All shadowing errors fixed, 75+ tests pass, builds successfully

- **Publisher Modernization: Database-Backed Redis Pub/Sub Architecture** (2025-12-28): Complete transformation of publisher service
  - **Architecture Change**: From YAML-configured direct-posting to database-backed Redis pub/sub routing hub
  - **Database Integration**: New PostgreSQL database (`postgres-publisher`) with four tables:
    - `sources`: Elasticsearch index patterns to monitor
    - `channels`: Topic-based Redis pub/sub channels (e.g., `articles:crime`, `articles:news`)
    - `routes`: Many-to-many source→channel mappings with quality/topic filters
    - `publish_history`: Persistent audit trail of all published articles
  - **Three-Component Architecture**:
    - **API Server** (`/app/publisher api`): REST API for managing sources, channels, routes; publishing statistics
    - **Router Service** (`/app/publisher router`): Background worker that queries Elasticsearch, filters articles, publishes to Redis
    - **Frontend Dashboard**: Vue.js 3 interface for CRUD operations, real-time stats, publish history
  - **Complete Decoupling**: Publisher no longer manages destinations/consumers
    - Publishes full article payloads to topic-based Redis channels
    - External services (Drupal, Laravel, etc.) subscribe independently
    - Consumers handle own filtering, deduplication, and storage
  - **Key Features**:
    - Dynamic routing configuration (no service restart needed)
    - Web UI for non-technical users to manage publisher
    - Quality score filtering (0-100) and topic-based routing
    - Database-backed deduplication via publish_history table
    - Many-to-many routes (multiple sources → multiple channels)
  - **Docker Integration**:
    - Two separate containers: `publisher-api` and `publisher-router`
    - Frontend is part of unified `dashboard` service
    - Single binary with multi-command CLI (`api` and `router` commands)
    - Production and development Docker configurations
    - Nginx routing for `/dashboard/publisher` frontend and `/api/publisher` API
  - **Documentation**:
    - `/publisher/docs/REDIS_MESSAGE_FORMAT.md` - Complete message specification
    - `/publisher/docs/CONSUMER_GUIDE.md` - Integration examples (Python, Node.js, PHP/Drupal)
    - `/publisher/docs/TESTING.md` - Comprehensive testing procedures with integration test script
    - `/publisher/docs/DEPLOYMENT.md` - Step-by-step deployment guide with rollback procedures
    - Updated `/publisher/README.md` and `/publisher/CLAUDE.md`
  - **System Diagram Updates**: Revised architecture diagram showing Redis pub/sub flow and external consumers
  - **Migration Strategy**: Big-bang cutover with rollback plan; YAML config deprecated in favor of database

- **Dashboard Authentication Implementation** (2025-12-27): JWT-based authentication for dashboard and APIs
  - **Auth service**: New Go service (`/auth`) for username/password authentication and JWT token generation
  - **Frontend authentication**: Login page (`LoginView.vue`), route guards, token storage (localStorage), and API interceptors
  - **Backend API protection**: JWT middleware (`infrastructure/jwt`) added to all backend services
  - **Protected routes**: All `/api/v1/*` routes require valid JWT tokens (health endpoints remain public)
  - **Nginx integration**: Added `/api/auth` location block for auth service routing
  - **Environment configuration**: `AUTH_USERNAME`, `AUTH_PASSWORD`, `AUTH_JWT_SECRET` variables
  - **Token security**: 24-hour expiration, HS256 signing algorithm, shared secret validation
  - **Works in dev and prod**: Environment-aware configuration with development-friendly defaults
  - **Documentation**: Updated CLAUDE.md with auth service details and authentication flow

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
