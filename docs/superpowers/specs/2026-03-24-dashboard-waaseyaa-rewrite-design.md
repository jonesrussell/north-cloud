# Dashboard Waaseyaa Rewrite — Design Spec

**Date:** 2026-03-24
**Status:** Draft
**Scope:** Replace the existing Vue 3 dashboard with a clean-slate Waaseyaa-scaffolded operator dashboard

---

## Context

The current North Cloud dashboard (`dashboard/`) is a Vue 3 + TypeScript SPA with 37 views across 7 categories. It was built while learning both what a content pipeline operator dashboard needs and what Grafana can do. Now that Grafana is mature (10 dashboards, 182 panels, 11 alert rules), the two tools have overlapping and unclear responsibilities.

This rewrite starts fresh — designing the ideal operator dashboard based on actual workflows, with a clear boundary between dashboard (actions) and Grafana (metrics).

## Decision: Dashboard vs Grafana Responsibility Split

**Rule: if you click a button, it's the dashboard. If you watch a graph, it's Grafana.**

### Operator Dashboard (Waaseyaa)

Actions, workflows, CRUD, queues — things you **do**:

- **Source Management** — CRUD, test-crawl, metadata validation, enable/disable
- **Crawl Control** — Start/schedule/pause/cancel jobs, view job detail
- **Classification Rules** — Create/edit topic rules, test against content
- **Channel Config** — Create/edit channels, preview routing rules
- **Verification Queue** — Review/accept/reject pending items (bulk ops)
- **Community Data** — Communities, leaders, band offices
- **Social Publishing** — Accounts, publish actions, retry failed
- **Index Lifecycle** — Create, migrate mappings, delete
- **Content Explorer** — Search/browse pipeline output, trace individual articles
- **Auth** — Login, session management

### Grafana

Metrics, trends, alerts — things you **watch**:

- Pipeline throughput (articles/min through each stage over time)
- Alerting (stalled classifiers, publisher lag, ML failures)
- Topic distribution (7-day trends, KL divergence, drift detection)
- Service health (CPU, memory, latency, uptime per service)
- ML sidecar metrics (accuracy, throughput, latency histograms)
- Log aggregation (structured logs from all services + Squid proxies)
- Error rate tracking (10+ errors/min alerts, per-service breakdowns)
- Proxy observability (IP rotation, block rates, latency per exit IP)
- Infrastructure (Elasticsearch cluster, Redis, Postgres health)
- AI Observer (drift reports, auto-remediation tracking)

### Bridge: Embedded Grafana Panels

Where operator context needs metrics alongside it, embed Grafana panels via iframe rather than rebuilding charts:

- Source detail page — that source's crawl success rate panel
- Crawl job view — crawler throughput panel for context
- Channel config — publish volume panel for that channel
- Dashboard home — pipeline overview panel as health summary

## Decision: Architecture Approach

**Hybrid: Waaseyaa Scaffold + Custom Vue SPA**

Use Waaseyaa's skeleton for project structure, build tooling, codified context, and dev workflow. Build a custom Vue 3 SPA that talks directly to Go APIs. Waaseyaa provides the foundation without imposing its entity model on data that lives in Go services.

Alternatives considered:
- **Thin PHP Shell** — Waaseyaa mostly unused at runtime, less framework value
- **Waaseyaa Admin Package** — entity-proxy adapter layer adds complexity without benefit since NC data lives in Go

## Project Structure

```
dashboard-waaseyaa/
├── CLAUDE.md                 (codified context, from skeleton)
├── .claude/                  (rules, settings)
├── composer.json             (waaseyaa framework dependency)
├── public/
│   └── index.php             (single entry point, serves SPA)
├── frontend/                 (Vue 3 SPA)
│   ├── src/
│   │   ├── app/              (Vue app setup, router, stores)
│   │   ├── features/         (feature modules)
│   │   ├── shared/           (API client, auth, UI primitives)
│   │   └── layouts/          (shell, sidebar, Grafana embed wrapper)
│   ├── package.json
│   └── vite.config.ts
├── config/                   (Waaseyaa config — minimal, mostly API URLs)
├── docs/                     (specs, design docs)
└── tests/                    (Vitest for frontend)
```

**Key decisions:**
- PHP serves the SPA shell in production (single route: `/*` → `index.html`). No PHP-side business logic.
- Vue app authenticates directly with NC auth service (JWT).
- API calls go directly to Go services via Vite proxy (dev) or nginx (prod).
- No PHP proxy layer — browser talks directly to Go services.

## Feature Modules

Organized around operator workflows, not backend service structure:

```
frontend/src/features/
├── sources/          Source CRUD, test-crawl, metadata validation, enable/disable
├── crawling/         Job list, start/schedule, pause/cancel, job detail + logs
├── classification/   Topic rules CRUD, test rule against content, reclassify
├── channels/         Channel CRUD, routing rule preview, publish history
├── verification/     Pending queue, bulk accept/reject, detail view
├── communities/      Community CRUD, leaders, band offices
├── social/           Social accounts, publish actions, retry failed
├── indexes/          Index list, create, migrate mappings, delete
├── content/          Search/browse pipeline output, article detail, trace through pipeline
└── home/             Pipeline health summary (embedded Grafana), quick actions
```

Each feature module is self-contained:

```
features/sources/
├── views/            (page components — SourceList, SourceDetail, SourceForm)
├── composables/      (useSourceApi, useSourcePolling)
├── components/       (SourceCard, TestCrawlDialog, MetadataPreview)
├── types.ts          (Source, SourceForm, SourceStatus)
└── index.ts          (route definitions, exported for router)
```

## Shared Infrastructure

```
frontend/src/shared/
├── api/
│   ├── client.ts           (Axios instance, JWT interceptor, base URL config)
│   ├── endpoints.ts        (typed endpoint map per service)
│   └── types.ts            (PaginatedResponse, ApiError, etc.)
├── auth/
│   ├── useAuth.ts          (login, logout, token refresh, auth state)
│   └── authGuard.ts        (Vue Router navigation guard)
├── components/
│   ├── GrafanaEmbed.vue    (iframe wrapper — panel ID, variables, loading state)
│   ├── DataTable.vue       (sortable, paginated table)
│   ├── ConfirmDialog.vue   (destructive action confirmation)
│   ├── StatusBadge.vue     (source/job/channel status indicators)
│   └── BulkActionBar.vue   (appears when items selected)
└── composables/
    ├── usePolling.ts       (configurable interval polling with pause/resume)
    ├── usePagination.ts    (offset/limit state, URL sync)
    └── useToast.ts         (success/error notifications)
```

**API client design:**
- Single Axios instance with JWT from localStorage
- Base URLs per service via environment variables
- TanStack Query for caching, background refetch, optimistic updates

**Auth flow:**
1. Vue router guard checks for valid JWT
2. No token → redirect to login view
3. Login view POSTs to `auth:8040/api/v1/auth/login` → stores JWT
4. All subsequent API calls include `Authorization: Bearer <token>`

## Navigation & Layout

Sidebar navigation grouped by pipeline stage:

- **Home** — Pipeline Overview (quick stats + embedded Grafana throughput panel + quick actions)
- **Content Intake** — Sources, Crawl Jobs, Verification Queue
- **Processing** — Classification Rules, Content Explorer
- **Distribution** — Channels, Social Publishing
- **Data** — Communities, Indexes

**Home page:**
- 4 quick stat cards: Active Sources, Running Jobs, Pending Review, Channels
- Embedded Grafana pipeline throughput panel
- Quick action buttons: Add Source, Start Crawl, Review Queue, Open Grafana

## Pre-work: Drift Detection Wiring

Before creating the new app, wire drift detection in Waaseyaa:

| Component | North Cloud | Waaseyaa |
|-----------|:-----------:|:--------:|
| Drift detector script | done | done |
| Taskfile task | done | **needed** |
| Lefthook git hooks | done | **needed** |
| CI integration | done | **needed** |

Tasks:
1. Add `Taskfile.yml` to Waaseyaa with `drift:check` task
2. Add `lefthook.yml` with pre-push `spec-drift` hook
3. Wire `drift:check` into CI workflow

## Framework Changes

The Waaseyaa skeleton is entity-focused. Creating an SPA-only app may reveal gaps:

- Skeleton may lack a `frontend/` convention for SPA apps
- Getting-started docs may need updates for non-entity use cases
- Build tooling adjustments for SPA-only projects

Process: discover issues during scaffolding → fix in framework → update getting-started docs → tag new release.

## Backend Services (No Changes)

The dashboard consumes existing Go API endpoints. No backend changes needed. Services and their ports:

| Service | Port | Dashboard Usage |
|---------|------|-----------------|
| Auth | 8040 | JWT login |
| Source Manager | 8050 | Source CRUD, communities, verification |
| Crawler | 8060 | Job management (via proxy/MCP) |
| Publisher | 8070 | Channels, routing, publish history |
| Classifier | 8071 | Rules, reclassify, stats |
| Index Manager | 8090 | Index lifecycle |
| Search | 8092 | Content explorer queries |
| Social Publisher | 8095 | Social account management |

## What's Explicitly Out of Scope

- Time-series charts (Grafana owns this)
- Log aggregation views (Grafana + Loki)
- Infrastructure monitoring (Grafana + Prometheus)
- ML model accuracy dashboards (Grafana)
- Porting any existing dashboard view 1:1
- PHP-side business logic or data storage
