# Render Worker

Playwright-based Node.js sidecar for dynamic HTML rendering. Optional service — if `CRAWLER_RENDER_WORKER_URL` is unset, the crawler falls back to static HTML fetching.

## Architecture

Single-process HTTP server (no framework) managing a headless Chromium browser pool.

```
Crawler → POST /render → render-worker → Chromium → rendered HTML
```

## Files

| File | Purpose |
|------|---------|
| `index.js` | Server lifecycle, browser management |
| `handler.js` | HTTP request/response, queue management |
| `stealth.js` | Browser fingerprint masking (navigator.webdriver, plugins, languages) |
| `test.js` | Unit and integration tests |

## Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/render` | Render URL, return HTML + final URL + status |
| GET | `/health` | Health check |

## Configuration

| Env Var | Default | Purpose |
|---------|---------|---------|
| `PORT` | 3000 | HTTP server port |
| `BROWSER_RECYCLE_AFTER` | 100 | Restart browser after N requests |
| `DEFAULT_TIMEOUT_MS` | 15000 | Render timeout |

Hardcoded: `MAX_QUEUE_DEPTH=50`, `MAX_BODY_BYTES=1MB`.

## Integration

- Crawler imports `internal/render/client.go` which POSTs to `{CRAWLER_RENDER_WORKER_URL}/render`
- Bootstrap in `crawler/internal/bootstrap/services.go` creates `renderClientAdapter` bridging to fetcher's `PageRenderer` interface
- Docker: 1 CPU, 512MB memory limit, `north-cloud-render-worker` container

## Commands

```bash
npm test          # Run tests
npm start         # Start server
```
