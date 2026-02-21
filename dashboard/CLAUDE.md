# Dashboard — Developer Guide

This file provides guidance to Claude Code (claude.ai/code) when working with the dashboard service.

## Quick Reference

```bash
# npm commands
npm run dev          # Start dev server (http://localhost:3002)
npm run build        # Production build
npm run preview      # Preview production build
npm run lint         # ESLint (check)
npm run lint:fix     # ESLint (auto-fix)
npm run test         # Run Vitest unit tests (one-shot)
npm run test:watch   # Vitest in watch mode

# Taskfile equivalents (from repo root)
task dev             # npm run dev
task build           # npm run build
task lint            # npm run lint
```

## Architecture

```
dashboard/src/
├── App.vue              # Root component
├── main.ts              # Entry point
├── style.css            # Global styles (Tailwind 4 @import syntax)
├── router/              # Vue Router configuration (index.ts)
│   └── index.ts         # All routes + auth guard + title update hook
├── stores/              # Pinia state stores
├── api/                 # Axios client modules
│   ├── client.ts        # Shared Axios instance with auth interceptor
│   └── auth.ts          # Auth-specific API calls
├── components/          # Reusable UI components
│   ├── ui/              # Base UI primitives (buttons, inputs, badges)
│   ├── layout/          # App shell, sidebar, navigation
│   ├── domain/          # Domain components (classifier health widget, etc.)
│   ├── crawler/         # Crawler-specific components
│   ├── indexes/         # Index management components
│   ├── pipeline/        # Pipeline monitor components
│   └── common/          # Shared domain-agnostic components
├── views/               # Page components grouped by section
│   ├── distribution/    # Channels, routes, articles
│   ├── feeds/           # Delivery logs, Redis streams
│   ├── intake/          # Jobs, discovered links, frontier, rules
│   ├── intelligence/    # Indexes, documents, classifier stats, breakdowns
│   ├── operations/      # Review queue
│   ├── scheduling/      # Sources, cities, reputation (source-of-truth components)
│   └── system/          # Health, auth, cache
├── composables/         # Vue composables
│   ├── useAuth.ts       # Login / logout / isAuthenticated
│   ├── usePolling.ts    # Generic polling composable
│   ├── useRealtime.ts   # SSE / realtime data hook
│   ├── useHealthRealtime.ts
│   ├── useJobsRealtime.ts
│   ├── usePublishHistory.ts / usePublishHistoryTable.ts
│   ├── useServerPaginatedTable.ts
│   ├── useBulkOperations.ts
│   ├── useCommandPalette.ts
│   ├── useFormValidation.ts
│   ├── usePageNumbers.ts
│   ├── useRecentPages.ts
│   ├── useSidebar.ts
│   ├── useTheme.ts
│   └── useToast.ts
├── types/               # TypeScript interfaces (see Type Safety below)
├── features/            # Feature-specific modules
├── lib/                 # Utility functions and helpers
├── plugins/             # Vue plugin registrations
└── config/              # App-level configuration constants
```

## Key Concepts

### Type Safety — No `any`

**CRITICAL**: No `any` types are allowed anywhere in the codebase. The ESLint config enforces this. Use:

- `unknown` for truly unknown types, then narrow with type guards
- Specific interfaces for all API responses
- Proper generics for reusable utilities

Core types live in `src/types/`:

```typescript
interface Source {
  id: string;
  name: string;
  url: string;
  selectors: Selectors;
  enabled: boolean;
}

interface Channel {
  id: string;
  name: string;
  description: string;
  enabled: boolean;
}

interface Route {
  id: string;
  source_id: string;
  channel_id: string;
  min_quality_score: number;
  topics: string[];
  enabled: boolean;
}

interface ApiError {
  message: string;
  status: number;
  details?: unknown;
}
```

Use `ApiError` (not raw `Error`) for typed error handling in components.

### Auth Flow

1. User POSTs credentials to `/api/v1/auth/login`.
2. JWT token returned and stored in `localStorage` under key `dashboard_token`.
3. `useAuth` composable exposes `login`, `logout`, `isAuthenticated`.
4. The shared Axios instance in `src/api/client.ts` injects `Authorization: Bearer <token>` on every request via an interceptor.
5. Vue Router `beforeEach` guard (in `src/router/index.ts`) checks for `dashboard_token` on every navigation and redirects to `/login` if absent.

```typescript
const { login, logout, isAuthenticated, user } = useAuth();
```

### Vue Query Pattern

Server state is managed with TanStack Vue Query (`@tanstack/vue-query`). Prefer `useQuery` / `useMutation` over raw Axios calls in views and composables:

```typescript
import { useQuery, useMutation } from '@tanstack/vue-query';
import { sourcesApi } from '@/api/sources';

const { data: sources, isLoading } = useQuery({
  queryKey: ['sources'],
  queryFn: () => sourcesApi.list(),
});
```

### Axios Interceptors

`src/api/client.ts` creates the shared Axios instance with:
- Request interceptor: injects `Authorization` header from `localStorage`.
- Response interceptor: handles 401 → redirect to login.

Import the shared client rather than creating new `axios` instances.

### Tailwind CSS 4 Syntax

The project uses Tailwind CSS v4. The import syntax is **different from v3**:

```css
/* style.css — correct v4 syntax */
@import "tailwindcss";
```

Do NOT use the old v3 directives (`@tailwind base`, `@tailwind components`, `@tailwind utilities`). Tailwind is integrated via the `@tailwindcss/vite` plugin; no `tailwind.config.js` is needed.

### Component Structure

All components use `<script setup lang="ts">` with explicit prop and emit types:

```vue
<script setup lang="ts">
import { ref, computed } from 'vue';
import type { Source } from '@/types';

const props = defineProps<{
  source: Source;
}>();

const emit = defineEmits<{
  update: [source: Source];
}>();
</script>

<template>
  <!-- Template -->
</template>
```

## API Reference

The dashboard talks to six backend services. The dev server (Vite) proxies requests:

| Frontend prefix | Service | Default target |
|-----------------|---------|----------------|
| `/api/crawler` | Crawler | `http://localhost:8060` |
| `/api/sources`, `/api/cities` | Source Manager | `http://localhost:8050` |
| `/api/publisher` | Publisher | `http://localhost:8070` |
| `/api/classifier` | Classifier | `http://localhost:8071` |
| `/api/v1/auth`, `/api/auth` | Auth | `http://localhost:8040` |
| `/api/index-manager` | Index Manager | `http://localhost:8090` |
| `/api/health/{service}` | Per-service health | respective service |

SSE (Server-Sent Events) for realtime log streaming from the crawler uses a 1-hour proxy timeout — do not reduce this.

## Configuration

Backend targets are set as shell environment variables before running `npm run dev` (they are Vite server-side proxy targets, not `VITE_` runtime vars):

```bash
CRAWLER_API_URL=http://localhost:8060       # Crawler API
SOURCES_API_URL=http://localhost:8050       # Source Manager API
PUBLISHER_API_URL=http://localhost:8070     # Publisher API
CLASSIFIER_API_URL=http://localhost:8071    # Classifier API
AUTH_API_URL=http://localhost:8040          # Auth service
INDEX_MANAGER_API_URL=http://localhost:8090 # Index Manager API
```

The CLAUDE.md previously documented `VITE_API_BASE_URL` and `VITE_AUTH_URL`. Those are **not used**. All routing goes through Vite's proxy configuration in `vite.config.ts`.

## Common Gotchas

1. **Auth token key**: The router guard reads `localStorage.getItem('dashboard_token')`. The old key was `auth_token`; make sure any new code uses `dashboard_token`.

2. **No VITE_ runtime env vars**: Backend URLs are proxy targets configured in `vite.config.ts`, not injected into the browser bundle. Do not add `import.meta.env.VITE_*` calls for backend URLs.

3. **Tailwind v4 syntax**: Use `@import "tailwindcss"` in CSS files, not `@tailwind` directives. Utility class names and theme tokens follow v4 conventions.

4. **Vue Router base path**: The router uses `createWebHistory('/dashboard/')`. All internal `<router-link>` paths are relative to this base. In production, nginx serves the app under `/dashboard/`.

5. **Route guards default to `requiresAuth: true`**: Any route without `meta: { requiresAuth: false }` is protected. The guard does `to.meta.requiresAuth !== false` — omitting the meta field means the route is protected.

6. **Legacy routes**: A large set of redirect entries in `src/router/index.ts` maps old URL shapes (e.g., `/crawler/jobs`, `/publisher/channels`) to new paths. Do not remove them without checking external links.

7. **SSE proxy timeout**: The `/api/crawler` proxy has a 1-hour timeout for SSE log streaming. Other proxies use 30 seconds; health checks use 10 seconds. Match these when adding new proxy entries.

8. **Error handling**: Use the `ApiError` interface for typed errors. Do not use bare `catch (e: any)` — use `catch (e: unknown)` and narrow the type.

9. **`@` path alias**: `@` resolves to `src/`. Use it for all imports within the project (`import { sourcesApi } from '@/api/sources'`).

## Testing

The project uses Vitest (configured in `vitest.config.ts`). Run tests with:

```bash
npm run test         # One-shot run
npm run test:watch   # Watch mode
```

Currently, test coverage is minimal. New features should add unit tests for composables and utility functions. Helper functions in tests must follow the project's Go convention analogue — ensure test utilities are well-scoped.

## ESLint Configuration

ESLint uses the flat config format (`eslint.config.js`) with `eslint-plugin-vue` and `typescript-eslint`. Run:

```bash
npm run lint         # Check
npm run lint:fix     # Check and auto-fix
```

The linter enforces no `any` types, Vue 3 best practices, and TypeScript strict mode. Fix all lint errors before committing.
