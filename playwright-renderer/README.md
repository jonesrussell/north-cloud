# playwright-renderer

A lightweight headless-browser renderer sidecar. Deployed in **production only**, gated behind two consumers' opt-in env vars.

## Purpose

Renders JS-heavy pages on demand and returns the rendered HTML. Optimized for one-off requests (small concurrency cap, persistent browser, image/media/font/CSS resource blocking) rather than sustained pipeline throughput.

## Consumers

| Consumer | How | Compose entry |
|---|---|---|
| `mcp-north-cloud` (`fetch_url` MCP tool) | Sets `RENDERER_URL: "http://playwright-renderer:8095"`; called when `fetch_url` is invoked with `js_render=true` | `docker-compose.prod.yml` |
| `signal-crawler` | Sets `RENDERER_URL=http://playwright-renderer:8095` and `RENDERER_ENABLED=true`; used per crawl when JS rendering is required | `docker-compose.prod.yml` |

When neither consumer needs JS rendering, the sidecar is unused but still scheduled (cheap, idle resource cost).

## Not the same as `render-worker/`

`render-worker/` is the crawler's main rendering pipeline (port 3000, `north-cloud-render-worker` container). It uses queued requests, stealth scripts (`stealth.js`), infinite-scroll handling (`scroll.js`), browser recycling after N requests, and a custom HTTP handler. Its design goal is sustained throughput inside the crawl loop.

`playwright-renderer/` (this directory, port 8095) is a smaller, simpler service for on-demand rendering outside the crawl loop. The two are intentionally separate; do not consolidate without migrating both consumers and the crawler-side queue logic.

## Layout

```
playwright-renderer/
├── package.json     # name = "playwright-renderer", v1.0.0; deps: express, playwright
├── Dockerfile       # Built FROM mcr.microsoft.com/playwright:v1.58.2-jammy
└── server.js        # Express app: POST /render, GET /health
```

## API

### `POST /render`

Request:
```json
{
  "url": "https://example.com/article",
  "wait_for": "networkidle",       // or "domcontentloaded"; default "networkidle"
  "timeout_ms": 30000              // capped at 60000
}
```

Response (200):
```json
{ "html": "<!DOCTYPE html>...", "url": "https://example.com/article" }
```

Errors: `400` if `url` missing, `503` if at concurrency cap, `500` on render failure.

### `GET /health`

Returns `{ "status": "ok", "active_requests": <int> }`.

## Configuration

| Env var | Default | Purpose |
|---|---|---|
| `RENDERER_PORT` | `8095` | Listen port |
| `RENDERER_TIMEOUT_MS` | `30000` | Per-request browser timeout (capped at 60s) |
| `RENDERER_MAX_CONCURRENT` | `3` | Reject (503) when this many requests are in-flight |

Resource blocking: the renderer aborts requests for `image`, `media`, `font`, and `stylesheet` resource types to keep render time fast (HTML/JS only).

## Production deploy

Deployed via `docker-compose.prod.yml` as `playwright-renderer` (image `docker.io/jonesrussell/playwright-renderer:${PLAYWRIGHT_RENDERER_TAG:-latest}`). 1 CPU / 1 GB memory limit. 30s healthcheck against `/health`.

Not present in `docker-compose.base.yml` or `docker-compose.dev.yml` — use `render-worker/` (port 3000) for local rendering experiments.

## Audit note (2026-04-30)

The 2026-04-30 code-smell audit initially flagged this directory for deletion as "no compose entry, no consumer". That was incorrect — the audit checked `docker-compose.base.yml` only and missed both the prod compose entry and the two prod consumers. Mission A WP03 closed as "audit was wrong; service is live in prod". See `kitty-specs/audit-dead-code-removal-01KQG57Z/tasks/WP03-...md` for evidence.
