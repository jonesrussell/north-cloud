# HTTP Replay Proxy Design

**Date:** 2026-02-01
**Status:** Draft
**Author:** Russell Jones + Claude

## Problem Statement

During development, the crawler hits production websites on every run. This causes:
- Unnecessary load on external sites
- Risk of getting rate-limited or blocked
- Inconsistent test data (pages change over time)
- Network dependency for local development
- Inability to test edge cases without real pages exhibiting them

## Solution Overview

**nc-http-proxy** is a sidecar HTTP/HTTPS proxy that intercepts crawler traffic and serves cached responses. It provides:

1. **Deterministic replay** - Same responses every time
2. **Safe development** - No outbound HTTP in replay mode
3. **Fixtures overlay** - Version-controlled edge cases override recordings
4. **Zero maintenance** - No CMS or fake site to maintain

## Operating Modes

| Mode | Behavior |
|------|----------|
| `replay` | Serve from fixtures/cache only. Hard fail on miss. **Default.** |
| `record` | Fetch live, store in cache, return response. |
| `live` | Pass through to real sites. Production behavior. |

## Architecture

### Lookup Order (replay/record modes)

1. `crawler/fixtures/<domain>/` - Version-controlled, in repo
2. `~/.northcloud/http-cache/<domain>/` - Recorded responses, per-developer
3. Live fetch (record mode only)

### Cache Key Structure

```
{METHOD}_{sha256(normalized_url + "\n" + header_hash)[:12]}
```

Components:
- **HTTP method** (GET, POST, etc.)
- **Normalized URL**: lowercase host, sorted query params, stripped tracking params (`utm_*`, `fbclid`)
- **Header hash**: SHA256 of `User-Agent + Accept-Language` only

### File Storage Format

```
crawler/fixtures/
  calgary-herald/
    GET_abc123.json      # Request/response metadata
    GET_abc123.body      # Raw response body

~/.northcloud/http-cache/
  calgary-herald/
    GET_def456.json
    GET_def456.body
```

**Metadata file (.json):**

```json
{
  "request": {
    "method": "GET",
    "url": "https://calgaryherald.com/news/article-123",
    "headers": {
      "User-Agent": "NorthCloud-Crawler/1.0",
      "Accept-Language": "en-US"
    }
  },
  "response": {
    "status": 200,
    "headers": {
      "Content-Type": "text/html; charset=utf-8"
    },
    "was_compressed": false
  },
  "recorded_at": "2026-02-01T14:30:00Z",
  "cache_key": "GET_abc123"
}
```

**Body file (.body):** Raw response bytes, exactly as received.

### HTTPS Handling

The proxy uses CONNECT tunnel with TLS termination:

1. Crawler sends `CONNECT example.com:443`
2. Proxy responds `200 Connection Established`
3. Proxy establishes TLS with origin (record mode) or serves cached response
4. Proxy terminates TLS with crawler using self-signed certificate
5. Crawler trusts the proxy's certificate (configured once)

No system-wide CA installation required.

## Proxy Service

### Configuration (Environment Variables)

| Variable | Default | Description |
|----------|---------|-------------|
| `PROXY_PORT` | `8055` | Listen port |
| `PROXY_MODE` | `replay` | Initial mode |
| `PROXY_FIXTURES_DIR` | `/app/fixtures` | Path to fixtures |
| `PROXY_CACHE_DIR` | `/app/cache` | Path to recorded cache |
| `PROXY_CERT_FILE` | `/app/certs/proxy.crt` | TLS cert |
| `PROXY_KEY_FILE` | `/app/certs/proxy.key` | TLS key |
| `PROXY_LIVE_TIMEOUT` | `30s` | Timeout for live requests |

### Admin API

```
POST /admin/mode/replay     # Switch to replay mode
POST /admin/mode/record     # Switch to record mode
POST /admin/mode/live       # Switch to live mode

GET  /admin/status          # Current mode, cache stats

GET    /admin/cache              # List all cached domains
GET    /admin/cache/{domain}     # List entries for domain
DELETE /admin/cache              # Clear all cache
DELETE /admin/cache/{domain}     # Clear domain cache
```

### Status Response

```json
{
  "mode": "replay",
  "fixtures_count": 42,
  "cache_count": 128,
  "domains": ["calgary-herald", "example-com"]
}
```

## Docker Integration

### docker-compose.dev.yml

```yaml
nc-http-proxy:
  build:
    context: ./nc-http-proxy
    dockerfile: Dockerfile
  container_name: north-cloud-http-proxy
  ports:
    - "8055:8055"
  volumes:
    - ./crawler/fixtures:/app/fixtures:ro
    - ~/.northcloud/http-cache:/app/cache
    - ./nc-http-proxy/certs:/app/certs:ro
  environment:
    - PROXY_MODE=replay
    - PROXY_PORT=8055
  networks:
    - north-cloud-network
  healthcheck:
    test: ["CMD", "curl", "-sf", "http://localhost:8055/admin/status"]
    interval: 10s
    timeout: 5s
    retries: 3
```

### Crawler Service Changes

```yaml
crawler:
  environment:
    - HTTP_PROXY=http://nc-http-proxy:8055
    - HTTPS_PROXY=http://nc-http-proxy:8055
  volumes:
    - ./nc-http-proxy/certs/proxy.crt:/usr/local/share/ca-certificates/nc-proxy.crt:ro
  depends_on:
    nc-http-proxy:
      condition: service_healthy
```

Crawler entrypoint must run `update-ca-certificates` before starting.

### Certificate Generation

```bash
# nc-http-proxy/certs/generate.sh
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout proxy.key -out proxy.crt \
  -days 3650 \
  -subj "/CN=nc-http-proxy" \
  -addext "subjectAltName=DNS:nc-http-proxy,DNS:localhost"
```

Certificates are committed to the repo (dev-only, self-signed).

## Taskfile Commands

```yaml
proxy:up:
  desc: Start the HTTP proxy
  cmds:
    - docker compose -f docker-compose.base.yml -f docker-compose.dev.yml up -d nc-http-proxy

proxy:status:
  desc: Show proxy mode and cache stats
  cmds:
    - curl -sf http://localhost:8055/admin/status | jq

proxy:replay:
  desc: Switch proxy to replay mode
  cmds:
    - curl -sf -X POST http://localhost:8055/admin/mode/replay
    - echo "Proxy switched to replay mode"

proxy:record:
  desc: Switch proxy to record mode
  cmds:
    - curl -sf -X POST http://localhost:8055/admin/mode/record
    - echo "Proxy switched to record mode"

proxy:live:
  desc: Switch proxy to live mode (use with caution)
  cmds:
    - curl -sf -X POST http://localhost:8055/admin/mode/live
    - echo "Proxy switched to live mode - requests will hit production"

proxy:list:
  desc: List all cached domains
  cmds:
    - curl -sf http://localhost:8055/admin/cache | jq

proxy:list-domain:
  desc: List cached entries for a domain
  cmds:
    - curl -sf http://localhost:8055/admin/cache/{{.CLI_ARGS}} | jq

proxy:clear:
  desc: Clear all recorded cache (not fixtures)
  cmds:
    - curl -sf -X DELETE http://localhost:8055/admin/cache
    - echo "Cache cleared"

proxy:clear-domain:
  desc: Clear cache for a specific domain
  cmds:
    - curl -sf -X DELETE http://localhost:8055/admin/cache/{{.CLI_ARGS}}
    - echo "Cache cleared for {{.CLI_ARGS}}"
```

## Error Handling

### Cache Miss in Replay Mode

```http
HTTP/1.1 502 Bad Gateway
Content-Type: application/json
X-Proxy-Mode: replay
X-Proxy-Cache-Miss: true
X-Proxy-Source: none

{
  "error": "cache_miss",
  "mode": "replay",
  "url": "https://calgaryherald.com/news/article-999",
  "cache_key": "GET_abc123",
  "message": "No fixture or recording found. Run 'task proxy:record' to capture this URL."
}
```

### Malformed Cache Files

If `.json` exists but `.body` is missing (or vice versa):
- Log warning with full path
- Treat as cache miss
- Never silently use partial data

### Domain Normalization

- Lowercase host
- Strip `www.` prefix
- Replace dots with dashes in directory names
- Example: `www.CalgaryHerald.com` → `calgaryherald-com`

### Concurrent Requests

In record mode, if multiple requests hit the same URL:
- First request populates the cache
- Subsequent requests wait (per-cache-key mutex)
- All return the same cached result

### Timeout Handling

Live requests timeout after 30 seconds (configurable). Response:

```json
{
  "error": "timeout",
  "url": "https://example.com/slow-page",
  "timeout_seconds": 30
}
```

## Developer Workflow

### Initial Setup (One-Time)

```bash
mkdir -p ~/.northcloud/http-cache
task docker:dev:up
```

### Daily Development

```bash
# Start dev environment (proxy defaults to replay mode)
task docker:dev:up

# Run crawler - uses cached responses, no live traffic
task crawler:run

# Need to update recordings?
task proxy:record
# Run crawl...
task proxy:replay
```

### Creating Fixtures for Edge Cases

```bash
# 1. Record the page
task proxy:record
# Crawl the URL...

# 2. Find the recording
task proxy:list-domain -- calgary-herald

# 3. Copy to fixtures
cp ~/.northcloud/http-cache/calgary-herald/GET_abc123.* \
   crawler/fixtures/calgary-herald/

# 4. Edit to create edge case
vim crawler/fixtures/calgary-herald/GET_abc123.body

# 5. Fixtures take priority
task proxy:replay
```

### CI Pipeline

```yaml
- name: Run crawler tests
  env:
    HTTP_PROXY: http://nc-http-proxy:8055
    HTTPS_PROXY: http://nc-http-proxy:8055
  run: |
    task proxy:status
    task test:crawler
```

## Project Structure

```
nc-http-proxy/
├── main.go              # Entry point (~100 lines)
├── proxy.go             # Core proxy logic (~300 lines)
├── cache.go             # Cache lookup/storage (~150 lines)
├── admin.go             # Admin API handlers (~100 lines)
├── Dockerfile
├── Taskfile.yml
├── certs/
│   ├── generate.sh
│   ├── proxy.crt
│   └── proxy.key
└── README.md
```

**Dependencies:** Standard library only (`net/http`, `crypto/tls`, `sync`)

**Estimated scope:** ~650 lines of Go

## Implementation Order

1. **Basic HTTP proxy** - Forward requests, no caching
2. **HTTPS CONNECT handling** - TLS termination with cert
3. **Cache storage** - Write/read `.json` + `.body` files
4. **Cache lookup** - Fixtures overlay, key generation
5. **Mode switching** - Admin API endpoints
6. **Replay mode** - Hard fail on miss
7. **Docker integration** - Compose config, volume mounts
8. **Taskfile commands** - Developer ergonomics

## Success Criteria

- [ ] `task proxy:replay` serves cached responses with zero network calls
- [ ] `task proxy:record` captures real responses and stores them correctly
- [ ] Cache miss in replay mode fails immediately with clear error
- [ ] Fixtures in repo override recorded cache
- [ ] Full crawler pipeline works with cached data
- [ ] CI runs deterministically using fixtures only
