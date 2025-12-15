# Database Migrations

This directory contains database migrations for the crawler service.

## Setup

The crawler uses PostgreSQL for storing job data. Migrations are managed using plain SQL files.

### Required Go Dependencies

```bash
go get github.com/jmoiron/sqlx
go get github.com/lib/pq
go get github.com/google/uuid
```

### Running Migrations

#### Manual Migration

Connect to the database and run the migration files:

```bash
# Connect to database
docker exec -it north-cloud-postgres-crawler psql -U postgres -d gocrawl

# Run migration
\i /migrations/001_create_jobs_table.up.sql
```

#### Using golang-migrate (Recommended)

1. Install golang-migrate:
```bash
# macOS
brew install golang-migrate

# Linux
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.15.2/migrate.linux-amd64.tar.gz | tar xvz
sudo mv migrate /usr/local/bin/
```

2. Run migrations:
```bash
# Set database URL
export DATABASE_URL="postgresql://postgres:postgres@localhost:5433/gocrawl?sslmode=disable"

# Run all pending migrations
migrate -path ./migrations -database "$DATABASE_URL" up

# Rollback last migration
migrate -path ./migrations -database "$DATABASE_URL" down 1
```

## Migrations

### 001_create_jobs_table

Creates the `jobs` table for storing crawler jobs with the following fields:

- `id` (UUID): Primary key
- `source_id` (VARCHAR): ID of the source from source-manager
- `source_name` (VARCHAR): Name of the source
- `url` (TEXT): URL to crawl
- `schedule_time` (VARCHAR): Cron expression for scheduling
- `schedule_enabled` (BOOLEAN): Whether scheduled crawling is enabled
- `status` (VARCHAR): Job status (pending, processing, completed, failed)
- `created_at` (TIMESTAMP): When the job was created
- `updated_at` (TIMESTAMP): When the job was last updated
- `started_at` (TIMESTAMP): When the job started processing
- `completed_at` (TIMESTAMP): When the job completed
- `error_message` (TEXT): Error message if job failed

The migration also creates:
- Indexes on `source_id`, `status`, and `created_at` for better query performance
- A trigger to automatically update the `updated_at` timestamp

==========

cd /home/jones/dev/north-cloud/crawler && docker run --rm --network north-cloud_north-cloud-network -v "$(pwd)/migrations:/migrations" migrate/migrate -path /migrations -database "postgresql://postgres:postgres@postgres-crawler:5432/gocrawl?sslmode=disable" up 2>&1
