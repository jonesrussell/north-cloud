# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with the dashboard service.

## Quick Reference

```bash
# Development
npm run dev           # Start dev server (port 3002)
npm run build         # Build for production
npm run lint          # Run ESLint
npm run preview       # Preview production build

# Or using Taskfile
task dev
task build
task lint
```

## Architecture

```
dashboard/src/
├── App.vue              # Root component
├── main.ts              # Entry point
├── router/              # Vue Router configuration
├── stores/              # Pinia stores
├── api/                 # API client modules
├── components/          # Reusable UI components
│   ├── ui/              # Base UI components (buttons, inputs)
│   ├── layout/          # Layout components
│   ├── sources/         # Source management components
│   ├── routes/          # Route management components
│   └── channels/        # Channel management components
├── views/               # Page components
│   ├── DashboardView.vue
│   ├── SourcesView.vue
│   ├── RoutesView.vue
│   ├── ChannelsView.vue
│   └── LoginView.vue
├── composables/         # Vue composables (useAuth, useApi)
├── types/               # TypeScript type definitions
├── features/            # Feature-specific modules
├── lib/                 # Utility libraries
├── plugins/             # Vue plugins
└── config/              # App configuration
```

## Tech Stack

- **Vue 3** - Composition API
- **TypeScript** - Strict type checking
- **Vite** - Build tool
- **Tailwind CSS 4** - Styling
- **Pinia** - State management
- **Vue Query** (@tanstack/vue-query) - Server state
- **Axios** - HTTP client
- **Radix Vue** - Headless UI components
- **Lucide** - Icons

## Type Safety Rules

**CRITICAL**: No `any` types allowed. Use:
- `unknown` for truly unknown types, then narrow
- Specific interfaces for API responses
- Proper generics for reusable code

**Types Directory** (`src/types/`):
```typescript
// Source type
interface Source {
  id: string;
  name: string;
  url: string;
  selectors: Selectors;
  enabled: boolean;
}

// Channel type
interface Channel {
  id: string;
  name: string;
  description: string;
  enabled: boolean;
}

// Route type
interface Route {
  id: string;
  source_id: string;
  channel_id: string;
  min_quality_score: number;
  topics: string[];
  enabled: boolean;
}
```

## Authentication

**JWT-based authentication**:
- Token stored in `localStorage`
- `useAuth` composable handles login/logout
- Route guards protect authenticated routes
- API interceptors add `Authorization` header

```typescript
// useAuth composable
const { login, logout, isAuthenticated, user } = useAuth();
```

## API Layer

**Axios with interceptors** (`src/api/`):
- Automatic token injection
- Error handling
- Base URL configuration

```typescript
// Example API call
import { sourcesApi } from '@/api/sources';

const sources = await sourcesApi.list();
const source = await sourcesApi.get(id);
await sourcesApi.create(sourceData);
```

## Common Patterns

**Component Structure**:
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

**Using Vue Query**:
```typescript
import { useQuery, useMutation } from '@tanstack/vue-query';

const { data: sources, isLoading } = useQuery({
  queryKey: ['sources'],
  queryFn: () => sourcesApi.list()
});
```

## Common Gotchas

1. **API base URL**: Set via `VITE_API_BASE_URL` env var or defaults to `http://localhost:8070`.

2. **Auth token storage**: Uses `localStorage.getItem('auth_token')`.

3. **Error handling**: Use `ApiError` interface for typed error handling:
   ```typescript
   interface ApiError {
     message: string;
     status: number;
     details?: unknown;
   }
   ```

4. **Tailwind CSS 4**: Uses new `@import "tailwindcss"` syntax, not `@tailwind` directives.

5. **Vue Router guards**: Auth check runs on every navigation.

## Environment Variables

```bash
VITE_API_BASE_URL=http://localhost:8070  # API server URL
VITE_AUTH_URL=http://localhost:8040      # Auth service URL
```

## Testing

Currently no test framework configured. Manual testing via dev server.

## ESLint Configuration

Uses Vue-specific ESLint config with TypeScript support. Run `npm run lint` to check and auto-fix issues.
