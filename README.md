# North Cloud

A microservices-based content management and publishing platform built with Go and Drupal.

## Architecture

The project consists of multiple independent services that work together:

- **crawler**: Web crawler service (gocrawl) for scraping content
- **source-manager**: Go API with Vue.js frontend for managing content sources
- **publisher**: Service that publishes content from Elasticsearch to Drupal
- **streetcode**: Drupal 11 CMS for content presentation and management

## Project Structure

```
project-root/
├── docker-compose.yml          # Main orchestration file
├── .env                        # Environment variables (not committed)
├── .env.example               # Environment variables template
├── .gitignore
├── README.md
│
├── crawler/                    # gocrawl service
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

```bash
docker-compose up -d
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

### Service Isolation

Each service has its own:
- Database (PostgreSQL instances)
- Configuration files
- Docker image

### Working on Individual Services

You can run services independently:

```bash
# Run only source-manager with its database
cd source-manager
docker-compose up -d

# Run only publisher
cd publisher
docker-compose up -d
```

### Hot Reloading

For development with hot reloading:

```bash
# Run services in development mode
docker-compose -f docker-compose.yml -f docker-compose.dev.yml up
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
- `gocrawl`: Crawler database
- `streetcode`: Drupal database

Access databases:

```bash
# Source manager database
docker exec -it north-cloud-postgres-source-manager psql -U postgres -d gosources

# Crawler database
docker exec -it north-cloud-postgres-crawler psql -U postgres -d gocrawl

# Streetcode database
docker exec -it north-cloud-postgres-streetcode psql -U postgres -d streetcode
```

## Stopping Services

```bash
# Stop all services
docker-compose down

# Stop and remove volumes (⚠️ deletes data)
docker-compose down -v
```

## Building Images

```bash
# Build all images
docker-compose build

# Build specific service
docker-compose build source-manager
```

## Logs

```bash
# View all logs
docker-compose logs -f

# View specific service logs
docker-compose logs -f source-manager
docker-compose logs -f publisher
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

