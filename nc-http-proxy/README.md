# nc-http-proxy

HTTP/HTTPS replay proxy for deterministic crawler development and testing.

## Overview

nc-http-proxy is a sidecar proxy that intercepts HTTP requests and can either:
- **Replay** cached responses (deterministic testing)
- **Record** live responses to cache (fixture creation)
- **Live** pass-through (production-like behavior)

## Quick Start

```bash
# Start the proxy
task proxy:up

# Check status
task proxy:status

# View logs
task proxy:logs
```

## Modes

### Replay Mode (Default)

Returns cached responses only. Fails hard if no cache entry found.

```bash
task proxy:mode:replay
```

**Lookup order:**
1. Fixtures directory (`crawler/fixtures/{domain}/`) - version controlled
2. User cache (`~/.northcloud/http-cache/{domain}/`) - local only

### Record Mode

Forwards requests to real servers, caches responses.

```bash
task proxy:mode:record
```

Use this to:
- Create new fixtures
- Update existing fixtures
- Debug live server responses

### Live Mode

Pass-through proxy, no caching.

```bash
task proxy:mode:live
```

## Admin API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/admin/status` | GET | Mode and cache stats |
| `/admin/mode/{mode}` | POST | Switch mode (replay/record/live) |
| `/admin/cache` | GET | List cached domains |
| `/admin/cache/{domain}` | GET | List entries for domain |
| `/admin/cache` | DELETE | Clear all cache |
| `/admin/cache/{domain}` | DELETE | Clear domain cache |

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `PROXY_PORT` | 8055 | Proxy listen port |
| `PROXY_MODE` | replay | Initial mode |
| `PROXY_FIXTURES_DIR` | /app/fixtures | Fixtures directory |
| `PROXY_CACHE_DIR` | /app/cache | Cache directory |
| `PROXY_LIVE_TIMEOUT` | 30s | Timeout for live requests |

## Cache Key Generation

Cache keys are deterministic SHA-256 hashes:

```
{METHOD}_{sha256(normalized_url)[:12]}
```

URL normalization:
- Lowercase domain
- Sort query parameters alphabetically
- Strip tracking parameters (utm_*, fbclid, etc.)

## Directory Structure

```
crawler/fixtures/           # Version-controlled fixtures
  example-com/
    GET_abc123def456.json
~/.northcloud/http-cache/   # User-local cache
  example-com/
    GET_789xyz012abc.json
```

## Development

```bash
# Run tests
task test:nc-http-proxy

# Run linter
task lint:nc-http-proxy

# Build locally
cd nc-http-proxy && task build

# Run locally (outside Docker)
cd nc-http-proxy && task run
```

## Workflow: Creating Fixtures

1. Start proxy in record mode:
   ```bash
   task proxy:mode:record
   ```

2. Configure crawler to use proxy:
   ```yaml
   # In crawler config
   http_proxy: http://localhost:8055
   ```

3. Run crawler:
   ```bash
   task docker:dev:up:crawler
   ```

4. Check recorded responses:
   ```bash
   task proxy:list
   ls ~/.northcloud/http-cache/
   ```

5. Copy desired responses to fixtures:
   ```bash
   cp ~/.northcloud/http-cache/example-com/*.json crawler/fixtures/example-com/
   ```

6. Switch to replay mode:
   ```bash
   task proxy:mode:replay
   ```

7. Commit fixtures:
   ```bash
   git add crawler/fixtures/
   git commit -m "Add fixtures for example.com"
   ```

## Integration with Crawler

The crawler can be configured to route requests through the proxy:

```yaml
# crawler/config.yml
http_proxy: http://nc-http-proxy:8055
```

In Docker Compose, the proxy is accessible as `nc-http-proxy` on the internal network.

## Troubleshooting

### Cache Miss in Replay Mode

```
Error: cache entry not found
```

1. Check if fixture exists:
   ```bash
   task proxy:list
   ```

2. Record the missing response:
   ```bash
   task proxy:mode:record
   # Make request
   task proxy:mode:replay
   ```

### Stale Fixtures

If responses have changed, re-record:

```bash
task proxy:clear         # Clear user cache
task proxy:mode:record   # Record fresh
# Run crawler
# Copy new fixtures
```

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌──────────────┐
│   Crawler   │────▶│ nc-http-proxy │────▶│ Real Server  │
└─────────────┘     └──────────────┘     └──────────────┘
                           │
                    ┌──────┴──────┐
                    │             │
              ┌─────▼─────┐ ┌─────▼─────┐
              │ Fixtures  │ │ User Cache│
              │ (git)     │ │ (local)   │
              └───────────┘ └───────────┘
```
