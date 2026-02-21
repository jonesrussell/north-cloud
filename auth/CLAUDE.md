# Auth — Developer Guide

This file provides guidance to Claude Code (claude.ai/code) when working with the auth service.

## Quick Reference

```bash
# Daily development
task test             # Run all tests
task test:coverage    # Tests + HTML coverage report
task test:race        # Tests with race detector
task lint             # fmt + vet + golangci-lint
task lint:no-cache    # Cache-clean lint (matches CI exactly)
task build            # Compile to bin/auth
task run              # go run main.go
task fmt              # gofmt + goimports
task vuln             # govulncheck

# API (port 8040)
curl -X POST http://localhost:8040/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "secret"}'

# Health check (no auth)
curl http://localhost:8040/health
```

## Architecture

```
auth/
├── main.go                    — Entry point: profiling, config, logger, server
├── config.yml                 — Default configuration (override with env vars)
├── Taskfile.yml               — All build/test/lint/benchmark tasks
├── .air.toml                  — Hot reload (Air) configuration
└── internal/
    ├── api/
    │   ├── server.go          — Gin server builder, route registration, timeouts
    │   └── auth_handler.go    — Login handler: credential validation, JWT response
    ├── auth/
    │   └── jwt.go             — JWTManager: GenerateToken, ValidateToken
    └── config/
        └── config.go          — Config struct, setDefaults, Validate, GetJWTConfig
```

**Bootstrap pattern**: simple — helper functions in `main.go` (`loadConfig`, `createLogger`, `runServer`) without a dedicated `internal/bootstrap` package. This matches the pattern used by other simple services (auth, search).

**Phase order**: pprof profiling → config load+validate → logger → server (routes) → blocking `srv.Run()`.

## Key Concepts

**Single-user model**: Auth supports exactly one username/password pair, supplied via environment variables (or `config.yml`). There is no user database.

**JWT format**: HS256-signed tokens. Claims:
- `sub`: always `"dashboard"`
- `iat`: issued-at Unix timestamp
- `nbf`: not-before (same as `iat`)
- `exp`: issued-at + expiration duration (default 24h)

**Expiration**: Configurable via `auth.jwt_expiration` in `config.yml` (Go duration string, e.g. `"24h"`). Defaults to 24 hours. There is no refresh mechanism; clients must re-authenticate on expiry.

**Shared secret**: `AUTH_JWT_SECRET` must be identical on every service that validates JWTs. The secret is validated at startup: in non-debug mode, an empty or default value (`"change-me-in-production"`) causes the service to refuse to start.

**Debug mode**: When `APP_DEBUG=true` (or `service.debug: true` in config), the JWT secret validation is relaxed, allowing the default placeholder secret. Never use debug mode in production.

**Infrastructure Gin server**: Auth uses `infragin.NewServerBuilder` from `github.com/north-cloud/infrastructure/gin`. This provides consistent server configuration (timeouts, health endpoint, graceful shutdown) across all services. Auth does NOT apply the JWT middleware to its own routes — it is the issuer.

## API Reference

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/health` | None | Returns 200 OK |
| POST | `/api/v1/auth/login` | None | Validate credentials, return JWT |

**Login request** (`username` and `password` are both required):
```json
{"username": "admin", "password": "secret"}
```

**Login response (200)**:
```json
{"token": "eyJhbGciOiJIUzI1NiIs..."}
```

**Error responses**: `400` (bad/missing fields), `401` (wrong credentials), `500` (token generation failure).

## Configuration

| Variable | yaml key | Default | Required | Description |
|----------|----------|---------|----------|-------------|
| `AUTH_USERNAME` | `auth.username` | `admin` | Yes | Login username |
| `AUTH_PASSWORD` | `auth.password` | `admin` | Yes | Login password |
| `AUTH_JWT_SECRET` | `auth.jwt_secret` | `change-me-in-production` | Yes (prod) | HS256 signing secret |
| `AUTH_PORT` | `service.port` | `8040` | No | HTTP listen port |
| `APP_DEBUG` | `service.debug` | `false` | No | Debug mode — relaxes JWT secret validation |
| `LOG_LEVEL` | `logging.level` | `info` | No | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `logging.format` | `json` | No | `json` or `console` |

`jwt_expiration` is only configurable via `config.yml` (no env var); set it as a Go duration string (e.g., `"24h"`, `"12h"`).

Generate a secure JWT secret:
```bash
openssl rand -hex 32
```

## Common Gotchas

1. **Single user only**: The service validates credentials against exactly one username/password pair. There is no user management API or database. To change credentials, update environment variables and restart the service.

2. **JWT secret must match across all services**: Every service that protects routes with `infraJWT.Middleware` reads `AUTH_JWT_SECRET`. A mismatch causes `401 invalid token` on every request, with no clear error in the protected service's logs beyond "invalid token".

3. **No refresh tokens**: Tokens expire after the configured duration (default 24h). Clients must POST to `/api/v1/auth/login` again. There is no `/refresh` endpoint.

4. **Default secret is rejected in production**: If `APP_DEBUG=false` (the default) and `AUTH_JWT_SECRET` is empty or equals `"change-me-in-production"`, the service exits immediately at startup with a `ValidationError`. Set a real secret before deploying.

5. **Health endpoint is always public**: `/health` bypasses JWT validation in the infrastructure middleware. Do not place sensitive information in the health response.

6. **Special characters in passwords break shell escaping**: When testing with `curl` in shell, passwords containing `!` or other shell-special characters will be mis-interpreted. Use a JSON file instead:
   ```bash
   cat > /tmp/login.json << 'EOF'
   {"username":"admin","password":"f00Bar123!"}
   EOF
   curl -s -X POST http://localhost:8040/api/v1/auth/login \
     -H "Content-Type: application/json" -d @/tmp/login.json
   ```

7. **`task dev` is not defined** in the auth Taskfile — use `task run` for `go run main.go` or configure Air directly (`air`) for hot reload.

## Testing

```bash
# All tests (verbose)
task test

# Unit tests only (internal packages)
task test:unit

# With coverage HTML report
task test:coverage
# Opens coverage.html (or: open coverage.html on macOS)

# With race detector
task test:race
```

**What is tested**:
- `internal/auth/jwt_test.go`: `NewJWTManager`, `GenerateToken`, `ValidateToken` (success, expired, wrong secret, malformed tokens)
- `internal/api/auth_handler_test.go`: `Login` handler (success, invalid credentials × 3 combinations, malformed requests × 5 cases)

All test helper functions call `t.Helper()` at the top as required by the linter.

## Code Patterns

### JWT Validation in Other Services

Other services validate tokens using the shared middleware from `infrastructure/jwt`:

```go
import infraJWT "github.com/north-cloud/infrastructure/jwt"

// Register middleware on protected route group
v1 := router.Group("/api/v1")
v1.Use(infraJWT.Middleware(cfg.Auth.JWTSecret))
```

The middleware (`infrastructure/jwt/middleware.go`):
- Reads `Authorization: Bearer <token>` header
- Falls back to `?token=` query parameter for SSE endpoints (EventSource cannot set custom headers)
- Skips `/health` and `/health/*` paths unconditionally
- Returns `401` for missing, expired, or invalid tokens
- Stores parsed `*jwt.Claims` in Gin context under key `"claims"`

To extract claims in a handler:

```go
claims, ok := infraJWT.GetClaims(c)
if !ok {
    c.JSON(http.StatusUnauthorized, gin.H{"error": "missing claims"})
    return
}
// claims.Sub == "dashboard"
```
