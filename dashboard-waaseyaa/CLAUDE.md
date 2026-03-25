# CLAUDE.md — Dashboard Waaseyaa

## Purpose

Operator dashboard for the North Cloud content pipeline. Vue 3 SPA served on a Waaseyaa scaffold.
PHP serves the SPA shell and handles SSR concerns; Vue talks directly to Go microservice APIs.
Grafana handles metrics/observability dashboards (embedded via iframe).

## Architecture

```
Waaseyaa (PHP)          Vue 3 SPA (frontend/)
├── SPA shell serving    ├── src/shared/api/     Axios client + endpoints
├── Auth bootstrap       ├── src/shared/auth/    JWT auth composable + guard
└── Config/env           ├── src/shared/components/  Reusable UI
                         ├── src/features/       Feature modules
                         ├── src/layouts/         AppShell, Sidebar
                         └── src/app/router.ts   Route definitions
```

Vue communicates with Go services via Vite dev proxy (dev) or Nginx reverse proxy (prod).

## Commands

```bash
# Frontend (from dashboard-waaseyaa/frontend/)
npm run dev          # Vite dev server on port 3002
npm run build        # Production build
npm test             # Alias for vitest run
npm run lint         # ESLint + Prettier check

# Backend (from dashboard-waaseyaa/)
composer install     # Install PHP deps
php bin/waaseyaa migrate  # Run migrations
```

## API Services (Go Microservices)

| Service | Port | Proxy path |
|---------|------|------------|
| auth | 8040 | `/api/auth` |
| source-manager | 8050 | `/api/sources` |
| crawler | 8080 | `/api/crawler` |
| publisher | 8070 | `/api/publisher` |
| classifier | 8070 | `/api/classifier` |
| index-manager | 8090 | `/api/index-manager` |
| search | 8092 | `/api/search` |

## Conventions

- **Vue 3 Composition API** with `<script setup lang="ts">`
- **TypeScript strict** — no `any` types; use `unknown` for generic values
- **TanStack Query** for all server state (no manual fetch + useState)
- **Pinia** for client-only state (auth token, UI preferences)
- **Tailwind CSS v4** for styling — dark theme (slate palette)
- **Types** in `src/shared/api/types.ts` or co-located with features

## Orchestration Table

| File Pattern | Skill | Spec |
|-------------|-------|------|
| `frontend/src/shared/**` | `feature-dev` | — |
| `frontend/src/features/**` | `feature-dev` | — |
| `frontend/src/layouts/**` | `feature-dev` | — |
| `src/Entity/**` | `waaseyaa:entity-system` | entity-system.md |
| `src/Access/**` | `waaseyaa:access-control` | access-control.md |
| `src/Provider/**` | `feature-dev` | — |
| `.claude/rules/**` | `updating-codified-context` | — |
| `docs/specs/**` | `updating-codified-context` | — |

## MCP Federation

Register Waaseyaa's MCP server in `.claude/settings.json` for on-demand framework specs:

```json
{
  "mcpServers": {
    "waaseyaa": {
      "command": "node",
      "args": ["vendor/waaseyaa/mcp/server.js"],
      "cwd": "."
    }
  }
}
```

## Codified Context

This app uses a three-tier codified context system inherited from Waaseyaa:

| Tier | Location | Purpose |
|------|----------|---------|
| **Constitution** | `CLAUDE.md` (this file) | Architecture, conventions, orchestration |
| **Rules** | `.claude/rules/waaseyaa-*.md` | Framework invariants (always active, never cited) |
| **Specs** | `docs/specs/*.md` | Domain contracts for each subsystem |

Framework rules are owned by Waaseyaa. Update them via `bin/waaseyaa sync-rules` after `composer update`.

## Gotchas

- **Never use `$_ENV`** — Waaseyaa's `EnvLoader` only populates `putenv()`/`getenv()`. Use `getenv()` or the `env()` helper.
- **SQLite write access** — Both the `.sqlite` file AND its parent directory need write permissions for WAL/journal files.

## Reference

- Design spec: `docs/superpowers/specs/2026-03-24-dashboard-waaseyaa-rewrite-design.md`
- Root project: see `/home/fsd42/dev/north-cloud/CLAUDE.md`
