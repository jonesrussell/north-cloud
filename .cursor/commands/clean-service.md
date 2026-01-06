---
description: Clean build artifacts for a service
variables:
  - name: SERVICE
    description: Service name (crawler, source-manager, classifier, publisher, index-manager, search, auth)
    default: crawler
---

# Clean Service

Removes build artifacts, test coverage files, and temporary files for a service.

## Usage

This command will:
1. Navigate to the service directory
2. Remove compiled binaries
3. Delete coverage reports
4. Clean temporary build files
5. Remove cached test results

## Command

```bash
cd /home/jones/dev/north-cloud/$SERVICE && task clean
```

## Example

```bash
# Clean crawler build artifacts
SERVICE=crawler
```

## What Gets Removed

- `bin/` - Compiled binaries
- `tmp/` - Air hot-reload temporary files
- `coverage.out` - Test coverage data
- `coverage.html` - Coverage HTML reports
- `*.test` - Test binaries
- Build cache

## When to Clean

- After switching branches
- Before fresh rebuild
- To free up disk space
- When build cache is corrupted
- After major refactoring

## Clean All Services

To clean all services at once:
```bash
cd /home/jones/dev/north-cloud && \
for service in crawler source-manager classifier publisher index-manager search auth; do \
  (cd $service && task clean); \
done
```

## Rebuild After Cleaning

After cleaning, rebuild the service:
```bash
cd /home/jones/dev/north-cloud/$SERVICE && task build
```

Or restart in Docker:
```bash
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml build $SERVICE
```

## Related Commands

- Use `restart-service.md` to rebuild and restart
- Use `build-prod.md` for production builds
