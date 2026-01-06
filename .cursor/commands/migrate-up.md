---
description: Apply database migrations for a service
variables:
  - name: SERVICE
    description: Service name with database (crawler, source-manager, publisher, index-manager)
    default: crawler
---

# Apply Migrations

Applies pending database migrations for a service using golang-migrate.

## Usage

This command will:
1. Navigate to the service directory
2. Run `task migrate:up` to apply pending migrations
3. Update the schema to the latest version

## Services with Databases

- `crawler` - PostgreSQL (jobs, executions)
- `source-manager` - PostgreSQL (sources)
- `publisher` - PostgreSQL (sources, channels, routes, publish_history)
- `index-manager` - PostgreSQL (index metadata)

## Command

```bash
cd /home/jones/dev/north-cloud/$SERVICE && task migrate:up
```

## Example

```bash
# Apply crawler migrations
SERVICE=crawler
```

## What Happens

- Executes all pending `.up.sql` migration files
- Updates schema_migrations table
- Prints migration results
- Fails if migration has errors (safe)

## Related Commands

- Use `migrate-down.md` to rollback migrations
- Check migration status with service logs
