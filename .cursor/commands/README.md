# North Cloud Cursor Commands

This directory contains 18 cursor commands for streamlining North Cloud development workflows.

## Command Categories

### Tier 1: High-Frequency Commands (Daily Use)

Essential commands used multiple times per day:

1. **start-service.md** - Start a service in Docker with logs
2. **test-service.md** - Run tests for a service
3. **lint-service.md** - Lint a service with golangci-lint
4. **restart-service.md** - Quick rebuild and restart after code changes
5. **check-health.md** - Check if a service is healthy

### Tier 2: Development Workflow Commands

Commands used during active development:

6. **migrate-up.md** - Apply database migrations
7. **migrate-down.md** - Rollback last migration
8. **run-benchmarks.md** - Run performance benchmarks
9. **profile-service.md** - Capture CPU or memory profiles
10. **test-coverage.md** - Run tests with coverage report

### Tier 3: Multi-Service Operations

Commands that operate across all services:

11. **test-all.md** - Run tests across all services
12. **lint-all.md** - Lint all services in parallel
13. **start-dev-env.md** - Start the full development environment
14. **stop-dev-env.md** - Stop the development environment
15. **view-logs.md** - View logs for a specific service

### Tier 4: Specialized Operations

Advanced commands for debugging and production:

16. **check-memory-leaks.md** - Detect memory leaks
17. **build-prod.md** - Build production Docker image
18. **clean-service.md** - Clean build artifacts

## Usage in Cursor

Commands appear in Cursor's command palette (Cmd/Ctrl + Shift + P) and can be executed with variable substitution.

### Common Variables

**SERVICE** - Service name options:
- `crawler` - Web crawler service (port 8060)
- `source-manager` - Source management (port 8050)
- `classifier` - Content classification (port 8070)
- `publisher-api` - Publisher API (port 8080)
- `publisher-router` - Publisher router (background)
- `index-manager` - Index manager (port 8090)
- `search` - Search service (port 8090)
- `auth` - Authentication (port 8040)
- `dashboard` - Dashboard UI (port 3002)

**PORT** - Service port mappings:
- 8040 - Auth
- 8050 - Source Manager
- 8060 - Crawler
- 8070 - Classifier
- 8080 - Publisher API
- 8090 - Index Manager / Search

**PROFILE_TYPE** - Profiling options:
- `cpu` - CPU usage profiling
- `heap` - Memory allocation profiling
- `goroutine` - Goroutine dump
- `allocs` - Allocation profiling
- `block` - Blocking operations
- `mutex` - Mutex contention

## Quick Reference

### Before Committing
```
1. test-service.md (test your changes)
2. lint-service.md (check code quality)
3. test-all.md (verify no regressions)
```

### Debugging Performance
```
1. profile-service.md (capture profile)
2. run-benchmarks.md (compare performance)
3. check-memory-leaks.md (detect leaks)
```

### Fresh Environment Setup
```
1. start-dev-env.md (start all services)
2. migrate-up.md (apply migrations)
3. check-health.md (verify services)
```

### After Code Changes
```
1. test-service.md (verify changes)
2. restart-service.md (reload code)
3. view-logs.md (monitor behavior)
```

## Command Dependencies

Commands use the following tools:
- **Docker Compose** - Container orchestration
- **Task Runner** - Build automation (taskfile.dev)
- **Go Tools** - Testing, linting, profiling
- **jq** - JSON processing for health checks
- **curl** - HTTP requests

## Directory Structure

```
.cursor/commands/
├── README.md (this file)
├── start-service.md
├── test-service.md
├── lint-service.md
├── restart-service.md
├── check-health.md
├── migrate-up.md
├── migrate-down.md
├── run-benchmarks.md
├── profile-service.md
├── test-coverage.md
├── test-all.md
├── lint-all.md
├── start-dev-env.md
├── stop-dev-env.md
├── view-logs.md
├── check-memory-leaks.md
├── build-prod.md
└── clean-service.md
```

## Tips

- Commands use absolute paths (`/home/jones/dev/north-cloud`) so they work from any directory
- Most commands provide helpful output and error messages
- Use Tab completion in Cursor to select variables
- Commands are idempotent where possible
- Check command documentation for advanced options

## Adding New Commands

To add a new command:

1. Create a `.md` file in this directory
2. Include frontmatter with description and variables
3. Document the command thoroughly
4. Test the command manually
5. Update this README

## Example Command Structure

```markdown
---
description: Brief description
variables:
  - name: VARIABLE_NAME
    description: What it means
    default: default_value
---

# Command Title

What the command does.

## Command

\`\`\`bash
command here with $VARIABLE_NAME
\`\`\`
```

## Support

For issues or suggestions:
- Check service README files in their directories
- See `/CLAUDE.md` for project architecture
- Review `/docs/PROFILING.md` for profiling guide
- Open an issue in the project repository
