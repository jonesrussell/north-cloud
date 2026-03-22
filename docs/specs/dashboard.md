# Dashboard Spec

> Last verified: 2026-03-20

## Overview

Vue 3 SPA that provides the operator UI for managing sources, crawl jobs, channels, classification rules, and monitoring pipeline health. Communicates with backend services via a Vite dev proxy (dev) or nginx reverse proxy (prod).

---

## File Map

```
dashboard/src/
  App.vue                  # Root component
  main.ts                  # Entry point
  style.css                # Global styles (Tailwind 4 @import syntax)
  router/index.ts          # Vue Router config, auth guard, route definitions
  stores/                  # Pinia state stores
  api/
    client.ts              # Shared Axios instance with auth interceptor
    auth.ts                # Auth API calls
    verification.ts        # Verification queue and moderation API client
  components/              # Reusable UI components (ui/, layout/, domain/, crawler/, etc.)
  views/                   # Page components (distribution/, feeds/, intake/, intelligence/, operations/, etc.)
  composables/             # Vue composables (useAuth, usePolling, useRealtime, etc.)
  types/                   # TypeScript interfaces
  config/                  # App-level configuration constants
```

---

## API Proxy Targets

The dashboard does not call backend services directly. In development, Vite proxies requests:

| Frontend prefix | Service | Default target |
|-----------------|---------|----------------|
| `/api/crawler` | Crawler | `http://localhost:8060` |
| `/api/sources`, `/api/cities` | Source Manager | `http://localhost:8050` |
| `/api/publisher` | Publisher | `http://localhost:8070` |
| `/api/classifier` | Classifier | `http://localhost:8071` |
| `/api/v1/auth`, `/api/auth` | Auth | `http://localhost:8040` |
| `/api/index-manager` | Index Manager | `http://localhost:8090` |
| `/api/verification` | Source Manager verification API | `http://localhost:8050` |

---

## Data Model

Core TypeScript interfaces defined in `src/types/`:

- **Source**: id, name, url, selectors, enabled
- **Channel**: id, name, description, enabled
- **Route**: id, source_id, channel_id, min_quality_score, topics, enabled

Verification operations use API-local interfaces from `src/api/verification.ts`:
- **VerificationPerson** and **VerificationBandOffice** carry queue metadata, source URL, and verification confidence/issues
- **PendingItem** is a discriminated union for `person` and `band_office` queue rows
- **VerificationStats** summarizes pending/scored counts and confidence buckets

Verification routes:
- `/operations/verification`
- `/operations/verification/stats`
- `/operations/verification/:type/:id`

---

## Configuration

Backend targets are Vite server-side proxy targets (not `VITE_` runtime vars):

| Variable | Default | Description |
|----------|---------|-------------|
| `CRAWLER_API_URL` | `http://localhost:8060` | Crawler API |
| `SOURCES_API_URL` | `http://localhost:8050` | Source Manager API |
| `PUBLISHER_API_URL` | `http://localhost:8070` | Publisher API |
| `CLASSIFIER_API_URL` | `http://localhost:8071` | Classifier API |
| `AUTH_API_URL` | `http://localhost:8040` | Auth service |
| `INDEX_MANAGER_API_URL` | `http://localhost:8090` | Index Manager API |
| `SOURCE_MANAGER_API_URL` | `http://localhost:8050` | Source Manager host for verification queue proxying |

Port: 3002 (dev server).

---

## Known Constraints

- **No `any` types**: ESLint enforces strict TypeScript. Use `unknown` and type guards.
- **Tailwind CSS v4**: uses `@import "tailwindcss"` syntax (not v3 `@tailwind` directives).
- **Auth token key**: `localStorage` key is `dashboard_token` (not `auth_token`).
- **No VITE_ runtime env vars for backend URLs**: all routing goes through Vite proxy config.
- **Vue Router base path**: `createWebHistory('/dashboard/')`. Production serves under `/dashboard/`.
- **SSE proxy timeout**: `/api/crawler` proxy has a 1-hour timeout for SSE log streaming.
- **Framework**: Vue 3 Composition API + TypeScript + Vite + TanStack Vue Query.
