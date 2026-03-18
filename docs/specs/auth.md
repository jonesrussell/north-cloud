# Auth Service Spec

> Last verified: 2026-03-18

## Overview

Single-user authentication service. Validates credentials against one username/password pair (from environment variables) and issues HS256-signed JWT tokens. No user database.

---

## File Map

```
auth/
  main.go                          # Entry point: profiling → config → logger → server
  config.yml                       # Default configuration
  internal/
    api/
      server.go                    # Gin server builder, route registration
      auth_handler.go              # Login handler: credential validation, JWT response
    auth/
      jwt.go                       # JWTManager: GenerateToken, ValidateToken
    config/
      config.go                    # Config struct, setDefaults, Validate, GetJWTConfig
```

---

## API Reference

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/health` | None | Returns 200 OK |
| POST | `/api/v1/auth/login` | None | Validate credentials, return JWT |

**Login request**:
```json
{"username": "admin", "password": "secret"}
```

**Login response (200)**:
```json
{"token": "eyJhbGciOiJIUzI1NiIs..."}
```

**Error responses**: `400` (bad/missing fields), `401` (wrong credentials), `500` (token generation failure).

---

## Data Model

**JWT Claims** (HS256):
- `sub`: always `"dashboard"`
- `iat`: issued-at Unix timestamp
- `nbf`: not-before (same as `iat`)
- `exp`: issued-at + expiration duration (default 24h)

No database. No refresh tokens.

---

## Configuration

| Variable | yaml key | Default | Required | Description |
|----------|----------|---------|----------|-------------|
| `AUTH_USERNAME` | `auth.username` | `admin` | Yes | Login username |
| `AUTH_PASSWORD` | `auth.password` | `admin` | Yes | Login password |
| `AUTH_JWT_SECRET` | `auth.jwt_secret` | `change-me-in-production` | Yes (prod) | HS256 signing secret |
| `AUTH_PORT` | `service.port` | `8040` | No | HTTP listen port |
| `APP_DEBUG` | `service.debug` | `false` | No | Debug mode (relaxes JWT secret validation) |
| `LOG_LEVEL` | `logging.level` | `info` | No | Log level |
| `LOG_FORMAT` | `logging.format` | `json` | No | Log format |

`jwt_expiration` is only configurable via `config.yml` (Go duration string, e.g., `"24h"`).

---

## Known Constraints

- **Single user only**: one username/password pair from env vars. No user management API.
- **JWT secret must match across all services**: every service using `infraJWT.Middleware` reads `AUTH_JWT_SECRET`. Mismatch causes 401 on all requests.
- **No refresh tokens**: clients re-authenticate on expiry (default 24h).
- **Default secret rejected in production**: if `APP_DEBUG=false` and `AUTH_JWT_SECRET` is empty or `"change-me-in-production"`, the service exits at startup.
- **Health endpoint always public**: `/health` bypasses JWT validation.
- **Bootstrap pattern**: simple (helper functions in `main.go`, no `internal/bootstrap/` package).
