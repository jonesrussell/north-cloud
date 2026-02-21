# Auth

> JWT authentication service for North Cloud. Issues 24-hour tokens accepted by all services.

## Overview

Auth handles credential validation and JWT token issuance for the North Cloud platform. It sits at the entry point of every authenticated request: clients POST credentials here and receive a signed token that all other services verify independently using the shared `AUTH_JWT_SECRET`.

## Features

- Username/password credential validation against environment-configured values
- HS256-signed JWT token generation with configurable expiration (default 24 hours)
- Structured JSON logging with per-request client IP tracking
- pprof profiling server for runtime inspection
- Hot reload in development via Air
- Public health endpoint, no token required

## Quick Start

### Docker (Recommended)

Auth starts as part of the core stack:

```bash
task docker:dev:up
# or
docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d auth
```

Get a token:

```bash
curl -s -X POST http://localhost:8040/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "changeme"}'
```

### Local Development

```bash
cd auth

# Run with hot reload
air

# Or run without hot reload
task run

# Or build and run
task build
./bin/auth
```

## API Reference

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/health` | None | Health check — returns 200 OK |
| POST | `/api/v1/auth/login` | None | Validate credentials and issue JWT |

### POST /api/v1/auth/login

**Request**:
```json
{
  "username": "admin",
  "password": "changeme"
}
```

**Success (200)**:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Invalid credentials (401)**:
```json
{
  "error": "invalid credentials"
}
```

**Malformed request (400)**:
```json
{
  "error": "invalid request"
}
```

The JWT payload contains:

| Claim | Value |
|-------|-------|
| `sub` | `"dashboard"` |
| `iat` | Issued-at timestamp |
| `nbf` | Not-before timestamp (same as `iat`) |
| `exp` | Expiry timestamp (`iat` + expiration duration) |

## Configuration

Configuration is loaded from `config.yml` then overridden by environment variables.

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `AUTH_USERNAME` | `admin` | Yes | Login username |
| `AUTH_PASSWORD` | `admin` | Yes | Login password |
| `AUTH_JWT_SECRET` | `change-me-in-production` | Yes (prod) | HS256 signing secret — must not be the default in non-debug mode |
| `AUTH_PORT` | `8040` | No | HTTP listen port |
| `APP_DEBUG` | `false` | No | Enable debug mode (relaxes JWT secret validation) |
| `LOG_LEVEL` | `info` | No | Log level: `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `json` | No | Log format: `json` or `console` |

Generate a secure JWT secret:

```bash
openssl rand -hex 32
```

## Architecture

```
auth/
├── main.go                    — Entry point: profiling, config, logger, server
├── config.yml                 — Default configuration (override with env vars)
├── Taskfile.yml               — Build, test, lint, benchmark tasks
├── .air.toml                  — Hot reload configuration
└── internal/
    ├── api/
    │   ├── server.go          — Gin server setup, route registration
    │   └── auth_handler.go    — Login handler: credential check, token response
    ├── auth/
    │   └── jwt.go             — JWTManager: GenerateToken, ValidateToken
    └── config/
        └── config.go          — Config struct, defaults, Validate()
```

The service follows the simple bootstrap pattern: helper functions in `main.go` (`loadConfig`, `createLogger`, `runServer`) without a dedicated `internal/bootstrap` package.

## Development

```bash
cd auth

# Run all tests
task test

# Run tests with coverage report
task test:coverage

# Run tests with race detector
task test:race

# Lint
task lint

# Lint without cache (matches CI)
task lint:no-cache

# Build binary
task build

# Vulnerability check
task vuln

# Format code
task fmt
```

## Integration

All other North Cloud services protect their `/api/v1/*` routes using the shared JWT middleware from `infrastructure/jwt`. The middleware:

1. Reads the `Authorization: Bearer <token>` header (or `?token=` query parameter for SSE endpoints that cannot set custom headers).
2. Parses and validates the token signature against `AUTH_JWT_SECRET`.
3. Rejects expired or malformed tokens with `401 Unauthorized`.
4. Stores the parsed claims in the Gin context for downstream handlers.

The `AUTH_JWT_SECRET` value must be identical across all services. A mismatch causes every token to fail validation with `invalid token`.

Example middleware wiring in another service:

```go
import infraJWT "github.com/north-cloud/infrastructure/jwt"

// Apply to all /api/v1/* routes
v1 := router.Group("/api/v1")
v1.Use(infraJWT.Middleware(cfg.Auth.JWTSecret))
```

Clients store the token and pass it on every subsequent request:

```bash
TOKEN=$(curl -s -X POST http://localhost:8040/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"changeme"}' | jq -r .token)

curl -H "Authorization: Bearer $TOKEN" http://localhost:8050/api/v1/sources
```

Tokens are valid for 24 hours. There is no refresh endpoint; clients must re-authenticate when a token expires.
