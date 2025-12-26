# Database Migrations

This directory contains database migrations for the auth service.

## Setup

The auth service uses PostgreSQL for storing user data. Migrations are managed using golang-migrate.

### Required Go Dependencies

The migration tool is integrated into the auth service binary. No external dependencies needed.

### Running Migrations

#### Using the Auth Service Binary (Recommended)

```bash
# Run all pending migrations
docker exec -it north-cloud-auth /app/auth migrate up

# Rollback last migration
docker exec -it north-cloud-auth /app/auth migrate down

# Rollback N migrations
docker exec -it north-cloud-auth /app/auth migrate down 2

# Check current migration version
docker exec -it north-cloud-auth /app/auth migrate version

# Force migration version (for fixing dirty state)
docker exec -it north-cloud-auth /app/auth migrate force 1
```

#### Using Taskfile

```bash
cd auth
task migrate:up      # Run migrations up
task migrate:down    # Rollback last migration
task migrate:version # Check migration version
```

#### Using golang-migrate CLI (Alternative)

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
export DATABASE_URL="postgresql://postgres:postgres@localhost:5432/auth?sslmode=disable"

# Run all pending migrations
migrate -path ./migrations -database "$DATABASE_URL" up

# Rollback last migration
migrate -path ./migrations -database "$DATABASE_URL" down 1
```

#### Using Docker (Alternative)

```bash
docker run --rm --network north-cloud_north-cloud-network \
  -v "$(pwd)/auth/migrations:/migrations" \
  migrate/migrate \
  -path /migrations \
  -database "postgresql://postgres:postgres@postgres-auth:5432/auth?sslmode=disable" \
  up
```

## Migrations

### 000001_create_users_table

Creates the `users` table for storing authentication user data with the following fields:

- `id` (UUID): Primary key, auto-generated
- `username` (VARCHAR): Unique username
- `email` (VARCHAR): Unique email address
- `password_hash` (VARCHAR): Bcrypt-hashed password
- `created_at` (TIMESTAMP): When the user was created
- `updated_at` (TIMESTAMP): When the user was last updated

The migration also creates:
- Index on `username` for faster lookups
- Index on `email` for faster lookups

## Migration Versioning

golang-migrate automatically tracks migration versions in a `schema_migrations` table:
- `version`: Current migration version number
- `dirty`: Whether migration is in a dirty state (failed mid-execution)

## Creating New Migrations

To create a new migration, use the Taskfile:

```bash
cd auth
task migrate:create add_user_roles
```

This creates:
- `000002_add_user_roles.up.sql`
- `000002_add_user_roles.down.sql`

Edit these files with your migration SQL, then run:

```bash
task migrate:up
```

## Automatic Migrations

The auth service supports automatic migration on startup using the `-auto-migrate` flag or `AUTO_MIGRATE=true` environment variable:

```bash
docker exec -it north-cloud-auth /app/auth -auto-migrate
```

This will:
1. Run all pending migrations
2. Start the HTTP server if migrations succeed
3. Exit with error if migrations fail (prevents starting with bad schema)

## Troubleshooting

### Dirty Migration State

If a migration fails mid-execution, the database may be in a "dirty" state. To fix:

```bash
# Check current version
docker exec -it north-cloud-auth /app/auth migrate version

# Force to a specific version (fixes dirty state)
docker exec -it north-cloud-auth /app/auth migrate force 1
```

### Manual Migration Rollback

If you need to manually rollback:

```bash
# Rollback last migration
docker exec -it north-cloud-auth /app/auth migrate down

# Rollback multiple migrations
docker exec -it north-cloud-auth /app/auth migrate down 3
```

