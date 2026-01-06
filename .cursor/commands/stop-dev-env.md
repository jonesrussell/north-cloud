---
description: Stop the development environment
---

# Stop Development Environment

Stops all North Cloud services and removes containers (preserves data volumes).

## Usage

This command will:
1. Navigate to the project root
2. Stop all running Docker containers
3. Remove containers (but keep volumes and data)
4. Clean up networks

## Command

```bash
cd /home/jones/dev/north-cloud && \
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml down
```

## What Gets Stopped

**All application services:**
- Crawler, Source Manager, Classifier
- Publisher (API and Router)
- Index Manager, Search, Auth
- Dashboard, MCP Server

**All infrastructure:**
- PostgreSQL databases
- Elasticsearch
- Redis
- MinIO
- Nginx

## Data Preservation

This command **preserves**:
- Database data (PostgreSQL volumes)
- Elasticsearch indexes
- Redis data
- MinIO stored files

## Complete Cleanup (⚠️ Deletes Data)

To remove volumes and delete all data:
```bash
cd /home/jones/dev/north-cloud && \
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml down -v
```

## Verifying Shutdown

Check that all containers are stopped:
```bash
docker ps | grep north-cloud
```

Should return no results if fully stopped.

## Related Commands

- Use `start-dev-env.md` to restart the environment
- Use `restart-service.md` to restart a single service
