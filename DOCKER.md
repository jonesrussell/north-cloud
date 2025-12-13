# Docker Compose Quick Reference

This project uses a multi-file Docker Compose setup for different environments.

## File Structure

- `docker-compose.base.yml` - Shared infrastructure (databases, Redis, Elasticsearch)
- `docker-compose.dev.yml` - Development overrides (hot reloading, debug mode)
- `docker-compose.prod.yml` - Production overrides (optimized, secure)

## Common Commands

### Development

```bash
# Start development environment
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d

# Stop development environment
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml down

# View logs
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f

# Rebuild and restart
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d --build

# Execute command in container
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml exec streetcode bash
```

### Production

```bash
# Build production images
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml build

# Start production environment
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d

# Stop production environment
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml down

# View logs
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml logs -f

# Update production (rebuild and restart)
docker-compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d --build
```

### Infrastructure Only

```bash
# Start only databases and infrastructure
docker-compose -f docker-compose.base.yml up -d

# Stop infrastructure
docker-compose -f docker-compose.base.yml down
```

## Service-Specific Commands

### Database Access

```bash
# Source manager database
docker exec -it north-cloud-postgres-source-manager psql -U postgres -d gosources

# Crawler database
docker exec -it north-cloud-postgres-crawler psql -U postgres -d gocrawl

# Streetcode database
docker exec -it north-cloud-postgres-streetcode psql -U postgres -d streetcode
```

### Drupal (Streetcode)

```bash
# Access Drupal container
docker exec -it north-cloud-streetcode-dev bash

# Clear Drupal cache
docker exec -it north-cloud-streetcode-dev /var/www/html/vendor/bin/drush cr

# Run Drupal updates
docker exec -it north-cloud-streetcode-dev /var/www/html/vendor/bin/drush updb
```

### View Service Status

```bash
# List running containers
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml ps

# Check service health
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml ps --format json | jq '.[] | {name: .Name, status: .State, health: .Health}'
```

## Environment Variables

Create a `.env` file in the project root with your configuration:

```bash
# Copy example (if exists)
cp .env.example .env

# Edit with your values
nano .env
```

Key variables:
- `APP_DEBUG` - Enable/disable debug mode
- `POSTGRES_*_PASSWORD` - Database passwords
- `REDIS_PASSWORD` - Redis password (required in production)
- `DRUPAL_TOKEN` - Drupal API token
- `ELASTICSEARCH_SECURITY_ENABLED` - Enable ES security (production)

## Troubleshooting

### Port Conflicts

If ports are already in use, update them in `.env`:

```bash
SOURCE_MANAGER_PORT=8051
STREETCODE_PORT=8081
```

### Container Won't Start

```bash
# Check logs
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml logs <service-name>

# Check container status
docker ps -a | grep north-cloud

# Restart specific service
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml restart <service-name>
```

### Clean Slate

```bash
# Stop and remove everything (including volumes)
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml down -v

# Remove all images
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml down --rmi all

# Prune system
docker system prune -a
```

## Differences: Development vs Production

| Feature | Development | Production |
|---------|------------|------------|
| Code Mounting | Volume mounts for hot reload | Baked into image |
| Debug Mode | Enabled | Disabled |
| Resource Limits | None | CPU/Memory limits |
| Security | Relaxed | Hardened |
| SSL/TLS | Optional | Required |
| Restart Policy | `unless-stopped` | `always` |
| Container Names | `-dev` suffix | Standard names |
