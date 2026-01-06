---
description: Rollback last database migration
variables:
  - name: SERVICE
    description: Service name with database (crawler, source-manager, publisher, index-manager)
    default: crawler
---

# Rollback Migration

Rolls back the last applied database migration for a service.

## Usage

This command will:
1. Navigate to the service directory
2. Run `task migrate:down` to rollback the last migration
3. Execute the `.down.sql` file for the latest migration

## Command

```bash
cd /home/jones/dev/north-cloud/$SERVICE && task migrate:down
```

## Example

```bash
# Rollback publisher migration
SERVICE=publisher
```

## Warning

⚠️ This can result in data loss if the down migration drops tables or columns.

## When to Use

- Testing new migrations
- Fixing migration errors
- Development/testing only (avoid in production)

## Related Commands

- Use `migrate-up.md` to re-apply migrations
