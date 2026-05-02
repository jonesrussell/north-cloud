# playwright-renderer (CLAUDE.md)

Lightweight headless-browser sidecar for on-demand JS page rendering. **Prod only.**

## When you're working here

- This is a small (3-file) Node.js + Express + Playwright service
- Consumers in prod: `mcp-north-cloud` (`fetch_url` MCP tool with `js_render=true`) and `signal-crawler` (when `RENDERER_ENABLED=true`)
- Not the same as `render-worker/` — see `render-worker/CLAUDE.md` for the crawler's main rendering pipeline. Don't consolidate without migrating both consumers.

## Files

- `server.js` — express app, two routes: `POST /render`, `GET /health`
- `package.json` — declares deps `express ^4.18.2`, `playwright ^1.48.0`
- `Dockerfile` — `FROM mcr.microsoft.com/playwright:v1.58.2-jammy`

## Quick local test

```bash
docker run --rm -p 8095:8095 docker.io/jonesrussell/playwright-renderer:latest
curl http://localhost:8095/health
curl -X POST http://localhost:8095/render \
  -H 'content-type: application/json' \
  -d '{"url":"https://example.com"}' | jq -r .html | head -5
```

## Editing rules

- Keep dependencies minimal — service is intentionally a thin wrapper around chromium
- Concurrency cap stays low (default 3) — this is a single-tenant prod sidecar, not a high-throughput pipeline
- Resource blocking (image/media/font/CSS) is intentional — do not relax without a benchmark showing it matters
- Bumping the Playwright base image: also bump `package.json` `playwright` to a compatible version

## Spec

See `README.md` for full API + config + architecture context. No `docs/specs/` entry exists today (small service, single API surface — not yet warranted).
