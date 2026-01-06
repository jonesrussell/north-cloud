---
description: Start the full development environment
---

# Start Development Environment

Starts all North Cloud services and infrastructure in Docker development mode.

## Usage

This command will:
1. Navigate to the project root
2. Start all services defined in docker-compose files
3. Launch services in detached mode (background)
4. Initialize infrastructure (PostgreSQL, Elasticsearch, Redis, MinIO)

## Command

```bash
cd /home/jones/dev/north-cloud && \
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d
```

## What Gets Started

**Application Services:**
- Crawler (port 8060)
- Source Manager (port 8050)
- Classifier (port 8070)
- Publisher API (port 8080)
- Publisher Router (background)
- Index Manager (port 8090)
- Search (port 8090)
- Auth (port 8040)
- Dashboard (port 3002)
- MCP Server

**Infrastructure:**
- PostgreSQL (5 instances)
- Elasticsearch (port 9200)
- Redis (port 6379)
- MinIO (ports 9000, 9001)
- Nginx (ports 80, 443)

## Checking Status

After starting, verify services are running:
```bash
docker compose ps
```

## Viewing Logs

To monitor all services:
```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f
```

## Development Features

- Hot reloading with Air (Go services)
- Source code mounted as volumes
- Detailed debug logging
- pprof endpoints exposed (ports 6060-6066)

## Related Commands

- Use `stop-dev-env.md` to stop all services
- Use `view-logs.md` to monitor specific services
- Use `check-health.md` to verify service health
