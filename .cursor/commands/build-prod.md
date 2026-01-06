---
description: Build production Docker image
variables:
  - name: SERVICE
    description: Service name to build (or empty for all services)
    default: crawler
---

# Build Production Image

Builds optimized production Docker images with code baked in (no volume mounts).

## Usage

This command will:
1. Build multi-stage Docker image
2. Compile Go code with optimization flags
3. Create minimal Alpine-based image
4. Tag image for production deployment

## Command

```bash
cd /home/jones/dev/north-cloud && ./scripts/build-prod.sh $SERVICE
```

## Examples

```bash
# Build single service
SERVICE=crawler

# Build all services
SERVICE=""
```

## Build Features

**Production optimizations:**
- Multi-stage build (small final image)
- Code compiled with `-ldflags="-s -w"` (strip debug info)
- Alpine Linux base (minimal size)
- No development tools included
- Security scanning ready

**Image characteristics:**
- No source code mounted
- No hot-reload tools
- Optimized binary
- Minimal attack surface
- Fast startup time

## Build Output

- Docker image tagged as `north-cloud-{service}:latest`
- Build logs showing compilation
- Final image size
- Layer information

## Deployment

After building, push to registry:
```bash
docker tag north-cloud-$SERVICE:latest registry.example.com/north-cloud-$SERVICE:1.0.0
docker push registry.example.com/north-cloud-$SERVICE:1.0.0
```

## Production Deployment

Use with `docker-compose.prod.yml`:
```bash
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d
```

## Build All Services

To build all services for production:
```bash
cd /home/jones/dev/north-cloud && ./scripts/build-prod.sh
```

## Related Commands

- Use `start-dev-env.md` for development builds
- Use `test-all.md` before building production images
