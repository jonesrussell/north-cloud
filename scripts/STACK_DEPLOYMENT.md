# Docker Swarm Stack Deployment Guide

## Overview

Docker Swarm (`docker stack deploy`) does not support the `build` directive in docker-compose files. Images must be built separately before deploying the stack.

## Quick Start

### 1. Build All Images

Run the build script to create all required images:

```bash
./scripts/build-stack-images.sh
```

This will build all images with the tags:
- `northcloud/search-service:latest`
- `northcloud/search-frontend:latest`
- `northcloud/auth:latest`
- `northcloud/crawler:latest`
- `northcloud/source-manager:latest`
- `northcloud/publisher:latest`
- `northcloud/classifier:latest`
- `northcloud/dashboard:latest`
- `northcloud/index-manager:latest` (if Dockerfile exists)

### 2. Deploy the Stack

```bash
docker stack deploy -c docker-compose.base.yml -c docker-compose.prod.yml northcloud
```

### 3. Check Stack Status

```bash
# View stack services
docker stack services northcloud

# View stack tasks
docker stack ps northcloud

# View logs
docker service logs northcloud_<service-name>
```

## Manual Image Building

If you prefer to build images manually:

```bash
# Search service
docker build -t northcloud/search-service:latest -f ./search/Dockerfile .

# Search frontend
docker build -t northcloud/search-frontend:latest -f ./search-frontend/Dockerfile ./search-frontend

# Auth
docker build -t northcloud/auth:latest -f ./auth/Dockerfile .

# Crawler
docker build -t northcloud/crawler:latest --build-arg BUILD_ENV=production -f ./crawler/Dockerfile .

# Source Manager
docker build -t northcloud/source-manager:latest --build-arg BUILD_ENV=production -f ./source-manager/Dockerfile .

# Publisher
docker build -t northcloud/publisher:latest --build-arg BUILD_ENV=production -f ./publisher/Dockerfile .

# Classifier
docker build -t northcloud/classifier:latest --build-arg BUILD_ENV=production -f ./classifier/Dockerfile .

# Dashboard
docker build -t northcloud/dashboard:latest --build-arg BUILD_ENV=production -f ./dashboard/Dockerfile ./dashboard
```

## Using a Docker Registry

For multi-node Swarm clusters, push images to a registry:

```bash
# Tag images for your registry
docker tag northcloud/crawler:latest your-registry.com/northcloud/crawler:latest

# Push to registry
docker push your-registry.com/northcloud/crawler:latest

# Update image names in compose files to use registry URLs
```

## Updating the Stack

After making code changes:

1. Rebuild affected images:
   ```bash
   ./scripts/build-stack-images.sh
   ```

2. Update the stack:
   ```bash
   docker stack deploy -c docker-compose.base.yml -c docker-compose.prod.yml northcloud
   ```

## Removing the Stack

```bash
docker stack rm northcloud
```

## Notes

- The compose files have been modified to remove `build` directives for stack compatibility
- For development, use `docker compose` (not `docker stack deploy`) which supports `build` directives
- Development workflow: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d`
