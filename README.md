# North Cloud

A microservices-based content management and publishing platform built with Go. The platform crawls news content, manages sources, classifies articles, and publishes them to external services via Redis pub/sub.

## Architecture

The project consists of multiple independent services that work together in a content processing pipeline:

### Core Services

- **crawler**: Web crawler service for scraping news articles with interval-based job scheduling
- **source-manager**: Go API with Vue.js frontend for managing content sources and crawling configurations
- **classifier**: Content classification service that processes raw content and enriches it with quality scores, topics, and metadata
- **publisher**: Database-backed routing hub that filters classified articles and publishes to Redis pub/sub channels
- **index-manager**: Centralized Elasticsearch index management service
- **search-service**: Full-text search microservice across all classified content
- **search-frontend**: Vue.js frontend for the search service
- **auth**: JWT-based authentication service for dashboard and API access
- **dashboard**: Unified Vue.js dashboard for managing all services

### Content Processing Pipeline

```
Crawler → Elasticsearch (raw_content) → Classifier → Elasticsearch (classified_content) → Publisher → Redis Pub/Sub → External Consumers
```

1. **Crawler** extracts articles and stores minimally-processed content to `{source}_raw_content` indexes
2. **Classifier** processes raw content, applies classification algorithms, and stores enriched content to `{source}_classified_content` indexes
3. **Publisher** queries classified content, filters by quality/topics, and publishes to topic-based Redis channels (e.g., `articles:crime`, `articles:news`)
4. **External consumers** (Drupal, Laravel, Node.js, Python, etc.) subscribe to Redis channels and handle their own storage/deduplication

### Infrastructure Services

- **PostgreSQL** (5 instances): source-manager, crawler, classifier, index-manager, publisher
- **Elasticsearch**: Content storage and search (raw_content and classified_content indexes)
- **Redis**: Pub/sub messaging for article distribution
- **MinIO**: Object storage for HTML archiving
- **Nginx**: Reverse proxy with SSL/TLS support
- **Kibana**: Elasticsearch visualization (development only)

## Project Structure

```
north-cloud/
├── docker-compose.base.yml     # Base configuration (infrastructure services)
├── docker-compose.dev.yml      # Development overrides
├── docker-compose.prod.yml     # Production overrides
├── .env                        # Environment variables (not committed)
├── .env.example               # Environment variables template
├── README.md
├── CLAUDE.md                  # Comprehensive AI assistant guide
├── DOCKER.md                  # Docker documentation
│
├── crawler/                    # Web crawler service
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   ├── cmd/
│   ├── internal/
│   ├── frontend/               # Vue.js dashboard
│   └── migrations/
│
├── source-manager/             # Source management service
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   ├── internal/
│   ├── frontend/               # Vue.js app
│   └── migrations/
│
├── classifier/                 # Content classification service
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   ├── cmd/
│   ├── internal/
│   └── migrations/
│
├── publisher/                  # Article publishing service
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   └── internal/
│
├── index-manager/              # Elasticsearch index management
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   └── migrations/
│
├── search/                     # Full-text search service
│   ├── Dockerfile
│   ├── go.mod
│   └── main.go
│
├── search-frontend/            # Search UI
│   ├── Dockerfile
│   └── src/
│
├── auth/                       # Authentication service
│   ├── Dockerfile
│   ├── go.mod
│   └── main.go
│
├── dashboard/                  # Unified dashboard frontend
│   ├── Dockerfile
│   └── src/
│
└── infrastructure/             # Shared configs
    ├── nginx/
    ├── elasticsearch/
    ├── postgres/
    ├── jwt/                    # JWT middleware
    └── certbot/                 # SSL/TLS certificates
```

## Prerequisites

- Docker and Docker Compose (v2+)
- Go 1.24+ (for local development of crawler, source-manager)
- Go 1.25+ (for local development of classifier, publisher, search, auth)
- Node.js and npm (for frontend development)

## Getting Started

### 1. Clone the repository

```bash
git clone <repository-url>
cd north-cloud
```

### 2. Configure environment variables

Copy `.env.example` to `.env` and update with your configuration:

```bash
cp .env.example .env
# Edit .env with your values
```

**Important variables:**
- `AUTH_USERNAME` and `AUTH_PASSWORD`: Dashboard credentials
- `AUTH_JWT_SECRET`: Shared JWT secret for token signing (generate with `openssl rand -hex 32`)
- Database passwords for all PostgreSQL instances
- `REDIS_PASSWORD`: Redis password (optional but recommended for production)

### 3. Start all services

#### Development Mode (Recommended for local development)

```bash
# Start in development mode with hot reloading
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d
```

**Development mode features:**
- Source code mounted as volumes for hot reloading
- Debug mode enabled (`APP_DEBUG=true`)
- All ports exposed for local access
- Development-friendly container names
- Kibana for Elasticsearch visualization

#### Production Mode

```bash
# Start in production mode
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d
```

**Production mode features:**
- Code baked into images (no volume mounts)
- Debug mode disabled (`APP_DEBUG=false`)
- Resource limits configured
- Security hardening enabled
- SSL/TLS ready

#### Quick Start

For convenience, you can create an alias:

```bash
# Add to your shell profile (.bashrc, .zshrc, etc.)
alias dc-dev='docker compose -f docker-compose.base.yml -f docker-compose.dev.yml'
alias dc-prod='docker compose -f docker-compose.base.yml -f docker-compose.prod.yml'

# Then use:
dc-dev up -d
dc-dev logs -f
dc-dev down
```

This will start all services:
- **PostgreSQL databases** (5 instances: source-manager, crawler, classifier, index-manager, publisher)
- **Elasticsearch** (for content storage and search)
- **Redis** (for pub/sub messaging)
- **MinIO** (for HTML archiving)
- **crawler** service (port 8060)
- **source-manager** service (port 8050)
- **classifier** service (port 8071)
- **publisher-api** service (port 8070)
- **publisher-router** service (background worker)
- **index-manager** service (port 8090)
- **search-service** (port 8092)
- **search-frontend** (port 3003)
- **auth** service (port 8040)
- **dashboard** (port 3002)
- **nginx** reverse proxy (port 80)
- **kibana** (port 5601, development only)

### 4. Access services

#### Via Nginx (Recommended)

- **Dashboard**: http://localhost/dashboard
- **Search Frontend**: http://localhost/search
- **Kibana**: http://localhost/kibana (development only)
- **API Endpoints**:
  - Crawler API: http://localhost/api/crawler
  - Source Manager API: http://localhost/api/sources
  - Publisher API: http://localhost/api/publisher
  - Classifier API: http://localhost/api/classifier
  - Search API: http://localhost/api/search
  - Auth API: http://localhost/api/auth

#### Direct Access (Development)

- **Dashboard**: http://localhost:3002
- **Crawler API**: http://localhost:8060
- **Source Manager API**: http://localhost:8050
- **Classifier API**: http://localhost:8071
- **Publisher API**: http://localhost:8070
- **Index Manager API**: http://localhost:8090
- **Search Service**: http://localhost:8092
- **Search Frontend**: http://localhost:3003
- **Auth Service**: http://localhost:8040
- **Elasticsearch**: http://localhost:9200
- **Kibana**: http://localhost:5601
- **Redis**: localhost:6379
- **MinIO Console**: http://localhost:9001 (default: minioadmin/changeme123)

## Development

### Development vs Production

The project uses separate Docker Compose configurations for different environments:

- **`docker-compose.base.yml`**: Shared infrastructure services (databases, Redis, Elasticsearch, MinIO)
- **`docker-compose.dev.yml`**: Development overrides (hot reloading, debug mode, volume mounts)
- **`docker-compose.prod.yml`**: Production overrides (optimized builds, security, resource limits)

### Development Mode

Development mode includes:
- **Hot Reloading**: Source code mounted as volumes for live code changes
- **Debug Mode**: `APP_DEBUG=true` for detailed error messages
- **Exposed Ports**: All service ports accessible locally
- **Development Tools**: Access to debugging ports and development utilities (Kibana)

```bash
# Start development environment
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d

# View logs
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f

# Stop development environment
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml down
```

### Production Mode

Production mode includes:
- **Optimized Builds**: Code baked into images, no volume mounts
- **Security**: Debug disabled, SSL/TLS ready, secure defaults
- **Resource Limits**: CPU and memory limits configured
- **High Availability**: `restart: always` policy

```bash
# Build production images
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml build

# Start production environment
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d

# Stop production environment
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml down
```

**⚠️ Important for Production:**
- Set all required environment variables in `.env` (no defaults)
- Configure SSL certificates for Nginx (see `/infrastructure/certbot/README.md`)
- Enable Elasticsearch security (`ELASTICSEARCH_SECURITY_ENABLED=true`)
- Set strong Redis password (`REDIS_PASSWORD`)
- Use SSL for database connections (`DB_SSLMODE=require`)
- Generate strong JWT secret: `openssl rand -hex 32`

### Service Isolation

Each service has its own:
- Database (PostgreSQL instances)
- Configuration files
- Docker image

### Working on Individual Services

You can run specific services using the compose files:

```bash
# Run only infrastructure (databases, Redis, Elasticsearch)
docker compose -f docker-compose.base.yml up -d

# Run a specific service in development mode
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d source-manager

# Run multiple services
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d crawler classifier publisher-api
```

## Environment Variables

Key environment variables (see `.env.example` for full list):

### Authentication
- `AUTH_USERNAME`: Dashboard username (required)
- `AUTH_PASSWORD`: Dashboard password (required)
- `AUTH_JWT_SECRET`: Shared JWT secret for token signing/validation (required in production)

### Service Ports
- `CRAWLER_PORT`: Crawler API port (default: 8060)
- `SOURCE_MANAGER_PORT`: Source manager port (default: 8050)
- `CLASSIFIER_PORT`: Classifier port (default: 8071)
- `PUBLISHER_PORT`: Publisher API port (default: 8070)
- `SEARCH_PORT`: Search service port (default: 8092)
- `DASHBOARD_PORT`: Dashboard port (default: 3002)
- `AUTH_PORT`: Auth service port (default: 8040)

### Database Configuration
- `POSTGRES_*_USER`: Database users for each service
- `POSTGRES_*_PASSWORD`: Database passwords for each service
- `POSTGRES_*_DB`: Database names

### Infrastructure
- `ELASTICSEARCH_PORT`: Elasticsearch port (default: 9200)
- `REDIS_PORT`: Redis port (default: 6379)
- `REDIS_PASSWORD`: Redis password (optional)
- `MINIO_ROOT_USER`: MinIO admin user (default: minioadmin)
- `MINIO_ROOT_PASSWORD`: MinIO admin password (default: changeme123)

### Application Settings
- `APP_DEBUG`: Enable debug mode (true/false)
- `ELASTICSEARCH_SECURITY_ENABLED`: Enable Elasticsearch security (default: false)

## Database Management

Each service has its own database:
- `gosources`: Source manager database
- `crawler`: Crawler database
- `classifier`: Classifier database
- `index_manager`: Index manager database
- `publisher`: Publisher database

Access databases:

```bash
# Source manager database
docker exec -it north-cloud-postgres-source-manager psql -U postgres -d gosources

# Crawler database
docker exec -it north-cloud-postgres-crawler psql -U postgres -d crawler

# Classifier database
docker exec -it north-cloud-postgres-classifier psql -U postgres -d classifier

# Index manager database
docker exec -it north-cloud-postgres-index-manager psql -U postgres -d index_manager

# Publisher database
docker exec -it north-cloud-postgres-publisher psql -U postgres -d publisher
```

## Stopping Services

```bash
# Stop all services (development)
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml down

# Stop all services (production)
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml down

# Stop and remove volumes (⚠️ deletes data)
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml down -v
```

## Building Images

```bash
# Build all images for development
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml build

# Build all images for production
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml build

# Build specific service
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml build source-manager

# Rebuild and restart
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d --build
```

## Logs

```bash
# View all logs (development)
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f

# View all logs (production)
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml logs -f

# View specific service logs
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f source-manager
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f publisher-api
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f classifier

# View last 100 lines
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs --tail=100 -f
```

## Authentication

The platform uses JWT-based authentication for dashboard and API access:

1. **Login**: POST to `/api/auth/api/v1/auth/login` with username/password
2. **Token**: Receive JWT token with 24-hour expiration
3. **API Requests**: Include token in `Authorization: Bearer <token>` header
4. **Protected Routes**: All `/api/v1/*` routes require authentication (except `/health` endpoints)

See `/auth/README.md` for detailed authentication documentation.

## Service Documentation

Each service has its own documentation:

- **Crawler**: `/crawler/README.md` - Web crawler with interval-based job scheduling
- **Source Manager**: `/source-manager/README.md` - Source management API and UI
- **Classifier**: `/classifier/README.md` - Content classification service
- **Publisher**: `/publisher/README.md` - Database-backed Redis pub/sub routing
- **Index Manager**: `/index-manager/README.md` - Elasticsearch index management
- **Search**: `/search/README.md` - Full-text search service
- **Auth**: `/auth/README.md` - Authentication service
- **Infrastructure**: `/infrastructure/certbot/README.md` - SSL/TLS certificate management

## Troubleshooting

### Port conflicts

If ports are already in use, update the port mappings in `.env` or the docker-compose files.

### Database connection issues

Ensure services wait for databases to be healthy using `depends_on` with `condition: service_healthy`. Check database logs:

```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs postgres-source-manager
```

### Elasticsearch memory

If Elasticsearch fails to start, you may need to increase available memory. Adjust `ES_JAVA_OPTS` in `.env`:

```bash
ELASTICSEARCH_MIN_HEAP=1g
ELASTICSEARCH_MAX_HEAP=1g
```

### Service won't start

1. Check logs: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs <service-name>`
2. Check environment variables: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml config`
3. Verify dependencies: Ensure dependent services are healthy
4. Check port conflicts: `netstat -tulpn | grep PORT`

### Authentication issues

- Verify `AUTH_JWT_SECRET` is set and matches across all services
- Check auth service logs: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs auth`
- Ensure token is included in API requests: `Authorization: Bearer <token>`

### Permission issues (development)

If you encounter permission errors with mounted volumes:

```bash
# Remove cache volumes and let Docker recreate them
docker volume rm crawler_go_mod_cache crawler_go_build_cache

# Or adjust UID/GID in docker-compose.dev.yml
UID=1000 GID=1000 docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d
```

## Additional Resources

- **CLAUDE.md**: Comprehensive AI assistant guide with architecture details
- **DOCKER.md**: Docker-specific documentation and quick reference
- **Service READMEs**: Individual service documentation in each service directory
- **Infrastructure Docs**: SSL/TLS, nginx, and other infrastructure documentation

## License

[Add your license here]

## Contributing

[Add contributing guidelines here]
