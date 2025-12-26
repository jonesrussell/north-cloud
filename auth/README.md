# Auth Service

Authentication service for the North Cloud platform. Provides JWT-based authentication, user management, and session handling for the dashboard.

## Features

- JWT token generation and validation
- User authentication (login/logout)
- Password hashing with bcrypt
- Token refresh mechanism
- Database-backed user management
- Migration management with golang-migrate

## Quick Start

### Running the Service

```bash
# Development
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d auth

# Production
docker compose -f docker-compose.base.yml -f docker-compose.prod.yml up -d auth
```

### Running Migrations

```bash
# Using the service binary
docker exec -it north-cloud-auth /app/auth migrate up

# Using Taskfile
cd auth && task migrate:up

# From root
task migrate:auth
```

### Creating Admin User

```bash
# Using the service binary
docker exec -it north-cloud-auth /app/auth seed -username admin -password $AUTH_ADMIN_PASSWORD

# Using Taskfile
cd auth && task seed
```

## Configuration

### Environment Variables

- `AUTH_SERVICE_PORT` - Service port (default: 8040)
- `AUTH_SERVICE_HOST` - Service host (default: 0.0.0.0)
- `POSTGRES_AUTH_HOST` - Database host
- `POSTGRES_AUTH_PORT` - Database port (default: 5432)
- `POSTGRES_AUTH_USER` - Database user
- `POSTGRES_AUTH_PASSWORD` - Database password
- `POSTGRES_AUTH_DB` - Database name (default: auth)
- `AUTH_JWT_SECRET` - JWT signing secret (required)
- `AUTH_JWT_EXPIRY` - Token expiration (default: 24h)
- `AUTH_JWT_REFRESH_EXPIRY` - Refresh token expiration (default: 168h)
- `AUTO_MIGRATE` - Run migrations on startup (true/false)
- `AUTH_ADMIN_PASSWORD` - Password for seed command

### Config File

See `config.yml` for configuration file format.

## API Endpoints

### Authentication

- `POST /api/v1/auth/login` - Login (username/password → JWT)
- `POST /api/v1/auth/logout` - Logout
- `GET /api/v1/auth/validate` - Validate current token
- `POST /api/v1/auth/refresh` - Refresh expired token

### Health

- `GET /health` - Health check

## Commands

### Migration Commands

```bash
# Run all pending migrations
auth migrate up

# Rollback last migration
auth migrate down

# Rollback N migrations
auth migrate down -steps 2

# Check current version
auth migrate version

# Force migration version (for fixing dirty state)
auth migrate force -version 1
```

### Seed Command

```bash
# Create admin user
auth seed -username admin -password changeme

# With custom email
auth seed -username admin -email admin@example.com -password changeme
```

### Automatic Migrations

Run migrations automatically on service startup:

```bash
# Using flag
auth -auto-migrate

# Using environment variable
AUTO_MIGRATE=true auth
```

## Database Migrations

Migrations are managed using golang-migrate. See `migrations/README.md` for detailed migration documentation.

### Running Migrations

```bash
# Using service binary
docker exec -it north-cloud-auth /app/auth migrate up

# Using Taskfile
cd auth && task migrate:up

# Using golang-migrate CLI
migrate -path ./migrations -database "postgresql://postgres:postgres@localhost:5432/auth?sslmode=disable" up
```

### Creating New Migrations

```bash
# Using Taskfile
cd auth && task migrate:create add_user_roles

# This creates:
# - migrations/000002_add_user_roles.up.sql
# - migrations/000002_add_user_roles.down.sql
```

## Development

### Local Development

```bash
cd auth

# Run service
go run main.go

# Run with auto-migrate
go run main.go -auto-migrate

# Run migrations
go run main.go migrate up

# Create admin user
go run main.go seed -username admin -password changeme
```

### Testing

```bash
# Run tests
go test ./...

# Run with coverage
go test -cover ./...
```

### Building

```bash
# Build binary
go build -o bin/auth main.go

# Build for Docker
docker build -t north-cloud-auth .
```

## Taskfile Commands

```bash
cd auth

task migrate:up      # Run migrations
task migrate:down    # Rollback migrations
task migrate:version # Check version
task migrate:create  # Create new migration
task seed            # Create admin user
task build           # Build binary
task test            # Run tests
task lint            # Lint code
```

## Integration with Dashboard

The auth service is integrated with the dashboard frontend:

1. Dashboard calls `/api/auth/login` with credentials
2. Auth service returns JWT token
3. Dashboard stores token and includes in API requests
4. Token is validated on each request
5. Token refresh on expiration

See `dashboard/src/composables/useAuth.js` for frontend integration.

## Security Considerations

- JWT tokens expire after 24 hours (configurable)
- Refresh tokens expire after 7 days (configurable)
- Passwords are hashed with bcrypt (cost factor 10)
- CORS is configured for dashboard origin
- HTTPS required in production (via nginx)

## Troubleshooting

### Migration Issues

**Dirty migration state:**
```bash
# Check version
auth migrate version

# Force to specific version
auth migrate force -version 1
```

**Migration fails:**
- Check database connection
- Verify migration files are correct
- Check database permissions

### Authentication Issues

**Token validation fails:**
- Verify JWT_SECRET matches between services
- Check token expiration
- Verify token format (Bearer <token>)

**Login fails:**
- Verify user exists in database
- Check password is correct
- Verify database connection

## See Also

- `migrations/README.md` - Migration documentation
- `CLAUDE.md` - AI assistant guide (includes auth service section)

