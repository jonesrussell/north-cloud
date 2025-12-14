# Crawler Setup Guide

## Development Setup with Docker

For the easiest development experience, use Docker Compose:

### 1. Create Configuration File

```bash
cd crawler
cp config.example.yaml config.yaml
```

### 2. Update config.yaml

Edit `config.yaml` to match your Docker environment:

```yaml
server:
  address: ":8060"  # Match CRAWLER_PORT in .env

elasticsearch:
  addresses:
    - "http://elasticsearch:9200"  # Use Docker service name
  tls:
    enabled: false  # Disable for local development
```

### 3. Start Services

From the project root:

```bash
# Start crawler with frontend
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d crawler

# View logs
docker-compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f crawler
```

### 4. Access the Application

- **Frontend Dashboard**: http://localhost:3001
- **Backend API**: http://localhost:8060
- **Health Check**: http://localhost:8060/health

## Configuration Reference

Key configuration sections in `config.yaml`:

### Server Settings
```yaml
server:
  address: ":8060"          # Port to listen on
  read_timeout: 10s
  write_timeout: 30s
```

### Elasticsearch
```yaml
elasticsearch:
  addresses:
    - "http://elasticsearch:9200"
  tls:
    enabled: false          # Set true for production
    skip_verify: true       # Only for development
```

### Crawler Settings
```yaml
crawler:
  sources_api_url: "http://source-manager:8050/api/v1/sources"
  max_depth: 2
  parallelism: 2
```

## Environment Variables

Set these in `.env` or docker-compose:

- `CRAWLER_PORT` - HTTP server port (default: 8060)
- `CRAWLER_HOST` - Host to bind to (default: 0.0.0.0)
- `DB_HOST` - PostgreSQL host
- `DB_NAME` - Database name (default: gocrawl)

## Frontend Development

The crawler includes a Vue.js dashboard. See `frontend/README.md` for details.

```bash
cd frontend
npm install
npm run dev  # Runs on port 3000
```

## Troubleshooting

### Port Mismatch Errors

If you see connection errors, ensure:
1. `config.yaml` server.address matches `CRAWLER_PORT` environment variable
2. Frontend `VITE_API_URL` points to correct backend port
3. Docker port mappings are correct in docker-compose.dev.yml

### Elasticsearch Connection Errors

1. Verify Elasticsearch is running: `curl http://localhost:9200`
2. Check `config.yaml` has correct elasticsearch.addresses
3. Ensure TLS is disabled for local development

### Permission Errors

If you encounter permission errors with node_modules:
```bash
sudo rm -rf frontend/node_modules
docker-compose down
docker volume rm north-cloud_crawler_node_modules  # If it exists
docker-compose up -d crawler
```
