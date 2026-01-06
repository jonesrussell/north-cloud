---
description: Rebuild and restart a service after code changes
variables:
  - name: SERVICE
    description: Service name (crawler, source-manager, classifier, publisher-api, publisher-router, index-manager, search, auth, dashboard)
    default: crawler
---

# Restart Service

Rebuilds the Docker image and restarts a service, showing the last 50 log lines and then tailing.

## Usage

This command will:
1. Rebuild the service's Docker image with latest code
2. Restart the service container
3. Show last 50 lines of logs
4. Tail logs for continued monitoring

Perfect for quick iteration during development.

## Command

```bash
cd /home/jones/dev/north-cloud && \
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml build $SERVICE && \
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d $SERVICE && \
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml logs --tail=50 -f $SERVICE
```

## Example

```bash
# Restart the classifier service after code changes
SERVICE=classifier
```

## When to Use

- After making code changes
- After updating dependencies
- After config changes
- When service is misbehaving

## Press Ctrl+C to stop tailing logs
