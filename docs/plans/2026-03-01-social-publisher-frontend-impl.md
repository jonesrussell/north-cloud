# Social Publisher Frontend Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add three social publishing management pages (content list, accounts CRUD, publish form) to the North Cloud dashboard under the Distribution section.

**Architecture:** Feature module pattern at `features/social-publishing/` with composables wrapping `useServerPaginatedTable`. Domain components at `components/domain/social-publishing/`. Views at `views/distribution/`. New axios client for the social-publisher backend (port 8078) proxied through Vite dev server.

**Tech Stack:** Vue 3 Composition API, TypeScript (strict, no `any`), TanStack Vue Query 5, Tailwind CSS 4, Lucide Vue Next icons, Radix Vue primitives (via `components/ui/`), Vue Sonner toasts.

**Design Doc:** `docs/plans/2026-03-01-social-publisher-frontend-design.md`

---

## Conventions (for workers)

- **No `any` types** — use `unknown` for truly unknown, specific interfaces otherwise
- **`<script setup lang="ts">`** for all components
- **`@` path alias** resolves to `src/`
- **Tailwind CSS 4** — `@import "tailwindcss"` (NOT `@tailwind` directives)
- **Auth token key**: `dashboard_token` in localStorage
- **Router base**: `/dashboard/` — all routes are relative to this
- **Pagination pattern**: `useServerPaginatedTable<T, F>` with `FetchParams<F>` → `PaginatedResponse<T>`
- **API response shape**: `{ items: T[], count: number, total: number, offset: number, limit: number }`
- **Icons**: Import from `lucide-vue-next`
- **UI primitives**: `@/components/ui/{button,card,badge,input,skeleton}`
- **Common components**: `@/components/common/{DataTablePagination,SortableColumnHeader}`
- **Formatting**: `formatDate`, `formatDateShort`, `formatRelativeTime` from `@/lib/utils`
- **Toast**: `import { toast } from 'vue-sonner'`
- **Mutations**: `useMutation` from `@tanstack/vue-query`

**Verification after each task:**
```bash
cd dashboard && npx tsc --noEmit
cd dashboard && npm run lint
```

---

## Task 1: Types

**Files:**
- Create: `dashboard/src/types/socialPublisher.ts`

**Step 1: Create the types file**

```typescript
// Social Publisher API Types

// ============================================================================
// Content
// ============================================================================

export interface DeliverySummary {
  total: number
  pending: number
  delivered: number
  failed: number
  retrying: number
}

export interface SocialContent {
  id: string
  type: string
  title: string
  summary: string
  url: string
  project: string
  source: string
  published: boolean
  scheduled_at?: string
  created_at: string
  delivery_summary?: DeliverySummary
}

export interface ContentListResponse {
  items: SocialContent[]
  count: number
  total: number
  offset: number
  limit: number
}

// ============================================================================
// Accounts
// ============================================================================

export interface SocialAccount {
  id: string
  name: string
  platform: string
  project: string
  enabled: boolean
  credentials_configured: boolean
  token_expiry?: string
  created_at: string
  updated_at: string
}

export interface CreateAccountRequest {
  name: string
  platform: string
  project: string
  enabled?: boolean
  credentials?: Record<string, unknown>
  token_expiry?: string
}

export interface UpdateAccountRequest {
  name?: string
  platform?: string
  project?: string
  enabled?: boolean
  credentials?: Record<string, unknown>
  token_expiry?: string
}

export interface AccountsListResponse {
  items: SocialAccount[]
  count: number
}

// ============================================================================
// Deliveries
// ============================================================================

export interface Delivery {
  id: string
  content_id: string
  platform: string
  account: string
  status: string
  attempts: number
  max_attempts: number
  error?: string
  platform_id?: string
  platform_url?: string
  delivered_at?: string
  created_at: string
}

// ============================================================================
// Publishing
// ============================================================================

export interface TargetConfig {
  platform: string
  account: string
}

export interface PublishRequest {
  type: string
  title?: string
  body?: string
  summary?: string
  url?: string
  images?: string[]
  tags?: string[]
  project?: string
  targets?: TargetConfig[]
  scheduled_at?: string
  metadata?: Record<string, string>
  source?: string
}

// ============================================================================
// Filters
// ============================================================================

export interface ContentFilters {
  status?: string
  type?: string
}

export interface AccountFilters {
  platform?: string
}
```

**Step 2: Verify**

Run: `cd dashboard && npx tsc --noEmit`
Expected: No errors

**Step 3: Commit**

```bash
git add dashboard/src/types/socialPublisher.ts
git commit -m "feat(dashboard): add social publisher TypeScript types"
```

---

## Task 2: Vite Proxy + API Client

**Files:**
- Modify: `dashboard/vite.config.ts`
- Modify: `dashboard/src/api/client.ts`

**Step 1: Add Vite dev proxy for social-publisher**

In `dashboard/vite.config.ts`, add at line 17 (after `CLICK_TRACKER_API_URL`):

```typescript
const SOCIAL_PUBLISHER_API_URL = process.env.SOCIAL_PUBLISHER_API_URL || 'http://localhost:8078'
```

Then in the `proxy` object, add a new entry after the click-tracker health proxy (before the auth section, around line 216):

```typescript
      // Social Publisher API proxy
      '/api/social-publisher': {
        target: SOCIAL_PUBLISHER_API_URL,
        changeOrigin: true,
        timeout: 30000,
        proxyTimeout: 30000,
        rewrite: (path) => path.replace(/^\/api\/social-publisher/, '/api/v1'),
        configure: (proxy, _options) => {
          proxy.on('proxyReq', (proxyReq, req, _res) => {
            const authHeader = req.headers.authorization || req.headers.Authorization
            if (authHeader) {
              proxyReq.setHeader('Authorization', authHeader)
            }
          })
        },
      },
      // Social Publisher health endpoint
      '/api/health/social-publisher': {
        target: SOCIAL_PUBLISHER_API_URL,
        changeOrigin: true,
        timeout: 10000,
        proxyTimeout: 10000,
        rewrite: () => '/health',
      },
```

**Step 2: Add social publisher axios instance and API object**

In `dashboard/src/api/client.ts`:

1. Add imports at the top (after the aggregation imports):

```typescript
import type {
  SocialContent,
  SocialAccount,
  ContentListResponse,
  AccountsListResponse,
  Delivery,
  CreateAccountRequest,
  UpdateAccountRequest,
  PublishRequest,
} from '../types/socialPublisher'
```

2. Add the axios client instance after `indexManagerClient` (after line 104):

```typescript
const socialPublisherClient: AxiosInstance = axios.create({
  baseURL: '/api/social-publisher',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
})
```

3. Add auth and debug interceptors (after the existing `addAuthInterceptor` and `addInterceptors` blocks):

```typescript
addAuthInterceptor(socialPublisherClient)
```

```typescript
addInterceptors(socialPublisherClient, 'SocialPublisher')
```

4. Add the API object (after `indexManagerApi`, before the default export):

```typescript
// Social Publisher API
export const socialPublisherApi = {
  content: {
    list: (params?: {
      limit?: number
      offset?: number
      status?: string
      type?: string
    }): Promise<AxiosResponse<ContentListResponse>> =>
      socialPublisherClient.get('/content', { params }),
    status: (id: string): Promise<AxiosResponse<{ deliveries: Delivery[] }>> =>
      socialPublisherClient.get(`/status/${id}`),
    publish: (data: PublishRequest): Promise<AxiosResponse<SocialContent>> =>
      socialPublisherClient.post('/publish', data),
    retry: (id: string): Promise<AxiosResponse<Delivery>> =>
      socialPublisherClient.post(`/retry/${id}`),
  },
  accounts: {
    list: (): Promise<AxiosResponse<AccountsListResponse>> =>
      socialPublisherClient.get('/accounts'),
    get: (id: string): Promise<AxiosResponse<SocialAccount>> =>
      socialPublisherClient.get(`/accounts/${id}`),
    create: (data: CreateAccountRequest): Promise<AxiosResponse<SocialAccount>> =>
      socialPublisherClient.post('/accounts', data),
    update: (id: string, data: UpdateAccountRequest): Promise<AxiosResponse<SocialAccount>> =>
      socialPublisherClient.put(`/accounts/${id}`, data),
    delete: (id: string): Promise<AxiosResponse<void>> =>
      socialPublisherClient.delete(`/accounts/${id}`),
  },
}
```

5. Update the default export to include the new API:

```typescript
export default {
  crawler: crawlerApi,
  sources: sourcesApi,
  publisher: publisherApi,
  classifier: classifierApi,
  indexManager: indexManagerApi,
  socialPublisher: socialPublisherApi,
}
```

**Step 3: Verify**

Run: `cd dashboard && npx tsc --noEmit && npm run lint`
Expected: No errors

**Step 4: Commit**

```bash
git add dashboard/vite.config.ts dashboard/src/api/client.ts
git commit -m "feat(dashboard): add social publisher API client and Vite proxy"
```

---

## Task 3: Feature Module — API + Composables

**Files:**
- Create: `dashboard/src/features/social-publishing/api/socialPublisher.ts`
- Create: `dashboard/src/features/social-publishing/api/index.ts`
- Create: `dashboard/src/features/social-publishing/composables/useContentTable.ts`
- Create: `dashboard/src/features/social-publishing/composables/useAccountsTable.ts`
- Create: `dashboard/src/features/social-publishing/composables/index.ts`
- Create: `dashboard/src/features/social-publishing/index.ts`

**Step 1: Create the feature API module**

`dashboard/src/features/social-publishing/api/socialPublisher.ts`:

```typescript
/**
 * Social Publisher API Functions
 *
 * Query key factory and fetch functions for the Social Publishing domain.
 */

import { socialPublisherApi } from '@/api/client'
import type { FetchParams, PaginatedResponse } from '@/types/table'
import type {
  SocialContent,
  SocialAccount,
  ContentFilters,
  AccountFilters,
} from '@/types/socialPublisher'

// ============================================================================
// Query Key Factory
// ============================================================================

export const socialPublisherKeys = {
  all: ['social-publisher'] as const,
  content: () => [...socialPublisherKeys.all, 'content'] as const,
  contentList: (filters?: ContentFilters) => [...socialPublisherKeys.content(), 'list', filters] as const,
  contentStatus: (id: string) => [...socialPublisherKeys.content(), 'status', id] as const,
  accounts: () => [...socialPublisherKeys.all, 'accounts'] as const,
  accountsList: () => [...socialPublisherKeys.accounts(), 'list'] as const,
  accountDetail: (id: string) => [...socialPublisherKeys.accounts(), 'detail', id] as const,
}

// ============================================================================
// Fetch Functions
// ============================================================================

/** Fetch content with pagination and filters — for ContentTable. */
export async function fetchContentPaginated(
  params: FetchParams<ContentFilters>
): Promise<PaginatedResponse<SocialContent>> {
  const queryParams: Record<string, string | number> = {
    limit: params.limit,
    offset: params.offset,
  }
  if (params.filters?.status) {
    queryParams.status = params.filters.status
  }
  if (params.filters?.type) {
    queryParams.type = params.filters.type
  }

  const response = await socialPublisherApi.content.list(queryParams)
  return {
    items: response.data?.items ?? [],
    total: response.data?.total ?? 0,
  }
}

/** Fetch accounts list — for AccountsTable. */
export async function fetchAccountsPaginated(
  _params: FetchParams<AccountFilters>
): Promise<PaginatedResponse<SocialAccount>> {
  const response = await socialPublisherApi.accounts.list()
  return {
    items: response.data?.items ?? [],
    total: response.data?.count ?? 0,
  }
}
```

`dashboard/src/features/social-publishing/api/index.ts`:

```typescript
export {
  socialPublisherKeys,
  fetchContentPaginated,
  fetchAccountsPaginated,
} from './socialPublisher'
```

**Step 2: Create the content table composable**

`dashboard/src/features/social-publishing/composables/useContentTable.ts`:

```typescript
/**
 * useContentTable - Server-paginated content for the Social Content table view.
 */

import { computed } from 'vue'
import { useServerPaginatedTable } from '@/composables/useServerPaginatedTable'
import { fetchContentPaginated } from '../api/socialPublisher'
import type { SocialContent, ContentFilters } from '@/types/socialPublisher'

const CONTENT_SORT_FIELDS = ['created_at', 'type', 'title', 'source']

export function useContentTable() {
  const table = useServerPaginatedTable<SocialContent, ContentFilters>({
    fetchFn: fetchContentPaginated,
    queryKeyPrefix: 'social-content',
    defaultLimit: 25,
    defaultSortBy: 'created_at',
    defaultSortOrder: 'desc',
    allowedSortFields: CONTENT_SORT_FIELDS,
    allowedPageSizes: [10, 25, 50, 100],
  })

  const hasActiveFilters = computed(() => {
    const f = table.filters.value
    return !!(f.status || f.type)
  })

  const activeFilterCount = computed(() => {
    const f = table.filters.value
    let count = 0
    if (f.status) count++
    if (f.type) count++
    return count
  })

  return {
    items: table.items,
    total: table.total,
    isLoading: table.isLoading,
    isFetching: table.isRefetching,
    error: table.error,
    hasError: table.hasError,

    page: table.page,
    pageSize: table.pageSize,
    totalPages: table.totalPages,
    allowedPageSizes: table.allowedPageSizes,
    setPage: table.setPage,
    setPageSize: table.setPageSize,

    sortBy: table.sortBy,
    sortOrder: table.sortOrder,
    toggleSort: table.toggleSort,

    filters: table.filters,
    setFilter: (key: keyof ContentFilters, value: ContentFilters[keyof ContentFilters]) => {
      table.setFilters({ [key]: value } as Partial<ContentFilters>)
    },
    clearFilters: table.clearFilters,
    hasActiveFilters,
    activeFilterCount,

    refetch: table.refetch,
  }
}
```

**Step 3: Create the accounts table composable**

`dashboard/src/features/social-publishing/composables/useAccountsTable.ts`:

```typescript
/**
 * useAccountsTable - Server-paginated accounts for the Social Accounts table view.
 */

import { computed } from 'vue'
import { useServerPaginatedTable } from '@/composables/useServerPaginatedTable'
import { fetchAccountsPaginated } from '../api/socialPublisher'
import type { SocialAccount, AccountFilters } from '@/types/socialPublisher'

const ACCOUNTS_SORT_FIELDS = ['name', 'platform', 'project', 'created_at']

export function useAccountsTable() {
  const table = useServerPaginatedTable<SocialAccount, AccountFilters>({
    fetchFn: fetchAccountsPaginated,
    queryKeyPrefix: 'social-accounts',
    defaultLimit: 25,
    defaultSortBy: 'name',
    defaultSortOrder: 'asc',
    allowedSortFields: ACCOUNTS_SORT_FIELDS,
    allowedPageSizes: [10, 25, 50, 100],
  })

  const hasActiveFilters = computed(() => {
    const f = table.filters.value
    return !!f.platform
  })

  const activeFilterCount = computed(() => {
    const f = table.filters.value
    let count = 0
    if (f.platform) count++
    return count
  })

  return {
    items: table.items,
    total: table.total,
    isLoading: table.isLoading,
    isFetching: table.isRefetching,
    error: table.error,
    hasError: table.hasError,

    page: table.page,
    pageSize: table.pageSize,
    totalPages: table.totalPages,
    allowedPageSizes: table.allowedPageSizes,
    setPage: table.setPage,
    setPageSize: table.setPageSize,

    sortBy: table.sortBy,
    sortOrder: table.sortOrder,
    toggleSort: table.toggleSort,

    filters: table.filters,
    setFilter: (key: keyof AccountFilters, value: AccountFilters[keyof AccountFilters]) => {
      table.setFilters({ [key]: value } as Partial<AccountFilters>)
    },
    clearFilters: table.clearFilters,
    hasActiveFilters,
    activeFilterCount,

    refetch: table.refetch,
  }
}
```

**Step 4: Create barrel exports**

`dashboard/src/features/social-publishing/composables/index.ts`:

```typescript
export { useContentTable } from './useContentTable'
export { useAccountsTable } from './useAccountsTable'
```

`dashboard/src/features/social-publishing/index.ts`:

```typescript
/**
 * Social Publishing Feature Module
 *
 * Provides composables and API functions for managing social media publishing.
 */

// API
export * from './api'

// Composables
export * from './composables'
```

**Step 5: Verify**

Run: `cd dashboard && npx tsc --noEmit && npm run lint`
Expected: No errors

**Step 6: Commit**

```bash
git add dashboard/src/features/social-publishing/
git commit -m "feat(dashboard): add social publishing feature module with composables"
```

---

## Task 4: Navigation + Router

**Files:**
- Modify: `dashboard/src/config/navigation.ts`
- Modify: `dashboard/src/router/index.ts`

**Step 1: Add navigation items**

In `dashboard/src/config/navigation.ts`:

1. Add imports for new icons (in the import block from `lucide-vue-next`):

```typescript
import {
  // ... existing imports ...
  FileText,
  Users,
  Send,
  // ... rest ...
} from 'lucide-vue-next'
```

Note: `FileText` is already imported. Add `Users` and `Send` if not present.

2. Add three new children to the Distribution section's `children` array (after the Delivery Logs entry):

```typescript
      { title: 'Social Content', path: '/distribution/social-content', icon: FileText },
      { title: 'Social Accounts', path: '/distribution/social-accounts', icon: Users },
```

3. Update the Distribution section's `quickAction` — keep the existing one (New Route). Optionally add the Publish action as a second quick action is not supported, so leave `quickAction` as-is.

**Step 2: Add routes**

In `dashboard/src/router/index.ts`:

1. No static import needed — use lazy loading (consistent with other newer routes).

2. Add three routes in the Distribution section (after the `distribution-logs` route, before the System section):

```typescript
  {
    path: '/distribution/social-content',
    name: 'distribution-social-content',
    component: () => import('../views/distribution/SocialContentView.vue'),
    meta: { title: 'Social Content', section: 'distribution', requiresAuth: true },
  },
  {
    path: '/distribution/social-accounts',
    name: 'distribution-social-accounts',
    component: () => import('../views/distribution/SocialAccountsView.vue'),
    meta: { title: 'Social Accounts', section: 'distribution', requiresAuth: true },
  },
  {
    path: '/distribution/social-publish',
    name: 'distribution-social-publish',
    component: () => import('../views/distribution/SocialPublishView.vue'),
    meta: { title: 'Publish', section: 'distribution', requiresAuth: true },
  },
```

**Step 3: Verify**

Run: `cd dashboard && npx tsc --noEmit && npm run lint`
Expected: May show warnings about missing view components (that's fine — we create them in later tasks). If TypeScript errors on missing files, create stub files:

For each of the 3 views, create a minimal stub:

```vue
<script setup lang="ts">
</script>

<template>
  <div>TODO</div>
</template>
```

**Step 4: Commit**

```bash
git add dashboard/src/config/navigation.ts dashboard/src/router/index.ts
git add dashboard/src/views/distribution/SocialContentView.vue
git add dashboard/src/views/distribution/SocialAccountsView.vue
git add dashboard/src/views/distribution/SocialPublishView.vue
git commit -m "feat(dashboard): add social publishing navigation and routes"
```

---

## Task 5: Domain Components — DeliverySummaryBadges + ContentFilterBar

**Files:**
- Create: `dashboard/src/components/domain/social-publishing/DeliverySummaryBadges.vue`
- Create: `dashboard/src/components/domain/social-publishing/ContentFilterBar.vue`
- Create: `dashboard/src/components/domain/social-publishing/index.ts`

**Step 1: Create DeliverySummaryBadges**

`dashboard/src/components/domain/social-publishing/DeliverySummaryBadges.vue`:

```vue
<script setup lang="ts">
import type { DeliverySummary } from '@/types/socialPublisher'

defineProps<{
  summary: DeliverySummary
}>()
</script>

<template>
  <div class="flex items-center gap-1.5">
    <span
      v-if="summary.delivered > 0"
      class="inline-flex items-center rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700 dark:bg-green-900/30 dark:text-green-400"
    >
      {{ summary.delivered }} delivered
    </span>
    <span
      v-if="summary.failed > 0"
      class="inline-flex items-center rounded-full bg-red-100 px-2 py-0.5 text-xs font-medium text-red-700 dark:bg-red-900/30 dark:text-red-400"
    >
      {{ summary.failed }} failed
    </span>
    <span
      v-if="summary.pending > 0"
      class="inline-flex items-center rounded-full bg-yellow-100 px-2 py-0.5 text-xs font-medium text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400"
    >
      {{ summary.pending }} pending
    </span>
    <span
      v-if="summary.retrying > 0"
      class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
    >
      {{ summary.retrying }} retrying
    </span>
    <span
      v-if="summary.total === 0"
      class="text-xs text-muted-foreground"
    >
      No deliveries
    </span>
  </div>
</template>
```

**Step 2: Create ContentFilterBar**

`dashboard/src/components/domain/social-publishing/ContentFilterBar.vue`:

```vue
<script setup lang="ts">
import { X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import type { ContentFilters } from '@/types/socialPublisher'

interface Props {
  filters: ContentFilters
  hasActiveFilters: boolean
  activeFilterCount: number
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:status', value: string | undefined): void
  (e: 'update:type', value: string | undefined): void
  (e: 'clear-filters'): void
}>()

const statusOptions = [
  { value: undefined as string | undefined, label: 'All Statuses' },
  { value: 'delivered', label: 'Delivered' },
  { value: 'failed', label: 'Failed' },
  { value: 'pending', label: 'Pending' },
] as const

const typeOptions = [
  { value: undefined as string | undefined, label: 'All Types' },
  { value: 'social_update', label: 'Social Update' },
  { value: 'blog_post', label: 'Blog Post' },
  { value: 'news_article', label: 'News Article' },
] as const
</script>

<template>
  <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
    <div class="flex flex-wrap items-center gap-2">
      <span class="text-sm font-medium text-muted-foreground">Status:</span>
      <button
        v-for="opt in statusOptions"
        :key="String(opt.value)"
        :class="[
          'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium transition-colors',
          (opt.value === undefined && !filters.status) || filters.status === opt.value
            ? 'bg-primary text-primary-foreground'
            : 'bg-muted text-muted-foreground hover:bg-muted/80',
        ]"
        @click="emit('update:status', opt.value)"
      >
        {{ opt.label }}
      </button>
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <span class="text-sm font-medium text-muted-foreground">Type:</span>
      <button
        v-for="opt in typeOptions"
        :key="String(opt.value)"
        :class="[
          'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium transition-colors',
          (opt.value === undefined && !filters.type) || filters.type === opt.value
            ? 'bg-primary text-primary-foreground'
            : 'bg-muted text-muted-foreground hover:bg-muted/80',
        ]"
        @click="emit('update:type', opt.value)"
      >
        {{ opt.label }}
      </button>
    </div>

    <Button
      v-if="hasActiveFilters"
      variant="outline"
      size="sm"
      class="shrink-0"
      @click="emit('clear-filters')"
    >
      <X class="mr-1 h-4 w-4" />
      Clear ({{ activeFilterCount }})
    </Button>
  </div>
</template>
```

**Step 3: Create barrel export**

`dashboard/src/components/domain/social-publishing/index.ts`:

```typescript
export { default as DeliverySummaryBadges } from './DeliverySummaryBadges.vue'
export { default as ContentFilterBar } from './ContentFilterBar.vue'
export { default as ContentTable } from './ContentTable.vue'
export { default as AccountsTable } from './AccountsTable.vue'
export { default as AccountFormDialog } from './AccountFormDialog.vue'
export { default as PublishForm } from './PublishForm.vue'
```

Note: Some of these components don't exist yet — TypeScript won't error because barrel exports aren't imported until used. If the linter complains, only export the ones that exist so far and update the barrel in later tasks.

**Step 4: Verify**

Run: `cd dashboard && npx tsc --noEmit && npm run lint`
Expected: No errors

**Step 5: Commit**

```bash
git add dashboard/src/components/domain/social-publishing/
git commit -m "feat(dashboard): add DeliverySummaryBadges and ContentFilterBar components"
```

---

## Task 6: Domain Components — ContentTable

**Files:**
- Create: `dashboard/src/components/domain/social-publishing/ContentTable.vue`

**Step 1: Create ContentTable**

`dashboard/src/components/domain/social-publishing/ContentTable.vue`:

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { formatDateShort } from '@/lib/utils'
import { Loader2, RotateCcw, ChevronDown, ChevronRight } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { DataTablePagination, SortableColumnHeader } from '@/components/common'
import { DeliverySummaryBadges } from '@/components/domain/social-publishing'
import { socialPublisherApi } from '@/api/client'
import type { SocialContent, Delivery } from '@/types/socialPublisher'

interface Props {
  items: SocialContent[]
  total: number
  isLoading: boolean
  page: number
  pageSize: number
  totalPages: number
  allowedPageSizes: readonly number[]
  sortBy: string
  sortOrder: 'asc' | 'desc'
  hasActiveFilters: boolean
  onSort: (key: string) => void
  onPageChange: (page: number) => void
  onPageSizeChange: (size: number) => void
  onClearFilters: () => void
  onRetry: () => void
}

const props = defineProps<Props>()

const expandedId = ref<string | null>(null)
const deliveries = ref<Delivery[]>([])
const loadingDeliveries = ref(false)
const retryingId = ref<string | null>(null)

const sortableColumns = [
  { key: 'type', label: 'Type' },
  { key: 'title', label: 'Title' },
  { key: 'source', label: 'Source' },
  { key: 'created_at', label: 'Created' },
] as const

async function toggleExpand(id: string) {
  if (expandedId.value === id) {
    expandedId.value = null
    deliveries.value = []
    return
  }
  expandedId.value = id
  loadingDeliveries.value = true
  try {
    const response = await socialPublisherApi.content.status(id)
    deliveries.value = response.data?.deliveries ?? []
  } catch {
    deliveries.value = []
  } finally {
    loadingDeliveries.value = false
  }
}

async function retryDelivery(deliveryId: string) {
  retryingId.value = deliveryId
  try {
    await socialPublisherApi.content.retry(deliveryId)
    // Re-fetch deliveries for the expanded content
    if (expandedId.value) {
      const response = await socialPublisherApi.content.status(expandedId.value)
      deliveries.value = response.data?.deliveries ?? []
    }
    props.onRetry()
  } catch {
    // Error is visible in the delivery status
  } finally {
    retryingId.value = null
  }
}
</script>

<template>
  <div class="space-y-4">
    <div class="rounded-md border">
      <table class="w-full">
        <thead>
          <tr class="border-b bg-muted/50">
            <th class="w-8 px-2 py-3" />
            <SortableColumnHeader
              v-for="col in sortableColumns"
              :key="col.key"
              :label="col.label"
              :sort-key="col.key"
              :current-sort-by="sortBy"
              :current-sort-order="sortOrder"
              @sort="onSort(col.key)"
            />
            <th class="px-4 py-3 text-sm font-medium text-muted-foreground">
              Deliveries
            </th>
          </tr>
        </thead>
        <tbody>
          <template v-if="isLoading">
            <tr
              v-for="i in 5"
              :key="i"
              class="border-b"
            >
              <td class="px-2 py-3" />
              <td class="px-4 py-3">
                <Skeleton class="h-5 w-16" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-4 w-48" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-4 w-24" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-4 w-20" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-5 w-32" />
              </td>
            </tr>
          </template>

          <tr
            v-else-if="items.length === 0"
            class="border-b"
          >
            <td
              colspan="6"
              class="px-4 py-12 text-center"
            >
              <p class="text-sm text-muted-foreground">
                {{ hasActiveFilters ? 'No content matches your filters' : 'No content found' }}
              </p>
              <Button
                v-if="hasActiveFilters"
                variant="outline"
                size="sm"
                class="mt-2"
                @click="onClearFilters"
              >
                Clear filters
              </Button>
            </td>
          </tr>

          <template
            v-for="item in items"
            v-else
            :key="item.id"
          >
            <tr
              class="border-b transition-colors hover:bg-muted/50 cursor-pointer"
              @click="toggleExpand(item.id)"
            >
              <td class="px-2 py-3 text-center">
                <ChevronDown
                  v-if="expandedId === item.id"
                  class="h-4 w-4 text-muted-foreground"
                />
                <ChevronRight
                  v-else
                  class="h-4 w-4 text-muted-foreground"
                />
              </td>
              <td class="px-4 py-3">
                <Badge variant="secondary">
                  {{ item.type }}
                </Badge>
              </td>
              <td class="px-4 py-3 text-sm font-medium">
                {{ item.title || item.summary || '(untitled)' }}
              </td>
              <td class="px-4 py-3 text-sm text-muted-foreground">
                {{ item.source || '—' }}
              </td>
              <td class="px-4 py-3 text-sm text-muted-foreground">
                {{ formatDateShort(item.created_at) }}
              </td>
              <td class="px-4 py-3">
                <DeliverySummaryBadges
                  v-if="item.delivery_summary"
                  :summary="item.delivery_summary"
                />
                <span
                  v-else
                  class="text-xs text-muted-foreground"
                >
                  —
                </span>
              </td>
            </tr>

            <!-- Expanded delivery detail row -->
            <tr
              v-if="expandedId === item.id"
              class="border-b bg-muted/20"
            >
              <td
                colspan="6"
                class="px-6 py-4"
              >
                <div
                  v-if="loadingDeliveries"
                  class="flex items-center gap-2 text-sm text-muted-foreground"
                >
                  <Loader2 class="h-4 w-4 animate-spin" />
                  Loading deliveries...
                </div>
                <div
                  v-else-if="deliveries.length === 0"
                  class="text-sm text-muted-foreground"
                >
                  No deliveries for this content.
                </div>
                <table
                  v-else
                  class="w-full text-sm"
                >
                  <thead>
                    <tr class="text-left text-muted-foreground">
                      <th class="pb-2 font-medium">
                        Platform
                      </th>
                      <th class="pb-2 font-medium">
                        Account
                      </th>
                      <th class="pb-2 font-medium">
                        Status
                      </th>
                      <th class="pb-2 font-medium">
                        Attempts
                      </th>
                      <th class="pb-2 font-medium">
                        Error
                      </th>
                      <th class="pb-2 text-right font-medium">
                        Actions
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="delivery in deliveries"
                      :key="delivery.id"
                      class="border-t border-muted"
                    >
                      <td class="py-2">
                        <Badge variant="outline">
                          {{ delivery.platform }}
                        </Badge>
                      </td>
                      <td class="py-2">
                        {{ delivery.account }}
                      </td>
                      <td class="py-2">
                        <Badge
                          :variant="delivery.status === 'delivered' ? 'success' : delivery.status === 'failed' ? 'destructive' : 'secondary'"
                        >
                          {{ delivery.status }}
                        </Badge>
                      </td>
                      <td class="py-2">
                        {{ delivery.attempts }}/{{ delivery.max_attempts }}
                      </td>
                      <td class="py-2 text-destructive">
                        {{ delivery.error || '—' }}
                      </td>
                      <td class="py-2 text-right">
                        <Button
                          v-if="delivery.status === 'failed'"
                          variant="ghost"
                          size="sm"
                          :disabled="retryingId === delivery.id"
                          @click.stop="retryDelivery(delivery.id)"
                        >
                          <Loader2
                            v-if="retryingId === delivery.id"
                            class="mr-1 h-3 w-3 animate-spin"
                          />
                          <RotateCcw
                            v-else
                            class="mr-1 h-3 w-3"
                          />
                          Retry
                        </Button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <DataTablePagination
      :page="page"
      :page-size="pageSize"
      :total="total"
      :total-pages="totalPages"
      :allowed-page-sizes="allowedPageSizes"
      item-label="content items"
      @update:page="onPageChange"
      @update:page-size="onPageSizeChange"
    />
  </div>
</template>
```

**Step 2: Verify**

Run: `cd dashboard && npx tsc --noEmit && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add dashboard/src/components/domain/social-publishing/ContentTable.vue
git commit -m "feat(dashboard): add ContentTable component for social publishing"
```

---

## Task 7: Domain Components — AccountsTable + AccountFormDialog

**Files:**
- Create: `dashboard/src/components/domain/social-publishing/AccountsTable.vue`
- Create: `dashboard/src/components/domain/social-publishing/AccountFormDialog.vue`

**Step 1: Create AccountsTable**

`dashboard/src/components/domain/social-publishing/AccountsTable.vue`:

```vue
<script setup lang="ts">
import { formatDateShort } from '@/lib/utils'
import { Loader2, Pencil, Trash2, Check, X as XIcon } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { DataTablePagination, SortableColumnHeader } from '@/components/common'
import type { SocialAccount } from '@/types/socialPublisher'

interface Props {
  items: SocialAccount[]
  total: number
  isLoading: boolean
  page: number
  pageSize: number
  totalPages: number
  allowedPageSizes: readonly number[]
  sortBy: string
  sortOrder: 'asc' | 'desc'
  hasActiveFilters: boolean
  deletingId?: string | null
  onSort: (key: string) => void
  onPageChange: (page: number) => void
  onPageSizeChange: (size: number) => void
  onClearFilters: () => void
  onEdit: (account: SocialAccount) => void
  onDelete: (id: string) => void
}

defineProps<Props>()

const sortableColumns = [
  { key: 'name', label: 'Name' },
  { key: 'platform', label: 'Platform' },
  { key: 'project', label: 'Project' },
  { key: 'created_at', label: 'Created' },
] as const

const platformColors: Record<string, string> = {
  x: 'bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900',
  facebook: 'bg-blue-600 text-white',
  instagram: 'bg-pink-600 text-white',
  linkedin: 'bg-blue-700 text-white',
  mastodon: 'bg-purple-600 text-white',
}

function getPlatformClass(platform: string): string {
  return platformColors[platform.toLowerCase()] ?? 'bg-muted text-muted-foreground'
}
</script>

<template>
  <div class="space-y-4">
    <div class="rounded-md border">
      <table class="w-full">
        <thead>
          <tr class="border-b bg-muted/50">
            <SortableColumnHeader
              v-for="col in sortableColumns"
              :key="col.key"
              :label="col.label"
              :sort-key="col.key"
              :current-sort-by="sortBy"
              :current-sort-order="sortOrder"
              @sort="onSort(col.key)"
            />
            <th class="px-4 py-3 text-sm font-medium text-muted-foreground">
              Status
            </th>
            <th class="px-4 py-3 text-sm font-medium text-muted-foreground">
              Credentials
            </th>
            <th class="px-4 py-3 text-right text-sm font-medium text-muted-foreground">
              Actions
            </th>
          </tr>
        </thead>
        <tbody>
          <template v-if="isLoading">
            <tr
              v-for="i in 5"
              :key="i"
              class="border-b"
            >
              <td
                v-for="j in 7"
                :key="j"
                class="px-4 py-3"
              >
                <Skeleton class="h-4 w-20" />
              </td>
            </tr>
          </template>

          <tr
            v-else-if="items.length === 0"
            class="border-b"
          >
            <td
              colspan="7"
              class="px-4 py-12 text-center"
            >
              <p class="text-sm text-muted-foreground">
                {{ hasActiveFilters ? 'No accounts match your filters' : 'No accounts configured' }}
              </p>
              <Button
                v-if="hasActiveFilters"
                variant="outline"
                size="sm"
                class="mt-2"
                @click="onClearFilters"
              >
                Clear filters
              </Button>
            </td>
          </tr>

          <tr
            v-for="account in items"
            v-else
            :key="account.id"
            class="border-b transition-colors hover:bg-muted/50"
          >
            <td class="px-4 py-3 text-sm font-medium">
              {{ account.name }}
            </td>
            <td class="px-4 py-3">
              <span
                :class="[
                  'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
                  getPlatformClass(account.platform),
                ]"
              >
                {{ account.platform }}
              </span>
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ account.project || '—' }}
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ formatDateShort(account.created_at) }}
            </td>
            <td class="px-4 py-3">
              <Badge :variant="account.enabled ? 'success' : 'secondary'">
                {{ account.enabled ? 'Active' : 'Inactive' }}
              </Badge>
            </td>
            <td class="px-4 py-3">
              <Check
                v-if="account.credentials_configured"
                class="h-4 w-4 text-green-600"
              />
              <XIcon
                v-else
                class="h-4 w-4 text-muted-foreground"
              />
            </td>
            <td
              class="px-4 py-3 text-right"
            >
              <div class="flex justify-end gap-2">
                <Button
                  variant="ghost"
                  size="icon"
                  @click="onEdit(account)"
                >
                  <Pencil class="h-4 w-4" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  :disabled="deletingId === account.id"
                  @click="onDelete(account.id)"
                >
                  <Loader2
                    v-if="deletingId === account.id"
                    class="h-4 w-4 animate-spin"
                  />
                  <Trash2
                    v-else
                    class="h-4 w-4 text-destructive"
                  />
                </Button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <DataTablePagination
      :page="page"
      :page-size="pageSize"
      :total="total"
      :total-pages="totalPages"
      :allowed-page-sizes="allowedPageSizes"
      item-label="accounts"
      @update:page="onPageChange"
      @update:page-size="onPageSizeChange"
    />
  </div>
</template>
```

**Step 2: Create AccountFormDialog**

`dashboard/src/components/domain/social-publishing/AccountFormDialog.vue`:

```vue
<script setup lang="ts">
import { ref, watch } from 'vue'
import { Loader2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { SocialAccount, CreateAccountRequest, UpdateAccountRequest } from '@/types/socialPublisher'

interface Props {
  open: boolean
  account?: SocialAccount | null
  saving: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'save', data: CreateAccountRequest | UpdateAccountRequest): void
}>()

const name = ref('')
const platform = ref('')
const project = ref('')
const enabled = ref(true)
const credentials = ref('')
const tokenExpiry = ref('')

const platformOptions = ['x', 'facebook', 'instagram', 'linkedin', 'mastodon'] as const

const isEdit = ref(false)

watch(() => props.open, (open) => {
  if (open && props.account) {
    isEdit.value = true
    name.value = props.account.name
    platform.value = props.account.platform
    project.value = props.account.project
    enabled.value = props.account.enabled
    credentials.value = ''
    tokenExpiry.value = props.account.token_expiry ?? ''
  } else if (open) {
    isEdit.value = false
    name.value = ''
    platform.value = 'x'
    project.value = ''
    enabled.value = true
    credentials.value = ''
    tokenExpiry.value = ''
  }
})

function handleSubmit() {
  const base: CreateAccountRequest = {
    name: name.value,
    platform: platform.value,
    project: project.value,
    enabled: enabled.value,
    token_expiry: tokenExpiry.value || undefined,
  }

  if (credentials.value.trim()) {
    try {
      base.credentials = JSON.parse(credentials.value) as Record<string, unknown>
    } catch {
      return // Invalid JSON — don't submit
    }
  }

  emit('save', base)
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-50 flex items-center justify-center"
    >
      <div
        class="fixed inset-0 bg-black/50"
        @click="emit('close')"
      />
      <div class="relative z-50 w-full max-w-lg rounded-lg border bg-background p-6 shadow-lg">
        <h2 class="mb-4 text-lg font-semibold">
          {{ isEdit ? 'Edit Account' : 'Add Account' }}
        </h2>

        <form
          class="space-y-4"
          @submit.prevent="handleSubmit"
        >
          <div>
            <label class="mb-1 block text-sm font-medium">Name</label>
            <Input
              v-model="name"
              placeholder="Account name"
              required
            />
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium">Platform</label>
            <select
              v-model="platform"
              class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
            >
              <option
                v-for="p in platformOptions"
                :key="p"
                :value="p"
              >
                {{ p }}
              </option>
            </select>
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium">Project</label>
            <Input
              v-model="project"
              placeholder="Project name"
              required
            />
          </div>

          <div class="flex items-center gap-2">
            <input
              id="account-enabled"
              v-model="enabled"
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300"
            >
            <label
              for="account-enabled"
              class="text-sm font-medium"
            >Enabled</label>
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium">
              Credentials (JSON)
              <span class="text-xs text-muted-foreground">
                {{ isEdit ? '— leave blank to keep current' : '' }}
              </span>
            </label>
            <textarea
              v-model="credentials"
              rows="4"
              :placeholder="isEdit ? 'Leave blank to keep current credentials' : '{\"api_key\": \"...\"}'"
              class="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 font-mono"
            />
          </div>

          <div>
            <label class="mb-1 block text-sm font-medium">Token Expiry (optional)</label>
            <Input
              v-model="tokenExpiry"
              type="datetime-local"
            />
          </div>

          <div class="flex justify-end gap-2 pt-2">
            <Button
              variant="outline"
              type="button"
              @click="emit('close')"
            >
              Cancel
            </Button>
            <Button
              type="submit"
              :disabled="saving || !name || !platform || !project"
            >
              <Loader2
                v-if="saving"
                class="mr-2 h-4 w-4 animate-spin"
              />
              {{ isEdit ? 'Save Changes' : 'Create Account' }}
            </Button>
          </div>
        </form>
      </div>
    </div>
  </Teleport>
</template>
```

**Step 3: Verify**

Run: `cd dashboard && npx tsc --noEmit && npm run lint`
Expected: No errors

**Step 4: Commit**

```bash
git add dashboard/src/components/domain/social-publishing/AccountsTable.vue
git add dashboard/src/components/domain/social-publishing/AccountFormDialog.vue
git commit -m "feat(dashboard): add AccountsTable and AccountFormDialog components"
```

---

## Task 8: Domain Component — PublishForm

**Files:**
- Create: `dashboard/src/components/domain/social-publishing/PublishForm.vue`

**Step 1: Create PublishForm**

`dashboard/src/components/domain/social-publishing/PublishForm.vue`:

```vue
<script setup lang="ts">
import { ref, computed } from 'vue'
import { Loader2, Send } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import type { SocialAccount, PublishRequest, TargetConfig } from '@/types/socialPublisher'

interface Props {
  accounts: SocialAccount[]
  accountsLoading: boolean
  publishing: boolean
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'publish', data: PublishRequest): void
}>()

const contentType = ref('social_update')
const title = ref('')
const body = ref('')
const summary = ref('')
const url = ref('')
const tags = ref('')
const project = ref('')
const source = ref('')
const selectedAccounts = ref<Set<string>>(new Set())
const scheduleMode = ref<'now' | 'later'>('now')
const scheduledAt = ref('')

const typeOptions = [
  { value: 'social_update', label: 'Social Update' },
  { value: 'blog_post', label: 'Blog Post' },
  { value: 'news_article', label: 'News Article' },
] as const

const canSubmit = computed(() => {
  return contentType.value && (title.value || body.value || summary.value)
})

function toggleAccount(accountName: string) {
  const next = new Set(selectedAccounts.value)
  if (next.has(accountName)) {
    next.delete(accountName)
  } else {
    next.add(accountName)
  }
  selectedAccounts.value = next
}

function handleSubmit() {
  const targets: TargetConfig[] = []
  for (const accountName of selectedAccounts.value) {
    const acct = (arguments[0] as { accounts: SocialAccount[] }).accounts?.find(
      (a: SocialAccount) => a.name === accountName
    )
    if (acct) {
      targets.push({ platform: acct.platform, account: acct.name })
    }
  }

  const data: PublishRequest = {
    type: contentType.value,
    title: title.value || undefined,
    body: body.value || undefined,
    summary: summary.value || undefined,
    url: url.value || undefined,
    tags: tags.value ? tags.value.split(',').map((t) => t.trim()).filter(Boolean) : undefined,
    project: project.value || undefined,
    source: source.value || undefined,
    targets: targets.length > 0 ? targets : undefined,
    scheduled_at: scheduleMode.value === 'later' && scheduledAt.value
      ? new Date(scheduledAt.value).toISOString()
      : undefined,
  }

  emit('publish', data)
}
</script>

<template>
  <form
    class="space-y-6"
    @submit.prevent="handleSubmit"
  >
    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="mb-1 block text-sm font-medium">Type</label>
        <select
          v-model="contentType"
          class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
        >
          <option
            v-for="opt in typeOptions"
            :key="opt.value"
            :value="opt.value"
          >
            {{ opt.label }}
          </option>
        </select>
      </div>

      <div>
        <label class="mb-1 block text-sm font-medium">Project</label>
        <Input
          v-model="project"
          placeholder="e.g. personal"
        />
      </div>
    </div>

    <div>
      <label class="mb-1 block text-sm font-medium">Title</label>
      <Input
        v-model="title"
        placeholder="Content title"
      />
    </div>

    <div>
      <label class="mb-1 block text-sm font-medium">Body</label>
      <textarea
        v-model="body"
        rows="5"
        placeholder="Write your content..."
        class="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
      />
    </div>

    <div>
      <label class="mb-1 block text-sm font-medium">Summary</label>
      <Input
        v-model="summary"
        placeholder="Brief summary"
      />
    </div>

    <div class="grid gap-4 sm:grid-cols-2">
      <div>
        <label class="mb-1 block text-sm font-medium">URL</label>
        <Input
          v-model="url"
          placeholder="https://..."
        />
      </div>
      <div>
        <label class="mb-1 block text-sm font-medium">Source</label>
        <Input
          v-model="source"
          placeholder="Content source"
        />
      </div>
    </div>

    <div>
      <label class="mb-1 block text-sm font-medium">Tags (comma-separated)</label>
      <Input
        v-model="tags"
        placeholder="tag1, tag2, tag3"
      />
    </div>

    <!-- Target Accounts -->
    <div>
      <label class="mb-2 block text-sm font-medium">Target Accounts</label>
      <div
        v-if="accountsLoading"
        class="text-sm text-muted-foreground"
      >
        <Loader2 class="mr-2 inline h-4 w-4 animate-spin" />
        Loading accounts...
      </div>
      <div
        v-else-if="accounts.length === 0"
        class="text-sm text-muted-foreground"
      >
        No accounts configured. <a
          href="/dashboard/distribution/social-accounts"
          class="text-primary hover:underline"
        >Add one first.</a>
      </div>
      <div
        v-else
        class="flex flex-wrap gap-2"
      >
        <button
          v-for="acct in accounts.filter(a => a.enabled)"
          :key="acct.id"
          type="button"
          :class="[
            'inline-flex items-center gap-2 rounded-lg border px-3 py-2 text-sm transition-colors',
            selectedAccounts.has(acct.name)
              ? 'border-primary bg-primary/10 text-primary'
              : 'border-muted hover:border-primary/50',
          ]"
          @click="toggleAccount(acct.name)"
        >
          <input
            type="checkbox"
            :checked="selectedAccounts.has(acct.name)"
            class="h-4 w-4 rounded border-gray-300"
            @click.stop
            @change="toggleAccount(acct.name)"
          >
          {{ acct.name }}
          <Badge
            variant="outline"
            class="text-xs"
          >
            {{ acct.platform }}
          </Badge>
        </button>
      </div>
    </div>

    <!-- Schedule -->
    <div>
      <label class="mb-2 block text-sm font-medium">Schedule</label>
      <div class="flex items-center gap-4">
        <button
          type="button"
          :class="[
            'inline-flex items-center rounded-full px-3 py-1.5 text-sm font-medium transition-colors',
            scheduleMode === 'now'
              ? 'bg-primary text-primary-foreground'
              : 'bg-muted text-muted-foreground hover:bg-muted/80',
          ]"
          @click="scheduleMode = 'now'"
        >
          Publish Now
        </button>
        <button
          type="button"
          :class="[
            'inline-flex items-center rounded-full px-3 py-1.5 text-sm font-medium transition-colors',
            scheduleMode === 'later'
              ? 'bg-primary text-primary-foreground'
              : 'bg-muted text-muted-foreground hover:bg-muted/80',
          ]"
          @click="scheduleMode = 'later'"
        >
          Schedule for Later
        </button>
      </div>
      <div
        v-if="scheduleMode === 'later'"
        class="mt-3"
      >
        <Input
          v-model="scheduledAt"
          type="datetime-local"
          required
        />
      </div>
    </div>

    <!-- Submit -->
    <div class="flex justify-end pt-2">
      <Button
        type="submit"
        :disabled="publishing || !canSubmit"
        size="lg"
      >
        <Loader2
          v-if="publishing"
          class="mr-2 h-4 w-4 animate-spin"
        />
        <Send
          v-else
          class="mr-2 h-4 w-4"
        />
        {{ scheduleMode === 'later' ? 'Schedule' : 'Publish' }}
      </Button>
    </div>
  </form>
</template>
```

**Note:** The `handleSubmit` function above has a bug — it tries to access accounts via `arguments[0]` which won't work in `<script setup>`. Fix it by accepting accounts as a prop and using it directly. Here's the corrected `handleSubmit`:

```typescript
function handleSubmit() {
  const accts = props.accounts // <-- use props instead of arguments
  const targets: TargetConfig[] = []
  for (const accountName of selectedAccounts.value) {
    const acct = accts.find((a) => a.name === accountName)
    if (acct) {
      targets.push({ platform: acct.platform, account: acct.name })
    }
  }

  // ... rest stays the same
}
```

Actually, to be explicit, the `defineProps` already gives us access via the `props` variable we destructure. Make sure the component has:

```typescript
const props = defineProps<Props>()
```

And `handleSubmit` uses `props.accounts`.

**Step 2: Verify**

Run: `cd dashboard && npx tsc --noEmit && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add dashboard/src/components/domain/social-publishing/PublishForm.vue
git commit -m "feat(dashboard): add PublishForm component for social publishing"
```

---

## Task 9: Update barrel export

**Files:**
- Modify: `dashboard/src/components/domain/social-publishing/index.ts`

**Step 1: Ensure all components are exported**

The barrel export should now have all 6 components. Verify it reads:

```typescript
export { default as DeliverySummaryBadges } from './DeliverySummaryBadges.vue'
export { default as ContentFilterBar } from './ContentFilterBar.vue'
export { default as ContentTable } from './ContentTable.vue'
export { default as AccountsTable } from './AccountsTable.vue'
export { default as AccountFormDialog } from './AccountFormDialog.vue'
export { default as PublishForm } from './PublishForm.vue'
```

**Step 2: Verify**

Run: `cd dashboard && npx tsc --noEmit && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add dashboard/src/components/domain/social-publishing/index.ts
git commit -m "feat(dashboard): finalize social publishing barrel exports"
```

---

## Task 10: View — SocialContentView

**Files:**
- Modify: `dashboard/src/views/distribution/SocialContentView.vue` (replace stub)

**Step 1: Implement the view**

`dashboard/src/views/distribution/SocialContentView.vue`:

```vue
<script setup lang="ts">
import { Loader2, FileText } from 'lucide-vue-next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { ContentFilterBar, ContentTable } from '@/components/domain/social-publishing'
import { useContentTable } from '@/features/social-publishing'

const contentTable = useContentTable()

function onStatusChange(value: string | undefined) {
  contentTable.setFilter('status', value)
}

function onTypeChange(value: string | undefined) {
  contentTable.setFilter('type', value)
}

function onRetry() {
  contentTable.refetch()
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-3xl font-bold tracking-tight">
        Social Content
      </h1>
      <p class="text-muted-foreground">
        Published and scheduled content with delivery tracking
      </p>
    </div>

    <div
      v-if="contentTable.isLoading.value && contentTable.items.value.length === 0"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <Card
      v-else-if="contentTable.error.value"
      class="border-destructive"
    >
      <CardContent class="pt-6">
        <p class="text-destructive">
          {{ contentTable.error.value?.message || 'Unable to load content.' }}
        </p>
      </CardContent>
    </Card>

    <Card v-else-if="contentTable.items.value.length === 0 && !contentTable.hasActiveFilters.value">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <FileText class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No social content yet
        </h3>
        <p class="text-muted-foreground">
          Content will appear here after you publish using the Publish page.
        </p>
      </CardContent>
    </Card>

    <template v-else>
      <Card>
        <CardHeader class="pb-4">
          <CardTitle class="text-base">
            Filter Content
          </CardTitle>
        </CardHeader>
        <CardContent>
          <ContentFilterBar
            :filters="contentTable.filters.value"
            :has-active-filters="contentTable.hasActiveFilters.value"
            :active-filter-count="contentTable.activeFilterCount.value"
            @update:status="onStatusChange"
            @update:type="onTypeChange"
            @clear-filters="contentTable.clearFilters"
          />
        </CardContent>
      </Card>

      <Card>
        <CardContent class="p-0">
          <ContentTable
            :items="contentTable.items.value"
            :total="contentTable.total.value"
            :is-loading="contentTable.isLoading.value"
            :page="contentTable.page.value"
            :page-size="contentTable.pageSize.value"
            :total-pages="contentTable.totalPages.value"
            :allowed-page-sizes="contentTable.allowedPageSizes"
            :sort-by="contentTable.sortBy.value"
            :sort-order="contentTable.sortOrder.value"
            :has-active-filters="contentTable.hasActiveFilters.value"
            :on-sort="contentTable.toggleSort"
            :on-page-change="contentTable.setPage"
            :on-page-size-change="contentTable.setPageSize"
            :on-clear-filters="contentTable.clearFilters"
            :on-retry="onRetry"
          />
        </CardContent>
      </Card>
    </template>
  </div>
</template>
```

**Step 2: Verify**

Run: `cd dashboard && npx tsc --noEmit && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add dashboard/src/views/distribution/SocialContentView.vue
git commit -m "feat(dashboard): implement SocialContentView with filters and delivery tracking"
```

---

## Task 11: View — SocialAccountsView

**Files:**
- Modify: `dashboard/src/views/distribution/SocialAccountsView.vue` (replace stub)

**Step 1: Implement the view**

`dashboard/src/views/distribution/SocialAccountsView.vue`:

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { Loader2, Users, Plus } from 'lucide-vue-next'
import { toast } from 'vue-sonner'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { AccountsTable, AccountFormDialog } from '@/components/domain/social-publishing'
import { useAccountsTable } from '@/features/social-publishing'
import { socialPublisherApi } from '@/api/client'
import type { SocialAccount, CreateAccountRequest, UpdateAccountRequest } from '@/types/socialPublisher'

const accountsTable = useAccountsTable()
const dialogOpen = ref(false)
const editingAccount = ref<SocialAccount | null>(null)
const saving = ref(false)
const deleting = ref<string | null>(null)

function openCreate() {
  editingAccount.value = null
  dialogOpen.value = true
}

function openEdit(account: SocialAccount) {
  editingAccount.value = account
  dialogOpen.value = true
}

async function handleSave(data: CreateAccountRequest | UpdateAccountRequest) {
  saving.value = true
  try {
    if (editingAccount.value) {
      await socialPublisherApi.accounts.update(editingAccount.value.id, data as UpdateAccountRequest)
      toast.success('Account updated')
    } else {
      await socialPublisherApi.accounts.create(data as CreateAccountRequest)
      toast.success('Account created')
    }
    dialogOpen.value = false
    accountsTable.refetch()
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'Failed to save account'
    toast.error(message)
  } finally {
    saving.value = false
  }
}

async function handleDelete(id: string) {
  if (!confirm('Are you sure you want to delete this account?')) return
  deleting.value = id
  try {
    await socialPublisherApi.accounts.delete(id)
    toast.success('Account deleted')
    accountsTable.refetch()
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'Failed to delete account'
    toast.error(message)
  } finally {
    deleting.value = null
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Social Accounts
        </h1>
        <p class="text-muted-foreground">
          Manage social media accounts for publishing
        </p>
      </div>
      <Button @click="openCreate">
        <Plus class="mr-2 h-4 w-4" />
        Add Account
      </Button>
    </div>

    <div
      v-if="accountsTable.isLoading.value && accountsTable.items.value.length === 0"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <Card
      v-else-if="accountsTable.error.value"
      class="border-destructive"
    >
      <CardContent class="pt-6">
        <p class="text-destructive">
          {{ accountsTable.error.value?.message || 'Unable to load accounts.' }}
        </p>
      </CardContent>
    </Card>

    <Card v-else-if="accountsTable.items.value.length === 0 && !accountsTable.hasActiveFilters.value">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <Users class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No social accounts configured
        </h3>
        <p class="text-muted-foreground mb-4">
          Add your first social media account to start publishing.
        </p>
        <Button @click="openCreate">
          <Plus class="mr-2 h-4 w-4" />
          Add Account
        </Button>
      </CardContent>
    </Card>

    <Card v-else>
      <CardContent class="p-0">
        <AccountsTable
          :items="accountsTable.items.value"
          :total="accountsTable.total.value"
          :is-loading="accountsTable.isLoading.value"
          :page="accountsTable.page.value"
          :page-size="accountsTable.pageSize.value"
          :total-pages="accountsTable.totalPages.value"
          :allowed-page-sizes="accountsTable.allowedPageSizes"
          :sort-by="accountsTable.sortBy.value"
          :sort-order="accountsTable.sortOrder.value"
          :has-active-filters="accountsTable.hasActiveFilters.value"
          :deleting-id="deleting"
          :on-sort="accountsTable.toggleSort"
          :on-page-change="accountsTable.setPage"
          :on-page-size-change="accountsTable.setPageSize"
          :on-clear-filters="accountsTable.clearFilters"
          :on-edit="openEdit"
          :on-delete="handleDelete"
        />
      </CardContent>
    </Card>

    <AccountFormDialog
      :open="dialogOpen"
      :account="editingAccount"
      :saving="saving"
      @close="dialogOpen = false"
      @save="handleSave"
    />
  </div>
</template>
```

**Step 2: Verify**

Run: `cd dashboard && npx tsc --noEmit && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add dashboard/src/views/distribution/SocialAccountsView.vue
git commit -m "feat(dashboard): implement SocialAccountsView with CRUD and dialog"
```

---

## Task 12: View — SocialPublishView

**Files:**
- Modify: `dashboard/src/views/distribution/SocialPublishView.vue` (replace stub)

**Step 1: Implement the view**

`dashboard/src/views/distribution/SocialPublishView.vue`:

```vue
<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useQuery } from '@tanstack/vue-query'
import { toast } from 'vue-sonner'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { PublishForm } from '@/components/domain/social-publishing'
import { socialPublisherApi } from '@/api/client'
import type { SocialAccount, PublishRequest } from '@/types/socialPublisher'

const router = useRouter()
const publishing = ref(false)

const { data: accountsData, isLoading: accountsLoading } = useQuery({
  queryKey: ['social-publisher', 'accounts', 'list'],
  queryFn: async (): Promise<SocialAccount[]> => {
    const response = await socialPublisherApi.accounts.list()
    return response.data?.items ?? []
  },
})

async function handlePublish(data: PublishRequest) {
  publishing.value = true
  try {
    await socialPublisherApi.content.publish(data)
    toast.success(data.scheduled_at ? 'Content scheduled' : 'Content published')
    router.push('/distribution/social-content')
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'Failed to publish content'
    toast.error(message)
  } finally {
    publishing.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-3xl font-bold tracking-tight">
        Publish
      </h1>
      <p class="text-muted-foreground">
        Create and publish content to social media accounts
      </p>
    </div>

    <Card>
      <CardHeader>
        <CardTitle>New Publication</CardTitle>
      </CardHeader>
      <CardContent>
        <PublishForm
          :accounts="accountsData ?? []"
          :accounts-loading="accountsLoading"
          :publishing="publishing"
          @publish="handlePublish"
        />
      </CardContent>
    </Card>
  </div>
</template>
```

**Step 2: Verify**

Run: `cd dashboard && npx tsc --noEmit && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add dashboard/src/views/distribution/SocialPublishView.vue
git commit -m "feat(dashboard): implement SocialPublishView with scheduling support"
```

---

## Task 13: Final Verification

**Step 1: Full type-check**

Run: `cd dashboard && npx tsc --noEmit`
Expected: No errors

**Step 2: Full lint**

Run: `cd dashboard && npm run lint`
Expected: No errors

**Step 3: Build check**

Run: `cd dashboard && npm run build`
Expected: Build succeeds. Check the output for any warnings.

**Step 4: Fix any issues found in steps 1-3**

Address any TypeScript or lint errors. Common issues:
- Missing imports
- Unused variables
- Type mismatches between component props and usage

**Step 5: Final commit (if fixes were needed)**

```bash
git add -A dashboard/
git commit -m "fix(dashboard): address lint and type errors in social publishing frontend"
```

---

## Summary

| Task | Files Created | Files Modified | Description |
|------|--------------|---------------|-------------|
| 1 | 1 | 0 | TypeScript types |
| 2 | 0 | 2 | Vite proxy + API client |
| 3 | 6 | 0 | Feature module (API + composables) |
| 4 | 3 stubs | 2 | Navigation + router |
| 5 | 3 | 0 | DeliverySummaryBadges + ContentFilterBar + barrel |
| 6 | 1 | 0 | ContentTable |
| 7 | 2 | 0 | AccountsTable + AccountFormDialog |
| 8 | 1 | 0 | PublishForm |
| 9 | 0 | 1 | Barrel export finalization |
| 10 | 0 | 1 | SocialContentView |
| 11 | 0 | 1 | SocialAccountsView |
| 12 | 0 | 1 | SocialPublishView |
| 13 | 0 | 0 | Final verification |

**Total:** ~17 new files, 6 modified files, 13 commits.
