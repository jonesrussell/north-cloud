# nc-http-proxy — Developer Guide

## Quick Reference

```bash
# Start / stop / restart
task proxy:up
task proxy:down
task proxy:restart

# Status and logs
task proxy:status       # GET /admin/status — mode + cache counts
task proxy:logs         # Follow container logs

# Switch modes at runtime (no restart needed)
task proxy:mode:replay  # Serve fixtures/cache only; 502 on miss
task proxy:mode:record  # Live fetch + cache responses
task proxy:mode:live    # Pure pass-through, no caching

# Inspect and clear the user cache
task proxy:list         # List cached domains
task proxy:clear        # Delete all entries in ~/.northcloud/http-cache/ (fixtures untouched)

# Build / rebuild the container image
task proxy:build

# Development (from nc-http-proxy/ directory)
task build              # Compile to bin/nc-http-proxy
task test               # go test -v ./...
task test:race          # go test -race ./...
task test:coverage      # Coverage report → coverage.html
task lint               # go fmt + go vet + golangci-lint
task lint:no-cache      # Same, with golangci-lint cache cleared (matches CI)
task run                # go run . (local, outside Docker)

# Root-level shortcuts
task test:nc-http-proxy
task test:race:nc-http-proxy
task lint:nc-http-proxy
task lint:nc-http-proxy:force
```

## Architecture

```
nc-http-proxy/
├── main.go           # Entry: LoadConfig → NewProxy → NewAdminHandler → http.Server
├── config.go         # Config struct + LoadConfig() from env vars; Mode type + IsValid()
├── proxy.go          # Proxy struct: ServeHTTP, handleHTTP, handleConnect (HTTPS MITM)
├── cache.go          # Cache struct: Lookup (fixtures-first), Store (cache only), Stats
├── cache_entry.go    # CacheEntry + CacheEntryMetadata types; MetadataPath/BodyPath helpers
├── cache_key.go      # GenerateCacheKey, NormalizeURL, NormalizeDomain
├── admin.go          # AdminHandler: mode switch, cache list/clear, path traversal guard
├── tls.go            # CertManager: auto-generate CA + per-host leaf certs (MITM)
├── *_test.go         # Unit and integration tests (httptest-based, no external deps)
└── Dockerfile        # Multi-stage: golang:1.26 builder → alpine:3.19 runtime
```

**Package**: single `main` package — no internal subdirectories. All types and functions are in the same package, so there are no import cycles to manage.

**Request flow (HTTP)**:
```
ServeHTTP
  └─ CONNECT? → handleConnect (TLS MITM) → serveHTTPSConnection → handleHTTP
  └─ other   → handleHTTP
       ├─ replay/record: Cache.Lookup (fixtures first, then cache)
       │    hit  → serveCachedResponse
       │    miss + replay  → serveCacheMissError (HTTP 502)
       │    miss + record  → fetchAndCache → serveCachedResponse
       └─ live → proxyLive (no cache read/write)
```

## Key Concepts

### Three Modes

| Mode | On cache hit | On cache miss | Writes cache |
|------|-------------|---------------|--------------|
| `replay` | Serve from fixtures or cache | HTTP 502 + JSON error | No |
| `record` | Serve from fixtures or cache | Fetch live, store in cache | Yes (cache only) |
| `live` | (skips cache entirely) | Fetch live | No |

Mode is held in `Proxy.mode` behind a `sync.RWMutex`. It is in-memory only — a container restart resets to `PROXY_MODE` env var (default: `replay`).

To change mode at runtime without restarting:
```bash
task proxy:mode:record   # or: curl -sX POST http://localhost:8055/admin/mode/record
```

### Two-Tier Storage

**Fixtures** (`PROXY_FIXTURES_DIR`, default `/app/fixtures`):
- Version-controlled in `crawler/fixtures/` and mounted read-only
- The proxy never writes here
- Always checked first before the user cache

**User cache** (`PROXY_CACHE_DIR`, default `/app/cache`):
- Mounted from `~/.northcloud/http-cache/` on the host
- Only `record` mode writes here
- Persists across container restarts (host volume)

### Cache Key Format

```
{METHOD}_{sha256(normalized_url + "\n" + header_hash)[:12]}
```

- **URL normalization**: lowercase host, strip `www.`, sort query params, strip tracking params (`utm_*`, `fbclid`, `gclid`, `ref`)
- **Header hash**: SHA-256 of `User-Agent + "\n" + Accept-Language`
- **Domain directory**: lowercase, `www.` stripped, dots → dashes (e.g. `news.example.com` → `news-example-com`)

Each entry is two files: `GET_abc123def456.json` (metadata) + `GET_abc123def456.body` (raw body).

### HTTPS MITM

The proxy intercepts HTTPS via a locally generated CA (stored in `PROXY_CERTS_DIR`):
1. Client sends `CONNECT host:443`
2. Proxy replies `200 Connection Established`, then performs TLS with a per-host leaf cert
3. The decrypted HTTP request is processed through the normal `handleHTTP` pipeline

The CA is generated on first startup if not found. Leaf certificates are cached in memory per hostname for the lifetime of the process.

To proxy HTTPS from a client that validates TLS (e.g., a local test tool), trust the CA at `PROXY_CERTS_DIR/ca.crt`. The crawler container in Docker Compose is already configured to accept the proxy's certificates via the `http_proxy` config setting.

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PROXY_PORT` | `8055` | Listen port |
| `PROXY_MODE` | `replay` | Startup mode (`replay`, `record`, `live`) |
| `PROXY_FIXTURES_DIR` | `/app/fixtures` | Read-only fixtures path |
| `PROXY_CACHE_DIR` | `/app/cache` | Writable user cache path |
| `PROXY_CERTS_DIR` | `/app/certs` | CA + leaf cert storage |
| `PROXY_LIVE_TIMEOUT` | `30s` | HTTP client timeout for outbound requests |

Config is loaded once at startup by `LoadConfig()`. All fields have defaults — no variable is required. Invalid `PROXY_MODE` values are silently ignored and the default (`replay`) is used.

## Common Gotchas

1. **Mode resets on container restart.** Mode is stored in memory. To persist a mode across restarts, set `PROXY_MODE` in `.env` or `docker-compose.dev.yml` before starting the container.

2. **`task proxy:clear` never deletes fixtures.** It only removes entries from `~/.northcloud/http-cache/`. Version-controlled fixtures in `crawler/fixtures/` are untouched and can only be removed with `git rm`.

3. **Fixtures are mounted read-only.** In Docker Compose dev, `crawler/fixtures` is mounted `:ro`. Any attempt by the proxy to write there (which it doesn't do) would fail silently. Copy recorded responses from the user cache to `crawler/fixtures/` on the host; don't expect the container to do it.

4. **Each cache entry is two files, not one.** The `*.json` holds metadata (request, response headers, timestamp). The `*.body` holds the raw response body. If one exists without the other, the entry is treated as a cache miss. Copy both files when committing fixtures.

5. **Cache key includes User-Agent and Accept-Language.** Two requests to the same URL with different `User-Agent` headers produce different cache keys. If replay returns a cache miss for a URL you know exists, check whether the user agent changed (e.g., after updating the crawler's HTTP client settings).

6. **Replay mode returns HTTP 502 on a miss — the crawler sees a server error.** This is intentional, not a proxy bug. The crawler will report a failed crawl. Check which URL triggered the miss via the `cache_key` field in the 502 response body, then record that URL.

7. **Record mode still serves fixtures first.** If a fixture exists for a URL, record mode serves it and does not re-fetch from the live server. To force a fresh recording, delete the matching fixture from `crawler/fixtures/` or clear the user cache with `task proxy:clear`.

8. **The CA certificate changes when `PROXY_CERTS_DIR` is not a persistent volume.** If the certs directory is ephemeral, a new CA is generated each restart. Clients that pinned the previous CA will fail TLS verification. In Docker Compose dev, the certs directory is managed by the container (not a named volume), so HTTPS clients may need reconfiguration after a container recreation.

9. **Domain normalization strips `www.` and replaces dots with dashes.** `www.example.com` and `example.com` map to the same directory `example-com`. Keep this in mind when organizing fixtures manually.

## Testing

```bash
# From nc-http-proxy/ directory
go test ./...              # All tests
go test -v ./...           # Verbose
go test -race ./...        # Race detector (recommended before committing)
go test -run TestCache ./... # Filter by name

# Via root Taskfile
task test:nc-http-proxy
task test:race:nc-http-proxy
task test:cover:nc-http-proxy   # Generates coverage.html
```

All tests use `httptest.Server` and `t.TempDir()` for fixtures/cache dirs — no running Docker or external network required. Integration tests live in `integration_test.go` and test the full mux (health, admin API, proxy behavior).

## Code Patterns

### Adding an Admin Endpoint

Admin routing is a plain `switch` on `r.URL.Path` + `r.Method` in `admin.go:ServeHTTP`. Add a new case:

```go
case path == "/admin/my-endpoint" && r.Method == http.MethodGet:
    h.handleMyEndpoint(w)
```

Always use `h.writeJSON(w, status, data)` for responses — it sets `Content-Type: application/json` and logs encode errors.

### Path Traversal Guard

The `safePath(baseDir, untrusted string)` helper in `admin.go` must be used any time a user-supplied domain name is joined to a file system path. It rejects `..` components and verifies the result stays within `baseDir`.

### Linting

The service uses the shared `.golangci.yml` from the repo root. The standard rules apply: no `interface{}` (use `any`), no magic numbers, no unchecked JSON errors, `t.Helper()` in all test helper functions.
