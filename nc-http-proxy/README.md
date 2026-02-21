# nc-http-proxy

HTTP/HTTPS replay proxy for deterministic crawler development and testing.

## Overview

nc-http-proxy is a sidecar proxy that intercepts HTTP/HTTPS requests and can either:
- **Replay** cached responses from fixtures or user cache (deterministic, offline-safe testing)
- **Record** live responses and save them to the user cache (fixture creation)
- **Live** pass-through requests to real servers without caching

It integrates with the North Cloud crawler as a development tool, enabling repeatable crawl runs against version-controlled fixtures rather than live websites.

## Features

- Three operating modes switchable at runtime via Admin API
- HTTPS support via MITM with an auto-generated local CA
- Deterministic cache keys: SHA-256 of normalized URL + relevant request headers
- Two-tier storage: version-controlled fixtures (read-only) and a user-local cache
- Tracking parameter stripping (`utm_*`, `fbclid`, `gclid`, `ref`)
- Admin API for mode switching, cache inspection, and cache clearing
- Response headers expose `X-Proxy-Mode` and `X-Proxy-Source` for debugging

## Quick Start

```bash
# Start the proxy container
task proxy:up

# Check mode and cache stats
task proxy:status

# View logs
task proxy:logs
```

By default the proxy starts in **replay mode**. Configure the crawler to route through it:

```yaml
# crawler/config.yml
http_proxy: http://nc-http-proxy:8055
```

In Docker Compose, the proxy is reachable at `nc-http-proxy:8055` on the internal network.
From the host, it is available at `http://localhost:8055` (or `$PROXY_PORT`).

## Modes

### Replay Mode (Default)

Serves responses from fixtures or user cache only. Returns HTTP 502 on a cache miss — it never contacts real servers.

```bash
task proxy:mode:replay
```

Cache lookup order on every request:

1. Fixtures directory (`crawler/fixtures/{domain}/`) — version-controlled, read-only
2. User cache (`~/.northcloud/http-cache/{domain}/`) — local only, writable

### Record Mode

Forwards requests to real servers and saves responses to the user cache. Existing fixtures are still served first (to avoid unnecessary network calls). New responses land in `~/.northcloud/http-cache/`.

```bash
task proxy:mode:record
```

Use record mode to:
- Capture responses that will become new fixtures
- Refresh stale cached responses
- Debug live server behavior

### Live Mode

Pure pass-through proxy — no caching, no fixture lookup. Useful when you need the crawler to behave exactly as in production.

```bash
task proxy:mode:live
```

**Note**: Mode is stored in memory. Restarting the container resets to the `PROXY_MODE` environment variable (default: `replay`).

## Admin API

All admin endpoints are available at `http://localhost:${PROXY_PORT:-8055}`.

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check — returns `200 OK` |
| `/admin/status` | GET | Current mode and cache/fixture counts |
| `/admin/mode/{mode}` | POST | Switch mode: `replay`, `record`, or `live` |
| `/admin/cache` | GET | List all cached domains |
| `/admin/cache/{domain}` | GET | List cache keys for a specific domain |
| `/admin/cache` | DELETE | Clear all user cache (fixtures are never deleted) |
| `/admin/cache/{domain}` | DELETE | Clear user cache for a specific domain |

Example status response:

```json
{
  "mode": "replay",
  "fixtures_count": 6,
  "cache_count": 0,
  "domains": ["fixture-news-site-com"]
}
```

## Configuration

All configuration is via environment variables. The container exposes port `8055` by default.

| Variable | Default | Description |
|----------|---------|-------------|
| `PROXY_PORT` | `8055` | Proxy listen port |
| `PROXY_MODE` | `replay` | Initial mode at startup (`replay`, `record`, `live`) |
| `PROXY_FIXTURES_DIR` | `/app/fixtures` | Path to version-controlled fixtures (read-only) |
| `PROXY_CACHE_DIR` | `/app/cache` | Path to user-local cache (writable) |
| `PROXY_CERTS_DIR` | `/app/certs` | Path to generated CA and leaf TLS certificates |
| `PROXY_LIVE_TIMEOUT` | `30s` | HTTP client timeout for live/record requests |

In `docker-compose.dev.yml`, the volumes are wired as:

```yaml
volumes:
  - ./crawler/fixtures:/app/fixtures:ro   # fixtures are read-only
  - ${HOME}/.northcloud/http-cache:/app/cache
```

## Cache Key Generation

Cache keys are deterministic 12-character hex strings derived from a SHA-256 hash:

```
{METHOD}_{sha256(normalized_url + "\n" + header_hash)[:12]}
```

**URL normalization**:
- Lowercase host
- Sort query parameters alphabetically
- Strip tracking parameters: `utm_source`, `utm_medium`, `utm_campaign`, `utm_term`, `utm_content`, `fbclid`, `gclid`, `ref`
- Strip `www.` prefix from domain

**Header hash** (also SHA-256): `User-Agent` + `Accept-Language`

**Domain directory name**: lowercase, `www.` stripped, dots replaced with dashes.
Example: `example.com` → `example-com`, `www.news.org` → `news-org`

## Directory Structure

Each cached entry consists of two files in a domain subdirectory:

```
crawler/fixtures/                      # Version-controlled (read-only in Docker)
  fixture-news-site-com/
    GET_c910c16364f6.json              # Metadata (request, response headers, timestamp)
    GET_c910c16364f6.body              # Raw response body

~/.northcloud/http-cache/              # User-local cache (writable)
  example-com/
    GET_abc123def456.json
    GET_abc123def456.body
```

Metadata file format (`*.json`):

```json
{
  "request": {
    "method": "GET",
    "url": "https://example.com/article/slug",
    "headers": { "User-Agent": "..." }
  },
  "response": {
    "status": 200,
    "headers": { "Content-Type": "text/html; charset=utf-8" },
    "was_compressed": false
  },
  "recorded_at": "2026-02-06T05:51:28Z",
  "cache_key": "GET_c910c16364f6"
}
```

## Architecture

```
┌─────────────┐     ┌───────────────┐     ┌──────────────┐
│   Crawler   │────▶│ nc-http-proxy │────▶│ Real Server  │
└─────────────┘     └───────────────┘     └──────────────┘
                           │
                    ┌──────┴──────┐
                    │             │
              ┌─────▼─────┐ ┌────▼──────┐
              │ Fixtures  │ │ User Cache│
              │ (git, ro) │ │ (local)   │
              └───────────┘ └───────────┘
```

HTTPS requests use MITM (Man-in-the-Middle) via a locally generated CA:
1. The crawler sends a `CONNECT` request for the target host
2. The proxy responds `200 Connection Established`
3. The proxy upgrades the connection to TLS using a per-host leaf certificate signed by its local CA
4. HTTP requests over the TLS tunnel are processed through the normal replay/record/live pipeline

The CA certificate is generated on first startup at `PROXY_CERTS_DIR/ca.crt`. To proxy HTTPS traffic through a non-Docker client (e.g., a local browser or test tool), trust this CA in your system or browser certificate store.

## Development

```bash
# Run tests
task test:nc-http-proxy

# Run with race detector
task test:race:nc-http-proxy

# Run linter
task lint:nc-http-proxy

# Build binary locally
cd nc-http-proxy && task build

# Run locally (outside Docker)
cd nc-http-proxy && task run
```

Or using the service-local Taskfile directly:

```bash
cd nc-http-proxy
task build          # Build binary to bin/nc-http-proxy
task test           # go test -v ./...
task test:race      # go test -race ./...
task test:coverage  # Coverage report → coverage.html
task lint           # go fmt + go vet + golangci-lint
task lint:no-cache  # Same as lint, but clears golangci-lint cache first (matches CI)
task run            # go run .
```

## Workflow: Creating Fixtures

1. Start the proxy in record mode:
   ```bash
   task proxy:mode:record
   ```

2. Configure the crawler to use the proxy (if not already):
   ```yaml
   # crawler/config.yml
   http_proxy: http://nc-http-proxy:8055
   ```

3. Run the crawler against the target source. Responses are written to `~/.northcloud/http-cache/`.

4. Inspect what was recorded:
   ```bash
   task proxy:list
   ls ~/.northcloud/http-cache/
   ```

5. Copy the desired responses into version-controlled fixtures:
   ```bash
   cp ~/.northcloud/http-cache/example-com/*.json crawler/fixtures/example-com/
   cp ~/.northcloud/http-cache/example-com/*.body crawler/fixtures/example-com/
   ```

6. Switch back to replay mode and verify the crawler works against fixtures:
   ```bash
   task proxy:mode:replay
   ```

7. Commit the fixtures:
   ```bash
   git add crawler/fixtures/
   git commit -m "Add fixtures for example.com"
   ```

## Troubleshooting

### Cache Miss in Replay Mode

```json
{
  "error": "cache_miss",
  "mode": "replay",
  "url": "https://example.com/page",
  "cache_key": "GET_abc123def456",
  "message": "No fixture or recording found. Run 'task proxy:record' to capture this URL."
}
```

1. Check if fixtures exist for the domain:
   ```bash
   task proxy:list
   ```

2. Switch to record mode, re-run the crawler, then switch back:
   ```bash
   task proxy:mode:record
   # trigger the crawl
   task proxy:mode:replay
   ```

### Stale Fixtures

If real server responses have changed, re-record them:

```bash
task proxy:clear          # Clear user cache (fixtures untouched)
task proxy:mode:record    # Switch to record
# Run the crawler
# Copy new recordings to crawler/fixtures/ as needed
task proxy:mode:replay    # Switch back
```

### HTTPS Proxying Fails

The proxy generates a local CA on first startup. If the crawler rejects the CA:
- Verify the container has a persistent `certs` volume so the CA survives restarts
- For CLI tools, trust the CA: `PROXY_CERTS_DIR/ca.crt`

### Mode Reverts After Restart

Mode is in-memory only. To make a mode persistent, set `PROXY_MODE` in `.env` or `docker-compose.dev.yml` before starting the container.
