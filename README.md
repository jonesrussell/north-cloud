# North Cloud

A microservices-based content management and publishing platform built with Go and Drupal.

## Architecture

The project consists of multiple independent services that work together:

- **crawler**: Web crawler service (crawler) for scraping content
- **source-manager**: Go API with Vue.js frontend for managing content sources
- **publisher**: Service that publishes content from Elasticsearch to Drupal
- **streetcode**: Drupal 11 CMS for content presentation and management

## Project Structure

```
project-root/
├── docker-compose.base.yml     # Base configuration (infrastructure services)
├── docker-compose.dev.yml      # Development overrides
├── docker-compose.prod.yml     # Production overrides
├── .env                        # Environment variables (not committed)
├── .env.example               # Environment variables template
├── .gitignore
├── README.md
│
├── crawler/                    # crawler service
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   ├── cmd/
│   ├── internal/
│   └── pkg/
│
├── source-manager/             # Go + Vue frontend
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   ├── internal/
│   ├── frontend/               # Vue app
│   └── migrations/
│
├── publisher/                  # Posts to Drupal
│   ├── Dockerfile
│   ├── go.mod
│   ├── main.go
│   └── internal/
│
├── streetcode/                 # Drupal 11
│   ├── Dockerfile
│   ├── composer.json
│   ├── web/
│   └── config/
│
└── infrastructure/             # Shared configs
    ├── nginx/
    ├── elasticsearch/
    └── postgres/
```

## Prerequisites

- Docker and Docker Compose
- Go 1.24+ (for local development)
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

### 3. Start all services

#### Development Mode (Recommended for local development)

```bash
# Start in development mode with hot reloading
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d
```

**Development mode features:**
- Source code mounted as volumes for hot reloading
- Debug mode enabled (`APP_DEBUG=true`)
- All ports exposed for local access
- Development-friendly container names

#### Production Mode

```bash
# Start in production mode
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d
```

**Production mode features:**
- Code baked into images (no volume mounts)
- Debug mode disabled (`APP_DEBUG=false`)
- Resource limits configured
- Security hardening enabled
- SSL/TLS ready

#### Quick Start (Uses base + dev by default)

For convenience, you can create an alias or use the base + dev combination:

```bash
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d
```

This will start all services:
- **PostgreSQL databases** (3 instances: source-manager, crawler, streetcode)
- **Elasticsearch** (for publisher service)
- **Redis** (for publisher queue and deduplication)
- **crawler** service
- **source-manager** service (port 8050)
- **publisher** service
- **streetcode** (Drupal 11 on port 8080)
- **nginx** reverse proxy (port 80)

### 4. Access services

- Source Manager API: http://localhost:8050
- Streetcode (Drupal): http://localhost:8080
- Elasticsearch: http://localhost:9200
- Redis: localhost:6379

## Development

### Development vs Production

The project uses separate Docker Compose configurations for different environments:

- **`docker-compose.base.yml`**: Shared infrastructure services (databases, Redis, Elasticsearch)
- **`docker-compose.dev.yml`**: Development overrides (hot reloading, debug mode, volume mounts)
- **`docker-compose.prod.yml`**: Production overrides (optimized builds, security, resource limits)

### Development Mode

Development mode includes:
- **Hot Reloading**: Source code mounted as volumes for live code changes
- **Debug Mode**: `APP_DEBUG=true` for detailed error messages
- **Exposed Ports**: All service ports accessible locally
- **Development Tools**: Access to debugging ports and development utilities

```bash
# Start development environment
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d

# View logs
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f

# Stop development environment
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml down
```

### Production Mode

Production mode includes:
- **Optimized Builds**: Code baked into images, no volume mounts
- **Security**: Debug disabled, SSL/TLS ready, secure defaults
- **Resource Limits**: CPU and memory limits configured
- **High Availability**: `restart: always` policy

```bash
# Build production images
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml build

# Start production environment
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d

# Stop production environment
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml down
```

**⚠️ Important for Production:**
- Set all required environment variables in `.env` (no defaults)
- Configure SSL certificates for Nginx
- Enable Elasticsearch security (`ELASTICSEARCH_SECURITY_ENABLED=true`)
- Set strong Redis password (`REDIS_PASSWORD`)
- Use SSL for database connections (`DB_SSLMODE=require`)

### Service Isolation

Each service has its own:
- Database (PostgreSQL instances)
- Configuration files
- Docker image

### Working on Individual Services

You can run specific services using the compose files:

```bash
# Run only infrastructure (databases, Redis, Elasticsearch)
docker-compose -f docker-compose.base.yml up -d postgres-source-manager

# Run a specific service in development mode
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d source-manager
```

## Environment Variables

Key environment variables (see `.env.example` for full list):

- `APP_DEBUG`: Enable debug mode (true/false)
- `SOURCE_MANAGER_PORT`: Port for source-manager service
- `STREETCODE_PORT`: Port for Drupal
- `POSTGRES_*_USER`: Database users
- `POSTGRES_*_PASSWORD`: Database passwords
- `ELASTICSEARCH_PORT`: Elasticsearch port
- `REDIS_PORT`: Redis port
- `DRUPAL_TOKEN`: API token for Drupal authentication

## Database Management

Each service has its own database:
- `gosources`: Source manager database
- `crawler`: Crawler database
- `streetcode`: Drupal database

Access databases:

```bash
# Source manager database
docker exec -it north-cloud-postgres-source-manager psql -U postgres -d gosources

# Crawler database
docker exec -it north-cloud-postgres-crawler psql -U postgres -d crawler

# Streetcode database
docker exec -it north-cloud-postgres-streetcode psql -U postgres -d streetcode
```

## Stopping Services

```bash
# Stop all services (development)
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml down

# Stop all services (production)
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml down

# Stop and remove volumes (⚠️ deletes data)
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml down -v
```

## Building Images

```bash
# Build all images for development
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml build

# Build all images for production
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml build

# Build specific service
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml build source-manager
```

## Logs

```bash
# View all logs (development)
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f

# View all logs (production)
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml logs -f

# View specific service logs
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f source-manager
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f publisher
```

## Troubleshooting

### Port conflicts

If ports are already in use, update the port mappings in `.env` or `docker-compose.yml`.

### Database connection issues

Ensure services wait for databases to be healthy using `depends_on` with `condition: service_healthy`.

### Elasticsearch memory

If Elasticsearch fails to start, you may need to increase available memory. Adjust `ES_JAVA_OPTS` in `.env`.

## License

[Add your license here]

## Contributing

[Add contributing guidelines here]

