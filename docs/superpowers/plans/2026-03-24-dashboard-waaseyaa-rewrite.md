# Dashboard Waaseyaa Rewrite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the existing Vue 3 dashboard with a clean-slate Waaseyaa-scaffolded operator dashboard (Phase 1: Auth + Home + Sources + Crawling).

**Architecture:** Waaseyaa skeleton provides project structure, codified context, and dev workflow. Custom Vue 3 SPA in `frontend/` talks directly to Go API services via Axios + TanStack Query. JWT auth against NC auth service.

**Tech Stack:** PHP 8.4 (Waaseyaa skeleton), Vue 3 Composition API, TypeScript, Vite, TanStack Query v5, Tailwind CSS v4, Axios, Vitest, Playwright

**Spec:** `docs/superpowers/specs/2026-03-24-dashboard-waaseyaa-rewrite-design.md`

---

## Pre-work

### Task 0: Wire Drift Detection in Waaseyaa

**Context:** Waaseyaa has `tools/drift-detector.sh` but it lacks enforcement (no exit codes, no timestamp comparison) and isn't wired into Taskfile or lefthook.

**Files:**
- Modify: `/home/fsd42/dev/waaseyaa/tools/drift-detector.sh`
- Create: `/home/fsd42/dev/waaseyaa/Taskfile.yml`
- Create: `/home/fsd42/dev/waaseyaa/lefthook.yml`
- Modify: `/home/fsd42/dev/waaseyaa/.github/workflows/ci.yml`

**Reference:** North Cloud's implementation at `/home/fsd42/dev/north-cloud/tools/drift-detector.sh` (423 lines, production-ready with timestamp comparison and exit code enforcement).

- [ ] **Step 1: Enhance drift-detector.sh with enforcement**

Read North Cloud's `tools/drift-detector.sh` for reference. Update Waaseyaa's script to add:
- Timestamp comparison (compare spec last-modified vs service last-modified in git history)
- Exit code 1 when specs are stale (currently always exits 0)
- Exclusion filters for non-spec-affecting files (test files, `.claude/`, `composer.lock`)

Run: `cd /home/fsd42/dev/waaseyaa && bash tools/drift-detector.sh 5`
Expected: Exits 0 if specs are current, 1 if stale

- [ ] **Step 2: Create Taskfile.yml**

```yaml
version: '3'

tasks:
  drift:check:
    desc: "Check for spec drift (stale specs vs recent changes)"
    cmds:
      - tools/drift-detector.sh {{.CLI_ARGS | default "5"}}

  lint:
    desc: "Run PHP linting"
    cmds:
      - vendor/bin/phpstan analyse --no-progress

  test:
    desc: "Run PHPUnit tests"
    cmds:
      - vendor/bin/phpunit

  ci:
    desc: "Run full CI pipeline"
    cmds:
      - task: drift:check
      - task: lint
      - task: test
```

Run: `cd /home/fsd42/dev/waaseyaa && task drift:check`
Expected: Drift detector runs, shows results

- [ ] **Step 3: Create lefthook.yml**

```yaml
pre-push:
  commands:
    spec-drift:
      run: tools/drift-detector.sh 5
```

Run: `cd /home/fsd42/dev/waaseyaa && lefthook install`
Expected: Hook installed successfully

- [ ] **Step 4: Wire drift:check into CI workflow**

Read `/home/fsd42/dev/waaseyaa/.github/workflows/ci.yml`. Add a `spec-drift` job or step that runs `tools/drift-detector.sh 5` before other CI steps.

- [ ] **Step 5: Verify North Cloud drift detection still works**

Run: `cd /home/fsd42/dev/north-cloud && task drift:check`
Expected: Exits 0 (all specs current)

- [ ] **Step 6: Commit**

```bash
cd /home/fsd42/dev/waaseyaa
git add tools/drift-detector.sh Taskfile.yml lefthook.yml .github/workflows/ci.yml
git commit -m "ci: wire drift detection into Taskfile, lefthook, and CI"
```

---

## Phase 1: Scaffold & Foundation

### Task 1: Scaffold Waaseyaa App

**Context:** Create the new dashboard app using Waaseyaa's skeleton. The skeleton is entity-focused (PHP/Twig) — we'll strip the entity-specific parts and add a Vue SPA in later tasks.

**Files:**
- Create: `/home/fsd42/dev/north-cloud/dashboard-waaseyaa/` (entire skeleton)

- [ ] **Step 1: Run composer create-project**

```bash
cd /home/fsd42/dev/north-cloud
composer create-project waaseyaa/waaseyaa dashboard-waaseyaa --stability=alpha
```

Expected: Skeleton created with `bin/`, `config/`, `public/`, `src/`, `templates/`, `storage/`, `CLAUDE.md`, `.claude/`

- [ ] **Step 2: Verify codified context is wired**

Check these files exist:
- `dashboard-waaseyaa/CLAUDE.md`
- `dashboard-waaseyaa/.claude/rules/waaseyaa-data-freshness.md`
- `dashboard-waaseyaa/.claude/rules/waaseyaa-framework.md`
- `dashboard-waaseyaa/.claude/rules/waaseyaa-shell-compat.md`

Run: `ls dashboard-waaseyaa/CLAUDE.md dashboard-waaseyaa/.claude/rules/`
Expected: All 4 files present

- [ ] **Step 3: Verify the skeleton boots**

```bash
cd /home/fsd42/dev/north-cloud/dashboard-waaseyaa
composer install
php bin/waaseyaa migrate
php bin/waaseyaa serve &
curl -s http://127.0.0.1:8080 | head -5
kill %1
```

Expected: Welcome page HTML returned

- [ ] **Step 4: Commit the scaffold**

```bash
cd /home/fsd42/dev/north-cloud
git add dashboard-waaseyaa/
git commit -m "feat(dashboard-waaseyaa): scaffold waaseyaa app from skeleton"
```

### Task 2: Customize CLAUDE.md for Dashboard Context

**Context:** The skeleton CLAUDE.md is generic (entity-focused). Replace it with dashboard-specific guidance that covers the Vue SPA, Go API integrations, and the dashboard vs Grafana split.

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/dashboard-waaseyaa/CLAUDE.md`

- [ ] **Step 1: Read the existing skeleton CLAUDE.md**

Read `/home/fsd42/dev/north-cloud/dashboard-waaseyaa/CLAUDE.md` to understand the skeleton's default content.

- [ ] **Step 2: Replace with dashboard-specific CLAUDE.md**

Key sections to include:
- **Purpose**: Operator dashboard for North Cloud content pipeline (Vue 3 SPA on Waaseyaa scaffold)
- **Architecture**: PHP serves SPA shell, Vue talks directly to Go APIs, Grafana for metrics
- **Orchestration table**: Map `frontend/src/features/*` to feature specs
- **Commands**: `npm run dev` (frontend), `php bin/waaseyaa serve` (backend shell), `npm test`, `npm run lint`
- **API services**: Port table from spec (auth:8040, source-manager:8050, crawler:8080, etc.)
- **Conventions**: Vue 3 Composition API, TypeScript strict, TanStack Query for data fetching, no `any` types
- **Reference**: Point to `docs/superpowers/specs/2026-03-24-dashboard-waaseyaa-rewrite-design.md` for full design

- [ ] **Step 3: Commit**

```bash
cd /home/fsd42/dev/north-cloud
git add dashboard-waaseyaa/CLAUDE.md
git commit -m "docs(dashboard-waaseyaa): customize CLAUDE.md for operator dashboard"
```

### Task 3: Set Up Vue 3 SPA with Vite

**Context:** The skeleton has no frontend files. Initialize a Vue 3 + TypeScript + Vite project in `frontend/`. This is the foundation everything else builds on.

**Files:**
- Create: `dashboard-waaseyaa/frontend/package.json`
- Create: `dashboard-waaseyaa/frontend/vite.config.ts`
- Create: `dashboard-waaseyaa/frontend/tsconfig.json`
- Create: `dashboard-waaseyaa/frontend/tsconfig.app.json`
- Create: `dashboard-waaseyaa/frontend/tsconfig.node.json`
- Create: `dashboard-waaseyaa/frontend/index.html`
- Create: `dashboard-waaseyaa/frontend/src/main.ts`
- Create: `dashboard-waaseyaa/frontend/src/App.vue`
- Create: `dashboard-waaseyaa/frontend/src/env.d.ts`
- Create: `dashboard-waaseyaa/frontend/tailwind.config.ts` (if needed for v4)

- [ ] **Step 1: Initialize Vite + Vue project**

```bash
cd /home/fsd42/dev/north-cloud/dashboard-waaseyaa
npm create vite@latest frontend -- --template vue-ts
```

Expected: `frontend/` directory created with Vue + TypeScript template

- [ ] **Step 2: Install core dependencies**

```bash
cd /home/fsd42/dev/north-cloud/dashboard-waaseyaa/frontend
npm install
npm install vue-router@4 @tanstack/vue-query axios pinia
npm install -D tailwindcss @tailwindcss/vite
```

Expected: All packages installed

- [ ] **Step 3: Configure Vite with Tailwind and API proxies**

Modify `frontend/vite.config.ts`:

```typescript
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath } from 'node:url'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 3002,
    proxy: {
      '/api/auth': {
        target: process.env.VITE_AUTH_URL || 'http://localhost:8040',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/auth/, '/api/v1/auth'),
      },
      '/api/sources': {
        target: process.env.VITE_SOURCE_MANAGER_URL || 'http://localhost:8050',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/sources/, '/api/v1/sources'),
      },
      '/api/crawler': {
        target: process.env.VITE_CRAWLER_URL || 'http://localhost:8080',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/crawler/, '/api/v1'),
      },
      '/api/publisher': {
        target: process.env.VITE_PUBLISHER_URL || 'http://localhost:8070',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/publisher/, '/api/v1'),
      },
      '/api/classifier': {
        target: process.env.VITE_CLASSIFIER_URL || 'http://localhost:8070',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/classifier/, '/api/v1'),
      },
      '/api/index-manager': {
        target: process.env.VITE_INDEX_MANAGER_URL || 'http://localhost:8090',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/index-manager/, '/api/v1'),
      },
      '/api/search': {
        target: process.env.VITE_SEARCH_URL || 'http://localhost:8092',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api\/search/, '/api/v1'),
      },
    },
  },
})
```

- [ ] **Step 4: Configure Tailwind v4**

Create `frontend/src/style.css`:

```css
@import "tailwindcss";
```

- [ ] **Step 5: Create minimal App.vue**

```vue
<script setup lang="ts">
import { RouterView } from 'vue-router'
</script>

<template>
  <RouterView />
</template>
```

- [ ] **Step 6: Create main.ts with Vue app setup**

```typescript
import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { VueQueryPlugin } from '@tanstack/vue-query'
import App from './App.vue'
import router from './app/router'
import './style.css'

const app = createApp(App)
app.use(createPinia())
app.use(VueQueryPlugin)
app.use(router)
app.mount('#app')
```

- [ ] **Step 7: Install ESLint + Prettier**

```bash
cd /home/fsd42/dev/north-cloud/dashboard-waaseyaa/frontend
npm install -D eslint @eslint/js typescript-eslint eslint-plugin-vue prettier
```

Create `.eslintrc.cjs` following Vue 3 + TypeScript conventions. Create `.prettierrc` with single quotes and no semicolons (or match project preference).

- [ ] **Step 8: Create .env.example**

Create `frontend/.env.example`:

```env
VITE_AUTH_URL=http://localhost:8040
VITE_SOURCE_MANAGER_URL=http://localhost:8050
VITE_CRAWLER_URL=http://localhost:8080
VITE_PUBLISHER_URL=http://localhost:8070
VITE_CLASSIFIER_URL=http://localhost:8070
VITE_INDEX_MANAGER_URL=http://localhost:8090
VITE_SEARCH_URL=http://localhost:8092
VITE_GRAFANA_URL=http://localhost:3000
```

Note: Replace the generated `App.vue` and `main.ts` from Vite template with the versions in steps 5 and 6.

- [ ] **Step 9: Verify dev server starts**

```bash
cd /home/fsd42/dev/north-cloud/dashboard-waaseyaa/frontend
npm run dev
```

Expected: Vite dev server on http://localhost:3002

- [ ] **Step 10: Commit**

```bash
cd /home/fsd42/dev/north-cloud
git add dashboard-waaseyaa/frontend/
git commit -m "feat(dashboard-waaseyaa): initialize Vue 3 + Vite + Tailwind + TanStack Query"
```

### Task 4: Shared API Client & Auth

**Context:** Set up the Axios client with JWT interceptor and the auth composable. Every feature module depends on this.

**Files:**
- Create: `dashboard-waaseyaa/frontend/src/shared/api/client.ts`
- Create: `dashboard-waaseyaa/frontend/src/shared/api/types.ts`
- Create: `dashboard-waaseyaa/frontend/src/shared/api/endpoints.ts`
- Create: `dashboard-waaseyaa/frontend/src/shared/auth/useAuth.ts`
- Create: `dashboard-waaseyaa/frontend/src/shared/auth/authGuard.ts`
- Test: `dashboard-waaseyaa/frontend/src/shared/api/__tests__/client.test.ts`
- Test: `dashboard-waaseyaa/frontend/src/shared/auth/__tests__/useAuth.test.ts`

- [ ] **Step 1: Install Vitest**

```bash
cd /home/fsd42/dev/north-cloud/dashboard-waaseyaa/frontend
npm install -D vitest @vue/test-utils happy-dom axios-mock-adapter
```

Add to `vite.config.ts`:

```typescript
// Add to defineConfig:
test: {
  environment: 'happy-dom',
},
```

- [ ] **Step 2: Write failing test for API client**

Create `frontend/src/shared/api/__tests__/client.test.ts`:

```typescript
import { describe, it, expect, vi, beforeEach } from 'vitest'

describe('apiClient', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('attaches Authorization header when token exists', async () => {
    localStorage.setItem('dashboard_token', 'test-jwt-token')
    const { apiClient } = await import('../client')

    // Make a real request and check it was configured correctly
    const { default: MockAdapter } = await import('axios-mock-adapter')
    const mock = new MockAdapter(apiClient)
    mock.onGet('/test').reply(200)

    const response = await apiClient.get('/test')
    expect(mock.history.get[0].headers?.Authorization).toBe('Bearer test-jwt-token')
    mock.restore()
  })

  it('does not attach header when no token', async () => {
    const { apiClient } = await import('../client')
    const { default: MockAdapter } = await import('axios-mock-adapter')
    const mock = new MockAdapter(apiClient)
    mock.onGet('/test').reply(200)

    await apiClient.get('/test')
    expect(mock.history.get[0].headers?.Authorization).toBeUndefined()
    mock.restore()
  })
})
```

Run: `cd /home/fsd42/dev/north-cloud/dashboard-waaseyaa/frontend && npx vitest run src/shared/api/__tests__/client.test.ts`
Expected: FAIL — module not found

- [ ] **Step 3: Implement API client**

Create `frontend/src/shared/api/types.ts`:

```typescript
export interface ApiError {
  status: number
  message: string
  details?: Record<string, unknown>
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  per_page: number
}
```

Create `frontend/src/shared/api/client.ts`:

```typescript
import axios from 'axios'

export const apiClient = axios.create({
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
})

apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('dashboard_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('dashboard_token')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  },
)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/fsd42/dev/north-cloud/dashboard-waaseyaa/frontend && npx vitest run src/shared/api/__tests__/client.test.ts`
Expected: PASS

- [ ] **Step 5: Write failing test for useAuth**

Create `frontend/src/shared/auth/__tests__/useAuth.test.ts`:

```typescript
import { describe, it, expect, beforeEach } from 'vitest'

describe('useAuth', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('isAuthenticated returns false when no token', async () => {
    const { useAuth } = await import('../useAuth')
    const { isAuthenticated } = useAuth()
    expect(isAuthenticated.value).toBe(false)
  })

  it('isAuthenticated returns true when valid token exists', async () => {
    // Create a non-expired JWT (exp = now + 1 hour)
    const payload = btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600 }))
    const fakeJwt = `header.${payload}.signature`
    localStorage.setItem('dashboard_token', fakeJwt)

    const { useAuth } = await import('../useAuth')
    const { isAuthenticated } = useAuth()
    expect(isAuthenticated.value).toBe(true)
  })

  it('logout clears the token', async () => {
    localStorage.setItem('dashboard_token', 'some-token')
    const { useAuth } = await import('../useAuth')
    const { logout } = useAuth()
    logout()
    expect(localStorage.getItem('dashboard_token')).toBeNull()
  })
})
```

Run: `cd /home/fsd42/dev/north-cloud/dashboard-waaseyaa/frontend && npx vitest run src/shared/auth/__tests__/useAuth.test.ts`
Expected: FAIL — module not found

- [ ] **Step 6: Implement useAuth**

Create `frontend/src/shared/api/endpoints.ts`:

```typescript
export const endpoints = {
  auth: {
    login: '/api/auth/login',
  },
  sources: {
    list: '/api/sources',
    detail: (id: string) => `/api/sources/${id}`,
    create: '/api/sources',
    update: (id: string) => `/api/sources/${id}`,
    delete: (id: string) => `/api/sources/${id}`,
    enable: (id: string) => `/api/sources/${id}/enable`,
    disable: (id: string) => `/api/sources/${id}/disable`,
    testCrawl: '/api/sources/test-crawl',
    fetchMetadata: '/api/sources/fetch-metadata',
  },
  crawler: {
    jobs: '/api/crawler/jobs',
    job: (id: string) => `/api/crawler/jobs/${id}`,
  },
  publisher: {
    channels: '/api/publisher/channels',
    channel: (id: string) => `/api/publisher/channels/${id}`,
  },
  classifier: {
    rules: '/api/classifier/rules',
    rule: (id: string) => `/api/classifier/rules/${id}`,
  },
  indexManager: {
    indexes: '/api/index-manager/indexes',
    index: (name: string) => `/api/index-manager/indexes/${name}`,
  },
  search: {
    content: '/api/search/feeds/latest',
    feed: (slug: string) => `/api/search/feeds/${slug}`,
  },
} as const
```

Create `frontend/src/shared/auth/useAuth.ts`:

```typescript
import { computed, ref } from 'vue'
import { apiClient } from '../api/client'
import { endpoints } from '../api/endpoints'

const TOKEN_KEY = 'dashboard_token'

// Module-level ref — shared across all useAuth() calls (singleton state)
const token = ref(localStorage.getItem(TOKEN_KEY))

function isTokenExpired(t: string): boolean {
  try {
    const payload = JSON.parse(atob(t.split('.')[1]))
    return payload.exp * 1000 < Date.now()
  } catch {
    return true
  }
}

export function useAuth() {
  const isAuthenticated = computed(() => {
    if (!token.value) return false
    return !isTokenExpired(token.value)
  })

  async function login(username: string, password: string): Promise<void> {
    const response = await apiClient.post(endpoints.auth.login, { username, password })
    token.value = response.data.token
    localStorage.setItem(TOKEN_KEY, response.data.token)
  }

  function logout(): void {
    token.value = null
    localStorage.removeItem(TOKEN_KEY)
  }

  return { isAuthenticated, token, login, logout }
}
```

- [ ] **Step 7: Implement auth guard**

Create `frontend/src/shared/auth/authGuard.ts`:

```typescript
import type { NavigationGuardWithThis } from 'vue-router'
import { useAuth } from './useAuth'

export const authGuard: NavigationGuardWithThis<undefined> = (to) => {
  const { isAuthenticated } = useAuth()

  if (to.meta.requiresAuth !== false && !isAuthenticated.value) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }

  return true
}
```

- [ ] **Step 8: Run all auth tests**

Run: `cd /home/fsd42/dev/north-cloud/dashboard-waaseyaa/frontend && npx vitest run src/shared/`
Expected: All tests PASS

- [ ] **Step 9: Commit**

```bash
cd /home/fsd42/dev/north-cloud
git add dashboard-waaseyaa/frontend/src/shared/
git commit -m "feat(dashboard-waaseyaa): add API client with JWT auth and route guard"
```

### Task 5: App Shell & Router

**Context:** Set up the sidebar layout, router with auth guard, and the navigation structure. This is the app chrome that all feature modules render inside.

**Files:**
- Create: `dashboard-waaseyaa/frontend/src/app/router.ts`
- Create: `dashboard-waaseyaa/frontend/src/layouts/AppShell.vue`
- Create: `dashboard-waaseyaa/frontend/src/layouts/Sidebar.vue`
- Create: `dashboard-waaseyaa/frontend/src/features/auth/views/LoginView.vue`

- [ ] **Step 1: Create router with auth guard**

Create `frontend/src/app/router.ts`:

```typescript
import { createRouter, createWebHistory } from 'vue-router'
import { authGuard } from '@/shared/auth/authGuard'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/features/auth/views/LoginView.vue'),
      meta: { requiresAuth: false },
    },
    {
      path: '/',
      component: () => import('@/layouts/AppShell.vue'),
      children: [
        {
          path: '',
          name: 'home',
          component: () => import('@/features/home/views/HomeView.vue'),
        },
        // Sources
        {
          path: 'sources',
          name: 'sources',
          component: () => import('@/features/sources/views/SourceList.vue'),
        },
        {
          path: 'sources/new',
          name: 'source-create',
          component: () => import('@/features/sources/views/SourceForm.vue'),
        },
        {
          path: 'sources/:id',
          name: 'source-detail',
          component: () => import('@/features/sources/views/SourceDetail.vue'),
        },
        {
          path: 'sources/:id/edit',
          name: 'source-edit',
          component: () => import('@/features/sources/views/SourceForm.vue'),
        },
        // Crawling
        {
          path: 'crawl-jobs',
          name: 'crawl-jobs',
          component: () => import('@/features/crawling/views/JobList.vue'),
        },
        {
          path: 'crawl-jobs/:id',
          name: 'crawl-job-detail',
          component: () => import('@/features/crawling/views/JobDetail.vue'),
        },
      ],
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('@/features/home/views/NotFoundView.vue'),
    },
  ],
})

router.beforeEach(authGuard)

export default router
```

- [ ] **Step 2: Create Sidebar component**

Create `frontend/src/layouts/Sidebar.vue`:

```vue
<script setup lang="ts">
import { useRoute } from 'vue-router'

const route = useRoute()

const navGroups = [
  {
    label: 'Home',
    items: [{ name: 'Pipeline Overview', route: '/' }],
  },
  {
    label: 'Content Intake',
    items: [
      { name: 'Sources', route: '/sources' },
      { name: 'Crawl Jobs', route: '/crawl-jobs' },
    ],
  },
]

function isActive(path: string): boolean {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}
</script>

<template>
  <nav class="w-56 bg-slate-950 border-r border-slate-800 flex flex-col min-h-screen">
    <div class="px-4 py-3 text-blue-500 font-bold text-lg border-b border-slate-800">
      North Cloud
    </div>
    <div v-for="group in navGroups" :key="group.label" class="mt-3">
      <div class="px-4 py-1 text-amber-500 text-xs uppercase tracking-wider">
        {{ group.label }}
      </div>
      <RouterLink
        v-for="item in group.items"
        :key="item.route"
        :to="item.route"
        class="block px-4 py-2 text-sm transition-colors"
        :class="isActive(item.route)
          ? 'text-slate-100 bg-slate-800 border-l-3 border-blue-500'
          : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900'"
      >
        {{ item.name }}
      </RouterLink>
    </div>
  </nav>
</template>
```

- [ ] **Step 3: Create AppShell layout**

Create `frontend/src/layouts/AppShell.vue`:

```vue
<script setup lang="ts">
import Sidebar from './Sidebar.vue'
</script>

<template>
  <div class="flex min-h-screen bg-slate-950 text-slate-100">
    <Sidebar />
    <main class="flex-1 p-6 overflow-auto">
      <RouterView />
    </main>
  </div>
</template>
```

- [ ] **Step 4: Create LoginView placeholder**

Create `frontend/src/features/auth/views/LoginView.vue`:

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuth } from '@/shared/auth/useAuth'

const { login } = useAuth()
const router = useRouter()
const route = useRoute()

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function handleLogin() {
  error.value = ''
  loading.value = true
  try {
    await login(username.value, password.value)
    const redirect = (route.query.redirect as string) || '/'
    router.push(redirect)
  } catch (e) {
    error.value = 'Invalid credentials'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen bg-slate-950 flex items-center justify-center">
    <form @submit.prevent="handleLogin" class="w-80 space-y-4">
      <h1 class="text-2xl font-bold text-slate-100 text-center">North Cloud</h1>
      <div v-if="error" class="bg-red-900/50 text-red-300 p-3 rounded text-sm">{{ error }}</div>
      <input
        v-model="username"
        type="text"
        placeholder="Username"
        class="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded text-slate-100 placeholder-slate-500"
      />
      <input
        v-model="password"
        type="password"
        placeholder="Password"
        class="w-full px-3 py-2 bg-slate-800 border border-slate-700 rounded text-slate-100 placeholder-slate-500"
      />
      <button
        type="submit"
        :disabled="loading"
        class="w-full py-2 bg-blue-600 hover:bg-blue-500 text-white rounded font-medium disabled:opacity-50"
      >
        {{ loading ? 'Signing in...' : 'Sign In' }}
      </button>
    </form>
  </div>
</template>
```

- [ ] **Step 5: Create NotFoundView**

Create `frontend/src/features/home/views/NotFoundView.vue`:

```vue
<template>
  <div class="flex flex-col items-center justify-center min-h-[60vh]">
    <h1 class="text-4xl font-bold text-slate-400 mb-4">404</h1>
    <p class="text-slate-500 mb-6">Page not found</p>
    <RouterLink to="/" class="text-blue-500 hover:text-blue-400">Back to dashboard</RouterLink>
  </div>
</template>
```

- [ ] **Step 6: Create HomeView placeholder**

Create `frontend/src/features/home/views/HomeView.vue`:

```vue
<template>
  <div>
    <h1 class="text-2xl font-bold mb-6">Pipeline Overview</h1>
    <p class="text-slate-400">Dashboard home — stats and quick actions coming in next tasks.</p>
  </div>
</template>
```

- [ ] **Step 6: Verify app loads with sidebar navigation**

```bash
cd /home/fsd42/dev/north-cloud/dashboard-waaseyaa/frontend
npm run dev
```

Open http://localhost:3002 — should redirect to /login. After login (if auth service running) or bypass guard temporarily, should see sidebar + home view.

- [ ] **Step 7: Commit**

```bash
cd /home/fsd42/dev/north-cloud
git add dashboard-waaseyaa/frontend/src/app/ dashboard-waaseyaa/frontend/src/layouts/ dashboard-waaseyaa/frontend/src/features/auth/ dashboard-waaseyaa/frontend/src/features/home/
git commit -m "feat(dashboard-waaseyaa): add app shell with sidebar navigation, router, and login"
```

---

## Phase 1: Feature Modules

### Task 6: Shared UI Components

**Context:** Build the reusable components that feature modules depend on: DataTable, StatusBadge, ErrorBanner, LoadingSkeleton, ConfirmDialog, GrafanaEmbed.

**Files:**
- Create: `dashboard-waaseyaa/frontend/src/shared/components/DataTable.vue`
- Create: `dashboard-waaseyaa/frontend/src/shared/components/StatusBadge.vue`
- Create: `dashboard-waaseyaa/frontend/src/shared/components/ErrorBanner.vue`
- Create: `dashboard-waaseyaa/frontend/src/shared/components/LoadingSkeleton.vue`
- Create: `dashboard-waaseyaa/frontend/src/shared/components/ConfirmDialog.vue`
- Create: `dashboard-waaseyaa/frontend/src/shared/components/BulkActionBar.vue`
- Create: `dashboard-waaseyaa/frontend/src/shared/components/GrafanaEmbed.vue`
- Create: `dashboard-waaseyaa/frontend/src/shared/composables/usePagination.ts`
- Create: `dashboard-waaseyaa/frontend/src/shared/composables/useToast.ts`
- Test: `dashboard-waaseyaa/frontend/src/shared/components/__tests__/DataTable.test.ts`
- Test: `dashboard-waaseyaa/frontend/src/shared/components/__tests__/StatusBadge.test.ts`

- [ ] **Step 1: Write failing test for DataTable**

Test that DataTable renders columns and rows from props, emits sort events.

- [ ] **Step 2: Implement DataTable**

Generic sortable, paginated table. Props: `columns: Column[]`, `rows: T[]`, `loading: boolean`, `total: number`. Emits: `sort`, `page-change`. Uses `usePagination` composable for offset/limit state.

- [ ] **Step 3: Write failing test for StatusBadge**

Test that StatusBadge renders correct color/text for known statuses (active, paused, error, pending).

- [ ] **Step 4: Implement StatusBadge**

Simple component: `<StatusBadge status="active" />` renders a colored pill.

- [ ] **Step 5: Write failing tests for ErrorBanner and LoadingSkeleton**

Test `ErrorBanner`: renders message, emits `retry` on button click.
Test `LoadingSkeleton`: renders correct number of animated placeholder lines from `lines` prop.

- [ ] **Step 6: Implement ErrorBanner, LoadingSkeleton, ConfirmDialog, BulkActionBar**

These are simple UI primitives:
- `ErrorBanner` — red banner with message + retry button, emits `retry`
- `LoadingSkeleton` — animated placeholder blocks, prop `lines: number`
- `ConfirmDialog` — modal with title, message, confirm/cancel buttons, emits `confirm`/`cancel`
- `BulkActionBar` — sticky bottom bar showing selected count + action buttons

- [ ] **Step 7: Implement GrafanaEmbed**

```vue
<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  panelId: string
  vars?: Record<string, string>
  height?: string
}>()

const grafanaUrl = import.meta.env.VITE_GRAFANA_URL || 'http://localhost:3000'

const src = computed(() => {
  const params = new URLSearchParams({ theme: 'dark' })
  if (props.vars) {
    for (const [key, value] of Object.entries(props.vars)) {
      params.set(`var-${key}`, value)
    }
  }
  return `${grafanaUrl}/d-solo/${props.panelId}?${params}`
})
</script>

<template>
  <iframe
    :src="src"
    :style="{ height: height || '300px' }"
    class="w-full border border-slate-700 rounded-lg"
    frameborder="0"
  />
</template>
```

- [ ] **Step 8: Implement usePagination and useToast composables**

- `usePagination(defaultPerPage = 20)` — manages `page`, `perPage`, `offset` as refs, syncs with URL query params
- `useToast()` — reactive queue of toast notifications, `show(message, type)`, auto-dismiss after 5s

- [ ] **Step 9: Run all shared component tests**

Run: `cd /home/fsd42/dev/north-cloud/dashboard-waaseyaa/frontend && npx vitest run src/shared/`
Expected: All PASS

- [ ] **Step 10: Commit**

```bash
cd /home/fsd42/dev/north-cloud
git add dashboard-waaseyaa/frontend/src/shared/
git commit -m "feat(dashboard-waaseyaa): add shared UI components and composables"
```

### Task 7: Sources Feature Module

**Context:** The highest-value feature — source CRUD, test-crawl, enable/disable. Talks to source-manager at :8050.

**Reference:** Source Manager API — `GET/POST /api/v1/sources`, `GET/PUT/DELETE /api/v1/sources/:id`, `POST /api/v1/sources/test-crawl`, `POST /api/v1/sources/fetch-metadata`, `PATCH /api/v1/sources/:id/enable`, `PATCH /api/v1/sources/:id/disable`.

**Files:**
- Create: `dashboard-waaseyaa/frontend/src/features/sources/types.ts`
- Create: `dashboard-waaseyaa/frontend/src/features/sources/composables/useSourceApi.ts`
- Create: `dashboard-waaseyaa/frontend/src/features/sources/views/SourceList.vue`
- Create: `dashboard-waaseyaa/frontend/src/features/sources/views/SourceDetail.vue`
- Create: `dashboard-waaseyaa/frontend/src/features/sources/views/SourceForm.vue`
- Create: `dashboard-waaseyaa/frontend/src/features/sources/components/TestCrawlDialog.vue`
- Create: `dashboard-waaseyaa/frontend/src/features/sources/components/MetadataPreview.vue`
- Create: `dashboard-waaseyaa/frontend/src/features/sources/index.ts`
- Test: `dashboard-waaseyaa/frontend/src/features/sources/__tests__/useSourceApi.test.ts`

- [ ] **Step 1: Define types**

Create `types.ts` with `Source`, `SourceForm`, `SourceStatus`, `TestCrawlResult`, `SourceMetadata` interfaces. Check source-manager handler/model files for the actual JSON shape.

Reference: `/home/fsd42/dev/north-cloud/source-manager/internal/handler/` and `/home/fsd42/dev/north-cloud/source-manager/internal/model/`

- [ ] **Step 2: Write failing test for useSourceApi**

Test that `useSourceApi().listSources()` calls the correct endpoint and returns typed data. Use `vi.mock('axios')` or MSW.

- [ ] **Step 3: Implement useSourceApi composable**

Wraps TanStack Query hooks:
- `useSourceList(params)` — `useQuery` on `GET /api/sources`
- `useSource(id)` — `useQuery` on `GET /api/sources/:id`
- `useCreateSource()` — `useMutation` on `POST /api/sources`
- `useUpdateSource()` — `useMutation` on `PUT /api/sources/:id`
- `useDeleteSource()` — `useMutation` on `DELETE /api/sources/:id`
- `useToggleSource()` — `useMutation` on `PATCH /api/sources/:id/enable` or `/disable`
- `useTestCrawl()` — `useMutation` on `POST /api/sources/test-crawl`

- [ ] **Step 4: Run tests**

Run: `npx vitest run src/features/sources/`
Expected: PASS

- [ ] **Step 5: Implement SourceList view**

Uses `DataTable` with columns: Name, URL, Status, Last Crawled, Actions. Status uses `StatusBadge`. Actions: Edit, Enable/Disable, Delete (with ConfirmDialog). "Add Source" button in header.

- [ ] **Step 6: Implement SourceDetail view**

Shows source metadata, crawl history, embedded Grafana panel for that source's crawl success rate. Action buttons: Edit, Test Crawl, Enable/Disable.

- [ ] **Step 7: Implement SourceForm view**

Create/edit form. Fields from source-manager model: name, url, crawl_interval, category, enabled. On submit: calls create or update mutation, navigates to detail view. Includes "Fetch Metadata" button that previews what the crawler would find.

- [ ] **Step 8: Implement TestCrawlDialog**

Modal triggered from SourceDetail. Calls `POST /api/sources/test-crawl` with source URL. Shows results: articles found, sample titles, any errors. Uses LoadingSkeleton while waiting.

- [ ] **Step 9: Export routes from index.ts**

```typescript
export const sourceRoutes = [
  { path: 'sources', name: 'sources', component: () => import('./views/SourceList.vue') },
  { path: 'sources/new', name: 'source-create', component: () => import('./views/SourceForm.vue') },
  { path: 'sources/:id', name: 'source-detail', component: () => import('./views/SourceDetail.vue') },
  { path: 'sources/:id/edit', name: 'source-edit', component: () => import('./views/SourceForm.vue') },
]
```

- [ ] **Step 10: Commit**

```bash
git add dashboard-waaseyaa/frontend/src/features/sources/
git commit -m "feat(dashboard-waaseyaa): add sources feature module with CRUD and test-crawl"
```

### Task 8: Crawling Feature Module

**Context:** Crawl job management — list jobs, start/schedule new crawls, pause/cancel, view job detail with logs.

**Reference:** Crawler API — jobs are managed via the MCP server tools or direct API. Check `/home/fsd42/dev/north-cloud/crawler/internal/handler/` for endpoints. Key endpoints: `GET /api/v1/jobs`, `POST /api/v1/jobs`, `GET /api/v1/jobs/:id`, `PATCH /api/v1/jobs/:id` (pause/resume/cancel).

**Files:**
- Create: `dashboard-waaseyaa/frontend/src/features/crawling/types.ts`
- Create: `dashboard-waaseyaa/frontend/src/features/crawling/composables/useCrawlApi.ts`
- Create: `dashboard-waaseyaa/frontend/src/features/crawling/views/JobList.vue`
- Create: `dashboard-waaseyaa/frontend/src/features/crawling/views/JobDetail.vue`
- Create: `dashboard-waaseyaa/frontend/src/features/crawling/components/StartCrawlDialog.vue`
- Create: `dashboard-waaseyaa/frontend/src/features/crawling/index.ts`
- Test: `dashboard-waaseyaa/frontend/src/features/crawling/__tests__/useCrawlApi.test.ts`

- [ ] **Step 1: Define types**

Read crawler handler/model files to determine the JSON shape for crawl jobs. Create interfaces: `CrawlJob`, `CrawlJobStatus`, `StartCrawlRequest`, `ScheduleCrawlRequest`.

Reference: `/home/fsd42/dev/north-cloud/crawler/internal/handler/` and `/home/fsd42/dev/north-cloud/crawler/internal/model/`

- [ ] **Step 2: Write failing test for useCrawlApi**

Test that `useCrawlJobs()` calls `GET /api/crawler/jobs` and returns typed data. Test that `useStartCrawl()` calls POST with the correct payload. Use `axios-mock-adapter`.

- [ ] **Step 3: Implement useCrawlApi composable**

TanStack Query wrappers:
- `useCrawlJobs()` — `useQuery` with `refetchInterval: 10000` (auto-refresh every 10s)
- `useCrawlJob(id)` — `useQuery` with `refetchInterval: 5000`
- `useStartCrawl()` — `useMutation`
- `useControlJob()` — `useMutation` for pause/resume/cancel

- [ ] **Step 4: Run tests**

Expected: PASS

- [ ] **Step 5: Implement JobList view**

DataTable with columns: Source, Status, Started, Articles Found, Duration, Actions. Status badges for running/paused/completed/failed. Auto-refreshes via TanStack Query refetchInterval. "Start Crawl" button opens dialog.

- [ ] **Step 6: Implement JobDetail view**

Shows full job info: source, status, start/end times, articles found, errors. Embedded Grafana panel for crawler throughput. Action buttons: Pause/Resume, Cancel. If available, show real-time log stream (SSE from crawler).

- [ ] **Step 7: Implement StartCrawlDialog**

Modal with source selector (dropdown of active sources), option for one-off vs scheduled. Calls start_crawl or schedule_crawl.

- [ ] **Step 8: Commit**

```bash
git add dashboard-waaseyaa/frontend/src/features/crawling/
git commit -m "feat(dashboard-waaseyaa): add crawling feature module with job management"
```

### Task 9: Home Page with Stats & Grafana

**Context:** The pipeline overview page — quick stats, embedded Grafana panel, quick action buttons.

**Files:**
- Modify: `dashboard-waaseyaa/frontend/src/features/home/views/HomeView.vue`
- Create: `dashboard-waaseyaa/frontend/src/features/home/composables/useHomeStats.ts`
- Create: `dashboard-waaseyaa/frontend/src/features/home/components/StatCard.vue`
- Create: `dashboard-waaseyaa/frontend/src/features/home/components/QuickActions.vue`

- [ ] **Step 1: Write failing test for useHomeStats**

Test that the composable fires parallel queries and returns structured stat data. Mock all 4 API calls with `axios-mock-adapter`.

- [ ] **Step 2: Implement useHomeStats composable**

Parallel queries to multiple services:
- Source count from `GET /api/sources?per_page=1` (use total from pagination)
- Active crawl jobs from `GET /api/crawler/jobs?status=running`
- Pending verification from `GET /api/sources/verification/stats` — **Note:** this endpoint is Phase 3 scope. If it returns an error, the StatCard should gracefully show "N/A". Use TanStack Query's `retry: false` for this query.
- Channel count from `GET /api/publisher/channels`

All via TanStack Query `useQueries`.

- [ ] **Step 3: Implement StatCard component**

```vue
<script setup lang="ts">
defineProps<{
  label: string
  value: number | string
  subtitle?: string
  subtitleColor?: 'green' | 'amber' | 'red'
}>()
</script>
```

Renders as a dark card with label (uppercase small text), large value, and optional colored subtitle.

- [ ] **Step 4: Implement QuickActions component**

Row of buttons: "Add Source" (→ /sources/new), "Start Crawl" (opens StartCrawlDialog from crawling feature — **cross-feature import**: `import StartCrawlDialog from '@/features/crawling/components/StartCrawlDialog.vue'`), "Review Queue" (→ /verification, future phase), "Open Grafana" (external link).

- [ ] **Step 5: Update HomeView with stats + Grafana embed + quick actions**

Layout:
1. Header: "Pipeline Overview" + time range selector
2. 4x StatCard grid
3. GrafanaEmbed with pipeline overview panel
4. QuickActions row

- [ ] **Step 6: Verify visually**

Start dev server, login, check home page renders stats (may show 0s if Go services aren't running — that's fine, ErrorBanner should show for failed fetches).

- [ ] **Step 7: Run useHomeStats tests**

Run: `npx vitest run src/features/home/`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add dashboard-waaseyaa/frontend/src/features/home/
git commit -m "feat(dashboard-waaseyaa): add home page with stats, Grafana embed, and quick actions"
```

### Task 10: Integration Test & Cleanup

**Note on Docker/nginx:** The new dashboard runs on port 3002 (dev). Docker/nginx integration for production deployment is deferred — it will be addressed when Phase 2 completes and we're ready to swap the old dashboard. For now, both dashboards can run in parallel on different ports.

**Context:** Verify the full app works end-to-end, clean up any rough edges, add Playwright for critical path.

**Files:**
- Create: `dashboard-waaseyaa/frontend/e2e/login.spec.ts`
- Create: `dashboard-waaseyaa/frontend/playwright.config.ts`
- Modify: `dashboard-waaseyaa/frontend/package.json` (add e2e scripts)

- [ ] **Step 1: Install Playwright**

```bash
cd /home/fsd42/dev/north-cloud/dashboard-waaseyaa/frontend
npm install -D @playwright/test
npx playwright install chromium
```

- [ ] **Step 2: Create Playwright config**

```typescript
import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  webServer: {
    command: 'npm run dev',
    port: 3002,
    reuseExistingServer: true,
  },
  use: {
    baseURL: 'http://localhost:3002',
  },
})
```

- [ ] **Step 3: Write login e2e test**

```typescript
import { test, expect } from '@playwright/test'

test('unauthenticated user is redirected to login', async ({ page }) => {
  await page.goto('/')
  await expect(page).toHaveURL('/login?redirect=%2F')
  await expect(page.locator('h1')).toHaveText('North Cloud')
})

test('login form submits and redirects to home', async ({ page }) => {
  // This test requires auth service running — skip in CI if not available
  await page.goto('/login')
  await page.fill('input[type="text"]', 'admin')
  await page.fill('input[type="password"]', 'test')
  await page.click('button[type="submit"]')
  // If auth service is down, expect error message
  // If up, expect redirect to home
})
```

- [ ] **Step 4: Run Playwright tests**

```bash
cd /home/fsd42/dev/north-cloud/dashboard-waaseyaa/frontend
npx playwright test
```

Expected: Redirect test passes. Login test may skip if auth service isn't running.

- [ ] **Step 5: Run all unit tests**

```bash
npx vitest run
```

Expected: All PASS

- [ ] **Step 6: Run linting**

```bash
npx vue-tsc --noEmit
npm run lint
```

Expected: No errors

- [ ] **Step 7: Final commit**

```bash
cd /home/fsd42/dev/north-cloud
git add dashboard-waaseyaa/
git commit -m "feat(dashboard-waaseyaa): add Playwright e2e tests and Phase 1 cleanup"
```

---

## Post-Phase 1

### Task 11: Waaseyaa Framework Changes & Release

**Context:** After completing Phase 1, document any framework changes needed and tag a new Waaseyaa release.

- [ ] **Step 1: Document framework gaps found during scaffolding**

Create an issue in the Waaseyaa repo for each gap:
- Skeleton lacks `frontend/` convention for SPA apps
- Getting-started docs assume entity-focused apps
- Any other issues discovered

- [ ] **Step 2: Update getting-started docs if process had rough edges**

Check https://waaseyaa.org/getting-started against our actual experience. Submit fixes.

- [ ] **Step 3: Tag new Waaseyaa release**

```bash
cd /home/fsd42/dev/waaseyaa
git tag -a v0.1.0-alpha.54 -m "feat: drift detection wiring, SPA app support improvements"
git push origin v0.1.0-alpha.54
```

- [ ] **Step 4: Update dashboard-waaseyaa composer.json to pin new version**

```bash
cd /home/fsd42/dev/north-cloud/dashboard-waaseyaa
composer update waaseyaa/waaseyaa
git add composer.json composer.lock
git commit -m "chore(dashboard-waaseyaa): update waaseyaa to latest release"
```
