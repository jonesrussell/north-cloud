---
description: Start a service in Docker with logs
variables:
  - name: SERVICE
    description: Service name (crawler, source-manager, classifier, publisher-api, publisher-router, index-manager, search, auth, dashboard)
    default: crawler
---

# Start Service

Starts a specific North Cloud service in Docker development mode and tails its logs.

## Usage

This command will:
1. Start the service container in detached mode
2. Automatically tail the service logs

## Common Services

- `crawler` - Web crawler service
- `source-manager` - Source management service
- `classifier` - Content classification service
- `publisher-api` - Publisher API service
- `publisher-router` - Publisher router service
- `index-manager` - Elasticsearch index manager
- `search` - Search service
- `auth` - Authentication service
- `dashboard` - Unified dashboard

## Command

```bash
cd /home/jones/dev/north-cloud && \
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d $SERVICE && \
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs -f $SERVICE
```

## Example

```bash
# Start the crawler service
SERVICE=crawler
```
