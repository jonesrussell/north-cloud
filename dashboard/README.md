# Dashboard

> Vue.js 3 management interface for the North Cloud content pipeline.

## Overview

The dashboard is the admin UI for configuring and monitoring every stage of the North Cloud pipeline. It provides a unified view of content sources and crawl scheduling, classifier health and index contents, publisher channels and routing rules, and delivery logs — giving operators a single pane of glass over the full Crawl → Classify → Publish workflow.

## Features

- **Pipeline Monitor** — real-time cockpit showing the overall health of all pipeline stages
- **Content Intake** — manage crawler jobs, browse discovered links, inspect the URL frontier, and configure crawl rules
- **Sources** — create, edit, and delete content sources; manage city groupings and source reputation scores
- **Intelligence** — explore Elasticsearch indexes and individual documents; view classifier stats and breakdowns by crime type, mining, and location
- **Distribution** — manage Redis pub/sub channels, configure source-to-channel routing rules, browse delivery logs
- **Operations** — browse recently published articles; review queue for manual content triage
- **System** — service health checks, auth token management, and cache inspection

## Quick Start

### Docker (Recommended)

The dashboard is available at http://localhost:3002 when running the full North Cloud stack:

```bash
task docker:dev:up
```

In production it is served by nginx at `/dashboard/`.

### Local Development

Node.js 20+ is required.

```bash
cd dashboard
npm install
npm run dev     # http://localhost:3002
```

The dev server proxies all `/api/*` requests to the backend services on their default local ports (see [Configuration](#configuration) below).

## Routes

| Path | View | Description |
|------|------|-------------|
| `/login` | LoginView | Login page (public) |
| `/` | PipelineMonitorView | Pipeline health cockpit (home) |
| `/operations/articles` | ArticlesView | Recently published articles |
| `/operations/review` | ReviewQueueView | Manual review queue |
| `/intelligence` | IntelligenceOverviewView | Intelligence overview |
| `/intelligence/crime` | CrimeBreakdownView | Crime classification breakdown |
| `/intelligence/mining` | MiningBreakdownView | Mining classification breakdown |
| `/intelligence/location` | LocationBreakdownView | Location classification breakdown |
| `/intelligence/indexes` | IndexesView | Elasticsearch index explorer |
| `/intelligence/indexes/:index_name` | IndexDetailView | Index details and document listing |
| `/intelligence/indexes/:index_name/documents/:document_id` | DocumentDetailView | Individual document viewer |
| `/intelligence/stats` | ClassifierStatsView | Classifier statistics (legacy) |
| `/intake/jobs` | JobsView | Crawler jobs list |
| `/intake/jobs/:id` | JobDetailView | Crawler job details and execution history |
| `/intake/discovered-links` | DiscoveredLinksView | Discovered link browser |
| `/intake/frontier` | FrontierView | URL frontier inspector |
| `/intake/rules` | RulesView | Classifier and crawl rules |
| `/sources` | SourcesView | All configured sources |
| `/sources/new` | SourceFormView | Create a new source |
| `/sources/:id/edit` | SourceFormView | Edit an existing source |
| `/sources/cities` | CitiesView | City groupings for sources |
| `/sources/reputation` | ReputationView | Source reputation scores |
| `/distribution/channels` | ChannelsView | Redis pub/sub channel management |
| `/distribution/routes` | RoutesView | Source-to-channel routing rules |
| `/distribution/logs` | DeliveryLogsView | Article delivery log |
| `/system/health` | HealthView | Service health checks |
| `/system/auth` | AuthView | Auth token management |
| `/system/cache` | CacheView | Cache inspection |

All routes except `/login` require a valid JWT token. Unauthenticated requests are redirected to `/login`.

## Authentication

The dashboard uses JWT-based authentication. The token is stored in `localStorage` under the key `dashboard_token`. On every navigation the Vue Router guard checks for a valid token and redirects to `/login` if one is not present.

Credentials are controlled by the `AUTH_USERNAME` and `AUTH_PASSWORD` environment variables on the auth service. The token is obtained by posting to `/api/v1/auth/login`.

## Configuration

Backend URLs are configured at build/dev-server time via environment variables. In production, nginx routes all `/api/*` traffic so no explicit URLs are needed.

| Variable | Default | Description |
|----------|---------|-------------|
| `CRAWLER_API_URL` | `http://localhost:8060` | Crawler service URL (dev proxy) |
| `SOURCES_API_URL` | `http://localhost:8050` | Source Manager service URL (dev proxy) |
| `PUBLISHER_API_URL` | `http://localhost:8070` | Publisher service URL (dev proxy) |
| `CLASSIFIER_API_URL` | `http://localhost:8071` | Classifier service URL (dev proxy) |
| `AUTH_API_URL` | `http://localhost:8040` | Auth service URL (dev proxy) |
| `INDEX_MANAGER_API_URL` | `http://localhost:8090` | Index Manager service URL (dev proxy) |

These are Vite dev-server proxy targets, not `VITE_` prefixed runtime variables. Set them in the shell before running `npm run dev` if your services run on non-default ports.

## Architecture

```
dashboard/src/
├── api/          # Axios client modules per service (client.ts, auth.ts)
├── components/   # Shared UI components
│   ├── ui/       # Base components (buttons, inputs, badges)
│   ├── layout/   # App shell, sidebar, nav
│   ├── domain/   # Domain components (classifier health widget, etc.)
│   ├── crawler/  # Crawler-specific components
│   ├── indexes/  # Index management components
│   ├── pipeline/ # Pipeline monitor components
│   └── common/   # Shared domain-agnostic components
├── composables/  # Vue composables (useAuth, usePolling, useRealtime, etc.)
├── config/       # App-level configuration constants
├── features/     # Feature-specific modules
├── lib/          # Utility functions and helpers
├── plugins/      # Vue plugin registrations
├── router/       # Vue Router with auth guards (src/router/index.ts)
├── stores/       # Pinia state stores
├── types/        # TypeScript interfaces (Source, Channel, Route, etc.)
└── views/        # Page components organised by section
    ├── distribution/   # Channels, routes, articles
    ├── feeds/          # Delivery logs, Redis streams
    ├── intake/         # Jobs, discovered links, frontier, rules
    ├── intelligence/   # Indexes, documents, classifier stats, breakdowns
    ├── operations/     # Review queue
    ├── scheduling/     # Sources, cities, reputation (source of truth components)
    └── system/         # Health, auth, cache
```

## Tech Stack

| Technology | Version | Role |
|------------|---------|------|
| Vue 3 (Composition API) | ^3.5.25 | UI framework |
| TypeScript | ~5.5.4 | Type safety |
| Vite | ^7.2.7 | Build tool and dev server |
| Tailwind CSS | ^4.1.17 | Utility-first styling |
| Pinia | ^2.3.1 | State management |
| TanStack Vue Query | ^5.92.9 | Async server state |
| Axios | ^1.13.5 | HTTP client |
| Vue Router | ^4.6.3 | Client-side routing |
| Radix Vue | ^1.9.0 | Headless UI primitives |
| Headless UI | ^1.7.23 | Additional headless components |
| Lucide Vue Next | ^0.400.0 | Icon set |
| VueUse | ^10.11.0 | Composition utility collection |
| vue-sonner | ^2.0.9 | Toast notifications |
| Vitest | ^4.0.18 | Unit testing |

## Development

```bash
npm run dev          # Start dev server on port 3002
npm run build        # Production build to dist/
npm run preview      # Preview the production build locally
npm run lint         # Run ESLint (Vue + TypeScript rules)
npm run lint:fix     # ESLint with auto-fix
npm run test         # Run Vitest unit tests (one-shot)
npm run test:watch   # Run Vitest in watch mode
```

Taskfile equivalents (from repo root):

```bash
task dev             # npm run dev
task build           # npm run build
task lint            # npm run lint
```

## Integration

The dashboard communicates with the following North Cloud backend services:

| Service | Dev proxy prefix | Production path |
|---------|-----------------|-----------------|
| Crawler | `/api/crawler` → crawler:8060 | nginx → crawler:8060 |
| Source Manager | `/api/sources`, `/api/cities` → source-manager:8050 | nginx → source-manager:8050 |
| Publisher | `/api/publisher` → publisher:8070 | nginx → publisher:8070 |
| Classifier | `/api/classifier` → classifier:8071 | nginx → classifier:8071 |
| Auth | `/api/v1/auth`, `/api/auth` → auth:8040 | nginx → auth:8040 |
| Index Manager | `/api/index-manager` → index-manager:8090 | nginx → index-manager:8090 |

All service health checks are proxied through `/api/health/{service}`.
