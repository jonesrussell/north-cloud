# NC HTTP Proxy Specification

> Last verified: 2026-03-19

## Purpose

Caching HTTP/HTTPS reverse proxy for development and testing. Intercepts outbound requests from the crawler (and other services), caching responses for deterministic replay without network dependencies.

## File Map

| Path | Purpose |
|------|---------|
| `nc-http-proxy/main.go` | Entry point, graceful shutdown |
| `nc-http-proxy/config.go` | Configuration from env vars |
| `nc-http-proxy/proxy.go` | HTTP handler, mode dispatch |
| `nc-http-proxy/cache.go` | Two-tier cache (fixtures + user cache) |
| `nc-http-proxy/admin.go` | Admin API for mode switching |
| `nc-http-proxy/tls.go` | HTTPS MITM certificate management |
| `nc-http-proxy/integration_test.go` | Integration tests |

## Interface Signatures

### Proxy Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| * | `/*` | Proxy all HTTP requests |
| CONNECT | `host:443` | HTTPS tunnel with MITM decryption |

### Admin Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/admin/status` | Proxy mode + cache counts |
| POST | `/admin/mode/{mode}` | Switch mode at runtime |
| GET | `/admin/cache` | List cached domains |
| DELETE | `/admin/cache` | Clear user cache (not fixtures) |
| GET | `/health` | Health check |

## Data Flow

```
HTTP Client Request
  ↓
ServeHTTP (dispatcher)
  ├─ CONNECT (port 443) → handleConnect
  │   ├─ GenerateTLS (per-host leaf cert from CA)
  │   ├─ serveHTTPSConnection (decrypted)
  │   └─ handleHTTP (normal flow)
  └─ Other methods → handleHTTP
      ├─ replay mode: Cache.Lookup → hit: serve / miss: 502
      ├─ record mode: Cache.Lookup → hit: serve / miss: fetch + store
      └─ live mode: proxyLive (bypass cache)
```

## Storage / Schema

### Cache Key Format
```
{METHOD}_{sha256(normalized_url + "\n" + header_hash)[:12]}
```

- URL normalized: lowercase host, strip `www.`, sort query params, remove tracking params (utm_*, fbclid, gclid, ref)
- Headers: SHA-256 of User-Agent + Accept-Language
- Domain directory: dots→dashes (news.example.com → news-example-com)
- Two files per entry: `.json` (metadata) + `.body` (response body)

### Two-Tier Storage

1. **Fixtures** (`PROXY_FIXTURES_DIR`): Read-only, version-controlled responses
2. **User Cache** (`PROXY_CACHE_DIR`): Writable, persists across restarts, clearable via admin API

Lookup order: fixtures first, then user cache.

## Configuration

| Env Var | Default | Purpose |
|---------|---------|---------|
| `PROXY_PORT` | 8055 | Listen port |
| `PROXY_MODE` | replay | Startup mode (replay, record, live) |
| `PROXY_FIXTURES_DIR` | /app/fixtures | Read-only fixture storage |
| `PROXY_CACHE_DIR` | /app/cache | Writable cache storage |
| `PROXY_CERTS_DIR` | /app/certs | CA + leaf cert storage |
| `PROXY_LIVE_TIMEOUT` | 30s | HTTP client timeout |

### Operating Modes

- **Replay**: Serve cached/fixture responses. 502 on miss. No writes.
- **Record**: Serve cached/fixture on hit. Fetch + cache on miss. Writes to cache only.
- **Live**: Pure pass-through. No cache read/write.

## Edge Cases

- **HTTPS MITM**: CA cert persisted in `PROXY_CERTS_DIR/ca.crt`. Leaf certs cached in memory per hostname for process lifetime.
- **Path traversal**: `safePath()` validates domain names before filesystem join.
- **Mode switching**: Thread-safe via RWMutex. No restart required.
- **Tracking param stripping**: utm_*, fbclid, gclid, ref removed from cache keys for deduplication.
- **Docker integration**: Fixtures mounted read-only from `crawler/fixtures/`. Named volume for cache persistence.
