# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with the auth service.

## Quick Reference

```bash
# Development
task dev              # Start with hot reload
task test             # Run tests
task lint             # Run linter

# API (port 8040)
curl -X POST http://localhost:8040/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "secret"}'
```

## Architecture

```
auth/
├── main.go
└── internal/
    ├── api/
    │   ├── server.go        # Gin server setup
    │   └── auth_handler.go  # Login endpoint
    ├── auth/
    │   └── jwt.go           # JWT generation/validation
    └── config/
        └── config.go        # Configuration
```

## Authentication Flow

```
Client → POST /api/v1/auth/login → Validate credentials → Generate JWT → Return token
                                          ↓
                      Uses AUTH_USERNAME + AUTH_PASSWORD env vars
```

**JWT Token**:
- Expires after 24 hours
- Contains: `username`, `exp`, `iat`
- Signed with `AUTH_JWT_SECRET`

## API Endpoints

**Public (no auth required)**:
- `GET /health` - Health check
- `POST /api/v1/auth/login` - Get JWT token

**Request Format**:
```json
{
  "username": "admin",
  "password": "secret"
}
```

**Success Response**:
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_at": "2025-12-29T15:30:45Z"
}
```

## Configuration

| Env Variable | Description | Required |
|-------------|-------------|----------|
| `AUTH_USERNAME` | Login username | Yes |
| `AUTH_PASSWORD` | Login password | Yes |
| `AUTH_JWT_SECRET` | JWT signing secret | Yes |
| `AUTH_PORT` | Server port (default: 8040) | No |

**Generate JWT Secret**:
```bash
openssl rand -hex 32
```

## Common Gotchas

1. **Single user only**: Auth service supports only one username/password pair via env vars.

2. **JWT secret must match across services**: All services validating JWTs must use the same `AUTH_JWT_SECRET`.

3. **No refresh tokens**: Tokens expire after 24h; client must re-authenticate.

4. **Health endpoint is public**: `/health` doesn't require authentication.

## JWT Validation in Other Services

Other services validate JWTs using middleware:

```go
// Middleware checks Authorization: Bearer <token> header
func JWTAuthMiddleware(secret string) gin.HandlerFunc {
    // Validates signature and expiration
}
```

## Testing

```bash
# Run tests
task test

# Test login manually
curl -X POST http://localhost:8040/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "test123"}'
```
