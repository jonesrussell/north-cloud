---
description: Build a service Docker image for development
variables:
  - name: SERVICE
    description: Service name (crawler, source-manager, classifier, publisher, index-manager, search, auth, dashboard, mcp-north-cloud)
    default: crawler
---

# Build Service

Builds a Docker image for a specific North Cloud service in development mode.

## Usage

This command will:
1. Build the Docker image for the specified service
2. Include development dependencies and tools
3. Configure for hot-reloading (where applicable)

## Buildable Services

- `crawler`
- `source-manager`
- `classifier`
- `publisher`
- `index-manager`
- `search`
- `auth`
- `dashboard`
- `mcp-north-cloud`

## Command

```bash
cd /home/jones/dev/north-cloud && docker compose -f docker-compose.base.yml -f docker-compose.dev.yml build $SERVICE
```

## Example

```bash
# Build the crawler service
SERVICE=crawler

# Build the publisher service
SERVICE=publisher
```

## Build Options

**Force rebuild (no cache):**
```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml build --no-cache $SERVICE
```

**Build with progress output:**
```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml build --progress=plain $SERVICE
```

## After Building

Restart the service to use the new image:
```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d $SERVICE
```

Or use `restart-service.md` command.

## Related Commands

- Use `build-prod.md` for production builds
- Use `restart-service.md` to restart after building
- Use `start-service.md` to start a service
- Use `view-logs.md` to check build results
