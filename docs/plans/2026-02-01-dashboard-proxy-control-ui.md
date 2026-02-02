# Dashboard - Proxy Control UI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a Proxy Control page to the dashboard for managing the nc-http-proxy service, including mode switching, domain overrides, cache management, and audit logs.

**Architecture:** Vue 3 Composition API with TypeScript, TanStack Query for server state, Pinia for UI state, Tailwind CSS for styling. Feature module pattern with api/, composables/, stores/.

**Tech Stack:** Vue 3.5+, TypeScript, TanStack Query, Pinia, Tailwind CSS 4, Lucide Icons

---

## Prerequisites

Before starting, ensure:
- Dashboard runs: `cd dashboard && npm run dev`
- Proxy runs: `task proxy:up` (port 8055)
- Nginx routes `/api/proxy/` to proxy service

---

## Task 1: Add Proxy Types

**Files:**
- Create: `/home/fsd42/dev/north-cloud/dashboard/src/types/proxy.ts`
- Modify: `/home/fsd42/dev/north-cloud/dashboard/src/types/index.ts`

**Step 1.1: Create proxy types file**

Create `src/types/proxy.ts`:

```typescript
/**
 * Proxy operating modes
 */
export type ProxyMode = 'replay' | 'record' | 'live' | 'hybrid'

/**
 * Proxy status response from GET /admin/status
 */
export interface ProxyStatus {
  mode: ProxyMode
  fixtures_count: number
  cache_count: number
  domains: string[]
  domain_overrides: Record<string, ProxyMode>
  hybrid_fallback: boolean
}

/**
 * Mode change audit entry from GET /admin/audit
 */
export interface ModeChangeAudit {
  timestamp: string
  user: string
  old_mode: string
  new_mode: string
  domain?: string
}

/**
 * Cache entry for a domain
 */
export interface CacheEntry {
  key: string
  method: string
  url: string
  status: number
  timestamp: string
}

/**
 * Request to set proxy mode
 */
export interface SetModeRequest {
  mode: ProxyMode
  user?: string
}

/**
 * Request to set domain mode override
 */
export interface SetDomainModeRequest {
  domain: string
  mode: ProxyMode
  user?: string
}
```

**Step 1.2: Export from index.ts**

Add to `src/types/index.ts`:

```typescript
export * from './proxy'
```

**Step 1.3: Run type check**

Run: `cd /home/fsd42/dev/north-cloud/dashboard && npm run type-check`
Expected: No errors

**Step 1.4: Commit**

```bash
git add dashboard/src/types/proxy.ts dashboard/src/types/index.ts
git commit -m "$(cat <<'EOF'
feat(dashboard): add proxy types

Defines ProxyStatus, ModeChangeAudit, CacheEntry,
SetModeRequest, and SetDomainModeRequest types.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Add Proxy API Client

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/dashboard/src/api/client.ts`

**Step 2.1: Add proxy client instance**

In `src/api/client.ts`, add after other client definitions:

```typescript
import type {
  ProxyStatus,
  ModeChangeAudit,
  CacheEntry,
  ProxyMode,
} from '@/types'

// Proxy client (nc-http-proxy admin API)
const proxyClient = axios.create({
  baseURL: '/api/proxy',
  timeout: 10000,
})

// Add auth interceptor
addAuthInterceptor(proxyClient)
```

**Step 2.2: Add proxy API object**

Add the proxy API methods:

```typescript
export const proxyApi = {
  /**
   * Get current proxy status
   */
  getStatus: () =>
    proxyClient.get<ProxyStatus>('/admin/status'),

  /**
   * Set global proxy mode
   */
  setMode: (mode: ProxyMode, user?: string) =>
    proxyClient.post(`/admin/mode/${mode}`, null, {
      headers: user ? { 'X-User': user } : {},
    }),

  /**
   * Set per-domain mode override
   */
  setDomainMode: (domain: string, mode: ProxyMode, user?: string) =>
    proxyClient.post(`/admin/mode/${domain}/${mode}`, null, {
      headers: user ? { 'X-User': user } : {},
    }),

  /**
   * Clear domain mode override
   */
  clearDomainMode: (domain: string) =>
    proxyClient.delete(`/admin/mode/${domain}`),

  /**
   * Enable/disable hybrid fallback mode
   */
  setHybridFallback: (enable: boolean) =>
    proxyClient.post(`/admin/mode/hybrid?enable=${enable}`),

  /**
   * Get mode change audit log
   */
  getAuditLog: () =>
    proxyClient.get<ModeChangeAudit[]>('/admin/audit'),

  /**
   * List all cached domains
   */
  listDomains: () =>
    proxyClient.get<string[]>('/admin/cache'),

  /**
   * Get cache entries for a domain
   */
  getDomainCache: (domain: string) =>
    proxyClient.get<CacheEntry[]>(`/admin/cache/${domain}`),

  /**
   * Clear all cache
   */
  clearCache: () =>
    proxyClient.delete('/admin/cache'),

  /**
   * Clear cache for specific domain
   */
  clearDomainCache: (domain: string) =>
    proxyClient.delete(`/admin/cache/${domain}`),
}
```

**Step 2.3: Run type check**

Run: `cd /home/fsd42/dev/north-cloud/dashboard && npm run type-check`
Expected: No errors

**Step 2.4: Commit**

```bash
git add dashboard/src/api/client.ts
git commit -m "$(cat <<'EOF'
feat(dashboard): add proxy API client

Adds proxyApi with methods for:
- Status, mode switching, domain overrides
- Hybrid fallback, audit log
- Cache listing and clearing

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Create Proxy Feature Module Structure

**Files:**
- Create: `/home/fsd42/dev/north-cloud/dashboard/src/features/proxy-control/index.ts`
- Create: `/home/fsd42/dev/north-cloud/dashboard/src/features/proxy-control/api/index.ts`
- Create: `/home/fsd42/dev/north-cloud/dashboard/src/features/proxy-control/api/proxy.ts`

**Step 3.1: Create directory structure**

```bash
mkdir -p /home/fsd42/dev/north-cloud/dashboard/src/features/proxy-control/{api,composables,stores}
```

**Step 3.2: Create API module with query keys**

Create `src/features/proxy-control/api/proxy.ts`:

```typescript
import { proxyApi } from '@/api/client'
import type { ProxyStatus, ModeChangeAudit, CacheEntry, ProxyMode } from '@/types'

/**
 * Query key factory for proxy queries
 */
export const proxyKeys = {
  all: ['proxy'] as const,
  status: () => [...proxyKeys.all, 'status'] as const,
  audit: () => [...proxyKeys.all, 'audit'] as const,
  domains: () => [...proxyKeys.all, 'domains'] as const,
  domainCache: (domain: string) => [...proxyKeys.all, 'cache', domain] as const,
}

/**
 * Fetch proxy status
 */
export async function fetchProxyStatus(): Promise<ProxyStatus> {
  const response = await proxyApi.getStatus()
  return response.data
}

/**
 * Fetch audit log
 */
export async function fetchAuditLog(): Promise<ModeChangeAudit[]> {
  const response = await proxyApi.getAuditLog()
  return response.data
}

/**
 * Fetch domain cache entries
 */
export async function fetchDomainCache(domain: string): Promise<CacheEntry[]> {
  const response = await proxyApi.getDomainCache(domain)
  return response.data
}

/**
 * Set global proxy mode
 */
export async function setProxyMode(mode: ProxyMode, user?: string): Promise<void> {
  await proxyApi.setMode(mode, user)
}

/**
 * Set domain mode override
 */
export async function setDomainMode(
  domain: string,
  mode: ProxyMode,
  user?: string
): Promise<void> {
  await proxyApi.setDomainMode(domain, mode, user)
}

/**
 * Clear domain mode override
 */
export async function clearDomainMode(domain: string): Promise<void> {
  await proxyApi.clearDomainMode(domain)
}

/**
 * Set hybrid fallback mode
 */
export async function setHybridFallback(enable: boolean): Promise<void> {
  await proxyApi.setHybridFallback(enable)
}

/**
 * Clear all cache
 */
export async function clearAllCache(): Promise<void> {
  await proxyApi.clearCache()
}

/**
 * Clear domain cache
 */
export async function clearDomainCacheApi(domain: string): Promise<void> {
  await proxyApi.clearDomainCache(domain)
}
```

**Step 3.3: Create API index**

Create `src/features/proxy-control/api/index.ts`:

```typescript
export * from './proxy'
```

**Step 3.4: Create feature index**

Create `src/features/proxy-control/index.ts`:

```typescript
export * from './api'
export * from './composables'
export * from './stores'
```

**Step 3.5: Commit**

```bash
git add dashboard/src/features/proxy-control/
git commit -m "$(cat <<'EOF'
feat(dashboard): create proxy-control feature module structure

Adds api/ with query keys and API functions for proxy control.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Create Proxy Stores

**Files:**
- Create: `/home/fsd42/dev/north-cloud/dashboard/src/features/proxy-control/stores/useProxyUIStore.ts`
- Create: `/home/fsd42/dev/north-cloud/dashboard/src/features/proxy-control/stores/index.ts`

**Step 4.1: Create UI store**

Create `src/features/proxy-control/stores/useProxyUIStore.ts`:

```typescript
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { ProxyMode } from '@/types'

export const useProxyUIStore = defineStore('proxy-ui', () => {
  // Modal state
  const modals = ref({
    modeConfirm: false,
    addOverride: false,
    clearCache: false,
    clearDomainCache: false,
  })

  // Pending mode change (for confirmation)
  const pendingModeChange = ref<{
    mode: ProxyMode
    domain?: string
  } | null>(null)

  // Selected domain for cache operations
  const selectedDomain = ref<string | null>(null)

  // Expanded domain in cache list
  const expandedDomain = ref<string | null>(null)

  // Actions
  function openModeConfirm(mode: ProxyMode, domain?: string) {
    pendingModeChange.value = { mode, domain }
    modals.value.modeConfirm = true
  }

  function closeModeConfirm() {
    modals.value.modeConfirm = false
    pendingModeChange.value = null
  }

  function openAddOverride() {
    modals.value.addOverride = true
  }

  function closeAddOverride() {
    modals.value.addOverride = false
  }

  function openClearCache() {
    modals.value.clearCache = true
  }

  function closeClearCache() {
    modals.value.clearCache = false
  }

  function openClearDomainCache(domain: string) {
    selectedDomain.value = domain
    modals.value.clearDomainCache = true
  }

  function closeClearDomainCache() {
    modals.value.clearDomainCache = false
    selectedDomain.value = null
  }

  function toggleExpandedDomain(domain: string) {
    expandedDomain.value = expandedDomain.value === domain ? null : domain
  }

  // Computed
  const requiresConfirmation = computed(() => {
    if (!pendingModeChange.value) return false
    const mode = pendingModeChange.value.mode
    return mode === 'record' || mode === 'live'
  })

  return {
    modals,
    pendingModeChange,
    selectedDomain,
    expandedDomain,
    requiresConfirmation,
    openModeConfirm,
    closeModeConfirm,
    openAddOverride,
    closeAddOverride,
    openClearCache,
    closeClearCache,
    openClearDomainCache,
    closeClearDomainCache,
    toggleExpandedDomain,
  }
})
```

**Step 4.2: Create stores index**

Create `src/features/proxy-control/stores/index.ts`:

```typescript
export { useProxyUIStore } from './useProxyUIStore'
```

**Step 4.3: Commit**

```bash
git add dashboard/src/features/proxy-control/stores/
git commit -m "$(cat <<'EOF'
feat(dashboard): add proxy UI store

Manages modal state, pending mode changes, and domain selection
for the proxy control UI.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Create Proxy Composables

**Files:**
- Create: `/home/fsd42/dev/north-cloud/dashboard/src/features/proxy-control/composables/useProxyQuery.ts`
- Create: `/home/fsd42/dev/north-cloud/dashboard/src/features/proxy-control/composables/useProxyMutations.ts`
- Create: `/home/fsd42/dev/north-cloud/dashboard/src/features/proxy-control/composables/useProxyControl.ts`
- Create: `/home/fsd42/dev/north-cloud/dashboard/src/features/proxy-control/composables/index.ts`

**Step 5.1: Create query composable**

Create `src/features/proxy-control/composables/useProxyQuery.ts`:

```typescript
import { useQuery } from '@tanstack/vue-query'
import {
  proxyKeys,
  fetchProxyStatus,
  fetchAuditLog,
  fetchDomainCache,
} from '../api'

/**
 * Query for proxy status with polling
 */
export function useProxyStatusQuery() {
  return useQuery({
    queryKey: proxyKeys.status(),
    queryFn: fetchProxyStatus,
    refetchInterval: 5000, // Poll every 5 seconds
    staleTime: 2000,
  })
}

/**
 * Query for audit log
 */
export function useAuditLogQuery() {
  return useQuery({
    queryKey: proxyKeys.audit(),
    queryFn: fetchAuditLog,
    staleTime: 10000,
  })
}

/**
 * Query for domain cache entries
 */
export function useDomainCacheQuery(domain: () => string | null) {
  return useQuery({
    queryKey: () => domain() ? proxyKeys.domainCache(domain()!) : proxyKeys.domains(),
    queryFn: () => domain() ? fetchDomainCache(domain()!) : Promise.resolve([]),
    enabled: () => !!domain(),
  })
}
```

**Step 5.2: Create mutations composable**

Create `src/features/proxy-control/composables/useProxyMutations.ts`:

```typescript
import { useMutation, useQueryClient } from '@tanstack/vue-query'
import {
  proxyKeys,
  setProxyMode,
  setDomainMode,
  clearDomainMode,
  setHybridFallback,
  clearAllCache,
  clearDomainCacheApi,
} from '../api'
import type { ProxyMode } from '@/types'

/**
 * Mutation for setting global proxy mode
 */
export function useSetModeMutation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ mode, user }: { mode: ProxyMode; user?: string }) =>
      setProxyMode(mode, user),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: proxyKeys.status() })
      queryClient.invalidateQueries({ queryKey: proxyKeys.audit() })
    },
  })
}

/**
 * Mutation for setting domain mode override
 */
export function useSetDomainModeMutation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      domain,
      mode,
      user,
    }: {
      domain: string
      mode: ProxyMode
      user?: string
    }) => setDomainMode(domain, mode, user),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: proxyKeys.status() })
      queryClient.invalidateQueries({ queryKey: proxyKeys.audit() })
    },
  })
}

/**
 * Mutation for clearing domain mode override
 */
export function useClearDomainModeMutation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (domain: string) => clearDomainMode(domain),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: proxyKeys.status() })
    },
  })
}

/**
 * Mutation for setting hybrid fallback
 */
export function useSetHybridFallbackMutation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (enable: boolean) => setHybridFallback(enable),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: proxyKeys.status() })
    },
  })
}

/**
 * Mutation for clearing all cache
 */
export function useClearCacheMutation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: () => clearAllCache(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: proxyKeys.status() })
    },
  })
}

/**
 * Mutation for clearing domain cache
 */
export function useClearDomainCacheMutation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (domain: string) => clearDomainCacheApi(domain),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: proxyKeys.status() })
    },
  })
}
```

**Step 5.3: Create main composable**

Create `src/features/proxy-control/composables/useProxyControl.ts`:

```typescript
import { computed, toRef } from 'vue'
import { useProxyUIStore } from '../stores'
import {
  useProxyStatusQuery,
  useAuditLogQuery,
  useDomainCacheQuery,
} from './useProxyQuery'
import {
  useSetModeMutation,
  useSetDomainModeMutation,
  useClearDomainModeMutation,
  useSetHybridFallbackMutation,
  useClearCacheMutation,
  useClearDomainCacheMutation,
} from './useProxyMutations'
import type { ProxyMode } from '@/types'

/**
 * Main composable for proxy control functionality
 */
export function useProxyControl() {
  // Stores
  const uiStore = useProxyUIStore()

  // Queries
  const statusQuery = useProxyStatusQuery()
  const auditQuery = useAuditLogQuery()
  const domainCacheQuery = useDomainCacheQuery(() => uiStore.expandedDomain)

  // Mutations
  const setModeMutation = useSetModeMutation()
  const setDomainModeMutation = useSetDomainModeMutation()
  const clearDomainModeMutation = useClearDomainModeMutation()
  const setHybridMutation = useSetHybridFallbackMutation()
  const clearCacheMutation = useClearCacheMutation()
  const clearDomainCacheMutation = useClearDomainCacheMutation()

  // Computed: Data
  const status = computed(() => statusQuery.data.value)
  const auditLog = computed(() => auditQuery.data.value || [])
  const domainCache = computed(() => domainCacheQuery.data.value || [])
  const isLoading = computed(() => statusQuery.isLoading.value)
  const error = computed(() => statusQuery.error.value)

  // Computed: Derived
  const currentMode = computed(() => status.value?.mode || 'replay')
  const domains = computed(() => status.value?.domains || [])
  const domainOverrides = computed(() => status.value?.domain_overrides || {})
  const hybridEnabled = computed(() => status.value?.hybrid_fallback || false)
  const fixturesCount = computed(() => status.value?.fixtures_count || 0)
  const cacheCount = computed(() => status.value?.cache_count || 0)

  // Computed: Loading states
  const isMutating = computed(
    () =>
      setModeMutation.isPending.value ||
      setDomainModeMutation.isPending.value ||
      clearCacheMutation.isPending.value
  )

  // Actions
  async function changeMode(mode: ProxyMode, user?: string) {
    await setModeMutation.mutateAsync({ mode, user })
    uiStore.closeModeConfirm()
  }

  async function addDomainOverride(domain: string, mode: ProxyMode, user?: string) {
    await setDomainModeMutation.mutateAsync({ domain, mode, user })
    uiStore.closeAddOverride()
  }

  async function removeDomainOverride(domain: string) {
    await clearDomainModeMutation.mutateAsync(domain)
  }

  async function toggleHybrid() {
    await setHybridMutation.mutateAsync(!hybridEnabled.value)
  }

  async function clearCache() {
    await clearCacheMutation.mutateAsync()
    uiStore.closeClearCache()
  }

  async function clearDomainCache(domain: string) {
    await clearDomainCacheMutation.mutateAsync(domain)
    uiStore.closeClearDomainCache()
  }

  function refetch() {
    statusQuery.refetch()
    auditQuery.refetch()
  }

  return {
    // Data
    status,
    auditLog,
    domainCache,
    isLoading,
    error,
    isMutating,

    // Derived
    currentMode,
    domains,
    domainOverrides,
    hybridEnabled,
    fixturesCount,
    cacheCount,

    // Actions
    changeMode,
    addDomainOverride,
    removeDomainOverride,
    toggleHybrid,
    clearCache,
    clearDomainCache,
    refetch,

    // UI Store
    ui: uiStore,
  }
}
```

**Step 5.4: Create composables index**

Create `src/features/proxy-control/composables/index.ts`:

```typescript
export { useProxyStatusQuery, useAuditLogQuery, useDomainCacheQuery } from './useProxyQuery'
export {
  useSetModeMutation,
  useSetDomainModeMutation,
  useClearDomainModeMutation,
  useSetHybridFallbackMutation,
  useClearCacheMutation,
  useClearDomainCacheMutation,
} from './useProxyMutations'
export { useProxyControl } from './useProxyControl'
```

**Step 5.5: Run type check**

Run: `cd /home/fsd42/dev/north-cloud/dashboard && npm run type-check`
Expected: No errors

**Step 5.6: Commit**

```bash
git add dashboard/src/features/proxy-control/composables/
git commit -m "$(cat <<'EOF'
feat(dashboard): add proxy control composables

Adds TanStack Query hooks for status/audit/cache queries
and mutations for mode changes and cache operations.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: Create Proxy Components

**Files:**
- Create: `/home/fsd42/dev/north-cloud/dashboard/src/components/proxy/ModeSelector.vue`
- Create: `/home/fsd42/dev/north-cloud/dashboard/src/components/proxy/ModeConfirmDialog.vue`
- Create: `/home/fsd42/dev/north-cloud/dashboard/src/components/proxy/DomainOverrideTable.vue`
- Create: `/home/fsd42/dev/north-cloud/dashboard/src/components/proxy/AuditLogTable.vue`
- Create: `/home/fsd42/dev/north-cloud/dashboard/src/components/proxy/CacheStatsCard.vue`

**Step 6.1: Create components directory**

```bash
mkdir -p /home/fsd42/dev/north-cloud/dashboard/src/components/proxy
```

**Step 6.2: Create ModeSelector component**

Create `src/components/proxy/ModeSelector.vue`:

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { Radio, Wifi, Database, Layers } from 'lucide-vue-next'
import type { ProxyMode } from '@/types'

const props = defineProps<{
  currentMode: ProxyMode
  hybridEnabled: boolean
  disabled?: boolean
}>()

const emit = defineEmits<{
  selectMode: [mode: ProxyMode]
  toggleHybrid: []
}>()

interface ModeOption {
  value: ProxyMode
  label: string
  description: string
  icon: typeof Radio
  color: string
}

const modes: ModeOption[] = [
  {
    value: 'replay',
    label: 'Replay',
    description: 'Read from fixtures/cache only',
    icon: Database,
    color: 'text-green-600 bg-green-50 border-green-200',
  },
  {
    value: 'record',
    label: 'Record',
    description: 'Fetch live + cache responses',
    icon: Radio,
    color: 'text-amber-600 bg-amber-50 border-amber-200',
  },
  {
    value: 'live',
    label: 'Live',
    description: 'Pass-through to real servers',
    icon: Wifi,
    color: 'text-red-600 bg-red-50 border-red-200',
  },
]

const effectiveMode = computed(() => {
  if (props.hybridEnabled && props.currentMode === 'replay') {
    return 'hybrid'
  }
  return props.currentMode
})

function handleModeSelect(mode: ProxyMode) {
  if (!props.disabled) {
    emit('selectMode', mode)
  }
}
</script>

<template>
  <div class="space-y-4">
    <div class="grid grid-cols-3 gap-4">
      <button
        v-for="mode in modes"
        :key="mode.value"
        :disabled="disabled"
        :class="[
          'relative p-4 rounded-lg border-2 transition-all',
          'hover:shadow-md focus:outline-none focus:ring-2 focus:ring-offset-2',
          currentMode === mode.value
            ? mode.color + ' ring-2 ring-offset-2'
            : 'bg-white border-gray-200 hover:border-gray-300',
          disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer',
        ]"
        @click="handleModeSelect(mode.value)"
      >
        <component
          :is="mode.icon"
          class="h-6 w-6 mx-auto mb-2"
          :class="currentMode === mode.value ? '' : 'text-gray-400'"
        />
        <div class="text-sm font-medium">{{ mode.label }}</div>
        <div class="text-xs text-gray-500 mt-1">{{ mode.description }}</div>
        <div
          v-if="currentMode === mode.value"
          class="absolute top-2 right-2 h-2 w-2 rounded-full bg-current"
        />
      </button>
    </div>

    <!-- Hybrid Toggle -->
    <div
      v-if="currentMode === 'replay'"
      class="flex items-center justify-between p-3 bg-gray-50 rounded-lg"
    >
      <div class="flex items-center gap-2">
        <Layers class="h-5 w-5 text-gray-500" />
        <div>
          <div class="text-sm font-medium">Hybrid Fallback</div>
          <div class="text-xs text-gray-500">
            Fall back to record on cache miss
          </div>
        </div>
      </div>
      <button
        :disabled="disabled"
        :class="[
          'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out',
          hybridEnabled ? 'bg-blue-600' : 'bg-gray-200',
          disabled ? 'opacity-50 cursor-not-allowed' : '',
        ]"
        @click="emit('toggleHybrid')"
      >
        <span
          :class="[
            'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
            hybridEnabled ? 'translate-x-5' : 'translate-x-0',
          ]"
        />
      </button>
    </div>
  </div>
</template>
```

**Step 6.3: Create ModeConfirmDialog component**

Create `src/components/proxy/ModeConfirmDialog.vue`:

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { AlertTriangle, X } from 'lucide-vue-next'
import type { ProxyMode } from '@/types'

const props = defineProps<{
  open: boolean
  mode: ProxyMode | null
  domain?: string | null
  isLoading?: boolean
}>()

const emit = defineEmits<{
  confirm: []
  cancel: []
}>()

const title = computed(() => {
  if (props.domain) {
    return `Set ${props.domain} to ${props.mode} mode?`
  }
  return `Switch to ${props.mode} mode?`
})

const warningMessage = computed(() => {
  if (props.mode === 'record') {
    return 'Record mode will make live HTTP requests and cache the responses. This may affect external services and consume bandwidth.'
  }
  if (props.mode === 'live') {
    return 'Live mode will make all requests to real servers without caching. This will affect external services and may be rate-limited.'
  }
  return ''
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-50 flex items-center justify-center"
    >
      <!-- Backdrop -->
      <div
        class="absolute inset-0 bg-black/50"
        @click="emit('cancel')"
      />

      <!-- Dialog -->
      <div class="relative bg-white rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
        <!-- Close button -->
        <button
          class="absolute top-4 right-4 text-gray-400 hover:text-gray-600"
          @click="emit('cancel')"
        >
          <X class="h-5 w-5" />
        </button>

        <!-- Icon -->
        <div class="flex items-center justify-center w-12 h-12 mx-auto mb-4 rounded-full bg-amber-100">
          <AlertTriangle class="h-6 w-6 text-amber-600" />
        </div>

        <!-- Content -->
        <h3 class="text-lg font-semibold text-center mb-2">
          {{ title }}
        </h3>
        <p class="text-sm text-gray-600 text-center mb-6">
          {{ warningMessage }}
        </p>

        <!-- Actions -->
        <div class="flex gap-3">
          <button
            class="flex-1 px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 transition-colors"
            :disabled="isLoading"
            @click="emit('cancel')"
          >
            Cancel
          </button>
          <button
            class="flex-1 px-4 py-2 text-sm font-medium text-white bg-amber-600 rounded-lg hover:bg-amber-700 transition-colors disabled:opacity-50"
            :disabled="isLoading"
            @click="emit('confirm')"
          >
            {{ isLoading ? 'Switching...' : 'Confirm' }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
```

**Step 6.4: Create DomainOverrideTable component**

Create `src/components/proxy/DomainOverrideTable.vue`:

```vue
<script setup lang="ts">
import { Trash2, Plus } from 'lucide-vue-next'
import type { ProxyMode } from '@/types'

defineProps<{
  overrides: Record<string, ProxyMode>
  isLoading?: boolean
}>()

const emit = defineEmits<{
  add: []
  remove: [domain: string]
}>()

function getModeColor(mode: ProxyMode): string {
  switch (mode) {
    case 'replay':
      return 'bg-green-100 text-green-800'
    case 'record':
      return 'bg-amber-100 text-amber-800'
    case 'live':
      return 'bg-red-100 text-red-800'
    default:
      return 'bg-gray-100 text-gray-800'
  }
}
</script>

<template>
  <div class="bg-white rounded-lg border border-gray-200">
    <div class="flex items-center justify-between px-4 py-3 border-b border-gray-200">
      <h3 class="text-sm font-medium text-gray-900">Domain Overrides</h3>
      <button
        class="inline-flex items-center gap-1 px-3 py-1.5 text-sm font-medium text-blue-600 bg-blue-50 rounded-lg hover:bg-blue-100 transition-colors"
        @click="emit('add')"
      >
        <Plus class="h-4 w-4" />
        Add Override
      </button>
    </div>

    <div v-if="Object.keys(overrides).length === 0" class="px-4 py-8 text-center text-gray-500">
      No domain overrides configured
    </div>

    <div v-else class="divide-y divide-gray-100">
      <div
        v-for="(mode, domain) in overrides"
        :key="domain"
        class="flex items-center justify-between px-4 py-3"
      >
        <div class="flex items-center gap-3">
          <code class="text-sm font-mono bg-gray-100 px-2 py-1 rounded">
            {{ domain }}
          </code>
          <span
            :class="[
              'inline-flex items-center px-2 py-0.5 rounded text-xs font-medium',
              getModeColor(mode as ProxyMode),
            ]"
          >
            {{ mode }}
          </span>
        </div>
        <button
          class="p-1.5 text-gray-400 hover:text-red-600 hover:bg-red-50 rounded transition-colors"
          :disabled="isLoading"
          @click="emit('remove', domain as string)"
        >
          <Trash2 class="h-4 w-4" />
        </button>
      </div>
    </div>
  </div>
</template>
```

**Step 6.5: Create AuditLogTable component**

Create `src/components/proxy/AuditLogTable.vue`:

```vue
<script setup lang="ts">
import { formatDistanceToNow } from 'date-fns'
import type { ModeChangeAudit } from '@/types'

defineProps<{
  entries: ModeChangeAudit[]
  isLoading?: boolean
}>()

function formatTime(timestamp: string): string {
  try {
    return formatDistanceToNow(new Date(timestamp), { addSuffix: true })
  } catch {
    return timestamp
  }
}
</script>

<template>
  <div class="bg-white rounded-lg border border-gray-200">
    <div class="px-4 py-3 border-b border-gray-200">
      <h3 class="text-sm font-medium text-gray-900">Mode Change History</h3>
    </div>

    <div v-if="isLoading" class="px-4 py-8 text-center text-gray-500">
      Loading...
    </div>

    <div v-else-if="entries.length === 0" class="px-4 py-8 text-center text-gray-500">
      No mode changes recorded
    </div>

    <div v-else class="overflow-x-auto">
      <table class="min-w-full divide-y divide-gray-200">
        <thead class="bg-gray-50">
          <tr>
            <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">
              Time
            </th>
            <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">
              User
            </th>
            <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">
              Change
            </th>
            <th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">
              Domain
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100">
          <tr v-for="(entry, index) in entries" :key="index" class="hover:bg-gray-50">
            <td class="px-4 py-2 text-sm text-gray-500 whitespace-nowrap">
              {{ formatTime(entry.timestamp) }}
            </td>
            <td class="px-4 py-2 text-sm text-gray-900">
              {{ entry.user }}
            </td>
            <td class="px-4 py-2 text-sm">
              <span class="text-gray-500">{{ entry.old_mode }}</span>
              <span class="mx-1 text-gray-400">→</span>
              <span class="font-medium text-gray-900">{{ entry.new_mode }}</span>
            </td>
            <td class="px-4 py-2 text-sm text-gray-500">
              {{ entry.domain || 'global' }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
```

**Step 6.6: Create CacheStatsCard component**

Create `src/components/proxy/CacheStatsCard.vue`:

```vue
<script setup lang="ts">
import { Database, FileJson, Trash2 } from 'lucide-vue-next'

defineProps<{
  fixturesCount: number
  cacheCount: number
  domainsCount: number
  isLoading?: boolean
}>()

const emit = defineEmits<{
  clearCache: []
}>()
</script>

<template>
  <div class="bg-white rounded-lg border border-gray-200 p-4">
    <div class="flex items-center justify-between mb-4">
      <h3 class="text-sm font-medium text-gray-900">Cache Statistics</h3>
      <button
        class="inline-flex items-center gap-1 px-3 py-1.5 text-sm font-medium text-red-600 bg-red-50 rounded-lg hover:bg-red-100 transition-colors"
        :disabled="isLoading"
        @click="emit('clearCache')"
      >
        <Trash2 class="h-4 w-4" />
        Clear All
      </button>
    </div>

    <div class="grid grid-cols-3 gap-4">
      <div class="text-center">
        <div class="flex items-center justify-center w-10 h-10 mx-auto mb-2 rounded-full bg-blue-50">
          <FileJson class="h-5 w-5 text-blue-600" />
        </div>
        <div class="text-2xl font-semibold text-gray-900">
          {{ fixturesCount }}
        </div>
        <div class="text-xs text-gray-500">Fixtures</div>
      </div>

      <div class="text-center">
        <div class="flex items-center justify-center w-10 h-10 mx-auto mb-2 rounded-full bg-green-50">
          <Database class="h-5 w-5 text-green-600" />
        </div>
        <div class="text-2xl font-semibold text-gray-900">
          {{ cacheCount }}
        </div>
        <div class="text-xs text-gray-500">Cached</div>
      </div>

      <div class="text-center">
        <div class="flex items-center justify-center w-10 h-10 mx-auto mb-2 rounded-full bg-purple-50">
          <Database class="h-5 w-5 text-purple-600" />
        </div>
        <div class="text-2xl font-semibold text-gray-900">
          {{ domainsCount }}
        </div>
        <div class="text-xs text-gray-500">Domains</div>
      </div>
    </div>
  </div>
</template>
```

**Step 6.7: Commit**

```bash
git add dashboard/src/components/proxy/
git commit -m "$(cat <<'EOF'
feat(dashboard): add proxy control components

Components for proxy control UI:
- ModeSelector: Mode buttons with hybrid toggle
- ModeConfirmDialog: Confirmation for record/live modes
- DomainOverrideTable: Domain override management
- AuditLogTable: Mode change history
- CacheStatsCard: Cache statistics display

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: Create Proxy Control View

**Files:**
- Create: `/home/fsd42/dev/north-cloud/dashboard/src/views/system/ProxyControlView.vue`

**Step 7.1: Create the view**

Create `src/views/system/ProxyControlView.vue`:

```vue
<script setup lang="ts">
import { useProxyControl } from '@/features/proxy-control'
import ModeSelector from '@/components/proxy/ModeSelector.vue'
import ModeConfirmDialog from '@/components/proxy/ModeConfirmDialog.vue'
import DomainOverrideTable from '@/components/proxy/DomainOverrideTable.vue'
import AuditLogTable from '@/components/proxy/AuditLogTable.vue'
import CacheStatsCard from '@/components/proxy/CacheStatsCard.vue'
import { RefreshCw, AlertCircle } from 'lucide-vue-next'

const proxy = useProxyControl()

async function handleModeSelect(mode: string) {
  // Require confirmation for record/live modes
  if (mode === 'record' || mode === 'live') {
    proxy.ui.openModeConfirm(mode as 'record' | 'live')
  } else {
    await proxy.changeMode(mode as 'replay')
  }
}

async function handleModeConfirm() {
  if (proxy.ui.pendingModeChange) {
    await proxy.changeMode(
      proxy.ui.pendingModeChange.mode,
      'dashboard-user'
    )
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold text-gray-900">HTTP Proxy Control</h1>
        <p class="text-sm text-gray-500 mt-1">
          Manage the development HTTP replay proxy
        </p>
      </div>
      <button
        class="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors"
        :disabled="proxy.isLoading"
        @click="proxy.refetch"
      >
        <RefreshCw
          class="h-4 w-4"
          :class="{ 'animate-spin': proxy.isLoading }"
        />
        Refresh
      </button>
    </div>

    <!-- Error State -->
    <div
      v-if="proxy.error"
      class="flex items-center gap-3 p-4 bg-red-50 border border-red-200 rounded-lg"
    >
      <AlertCircle class="h-5 w-5 text-red-600 flex-shrink-0" />
      <div class="text-sm text-red-800">
        Failed to load proxy status. Is the proxy running?
      </div>
    </div>

    <!-- Loading State -->
    <div v-else-if="proxy.isLoading && !proxy.status" class="text-center py-12">
      <RefreshCw class="h-8 w-8 text-gray-400 animate-spin mx-auto mb-4" />
      <p class="text-gray-500">Loading proxy status...</p>
    </div>

    <!-- Content -->
    <template v-else>
      <!-- Mode Selector -->
      <div class="bg-white rounded-lg border border-gray-200 p-6">
        <h2 class="text-lg font-semibold text-gray-900 mb-4">Operating Mode</h2>
        <ModeSelector
          :current-mode="proxy.currentMode"
          :hybrid-enabled="proxy.hybridEnabled"
          :disabled="proxy.isMutating"
          @select-mode="handleModeSelect"
          @toggle-hybrid="proxy.toggleHybrid"
        />
      </div>

      <!-- Stats and Overrides Grid -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <!-- Cache Stats -->
        <CacheStatsCard
          :fixtures-count="proxy.fixturesCount"
          :cache-count="proxy.cacheCount"
          :domains-count="proxy.domains.length"
          :is-loading="proxy.isMutating"
          @clear-cache="proxy.ui.openClearCache"
        />

        <!-- Domain Overrides -->
        <DomainOverrideTable
          :overrides="proxy.domainOverrides"
          :is-loading="proxy.isMutating"
          @add="proxy.ui.openAddOverride"
          @remove="proxy.removeDomainOverride"
        />
      </div>

      <!-- Audit Log -->
      <AuditLogTable
        :entries="proxy.auditLog"
        :is-loading="proxy.isLoading"
      />
    </template>

    <!-- Mode Confirm Dialog -->
    <ModeConfirmDialog
      :open="proxy.ui.modals.modeConfirm"
      :mode="proxy.ui.pendingModeChange?.mode ?? null"
      :domain="proxy.ui.pendingModeChange?.domain"
      :is-loading="proxy.isMutating"
      @confirm="handleModeConfirm"
      @cancel="proxy.ui.closeModeConfirm"
    />
  </div>
</template>
```

**Step 7.2: Commit**

```bash
git add dashboard/src/views/system/ProxyControlView.vue
git commit -m "$(cat <<'EOF'
feat(dashboard): add ProxyControlView

Main view for proxy control with:
- Mode selector with confirmation dialogs
- Cache statistics card
- Domain override management
- Audit log display

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Add Route and Navigation

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/dashboard/src/router/index.ts`
- Modify: `/home/fsd42/dev/north-cloud/dashboard/src/config/navigation.ts`

**Step 8.1: Add route**

In `src/router/index.ts`, add the route in the System Overview section:

```typescript
import ProxyControlView from '../views/system/ProxyControlView.vue'

// Add to routes array in System Overview section:
{
  path: '/system/proxy',
  name: 'system-proxy',
  component: ProxyControlView,
  meta: {
    title: 'Proxy Control',
    section: 'system',
    requiresAuth: true,
  },
},
```

**Step 8.2: Add navigation item**

In `src/config/navigation.ts`, add to the System Overview section:

```typescript
import { Network } from 'lucide-vue-next'

// In the System Overview section children array:
{
  title: 'System Overview',
  icon: Settings,
  children: [
    { title: 'Health', path: '/system/health', icon: HeartPulse },
    { title: 'Auth', path: '/system/auth', icon: Shield },
    { title: 'Cache', path: '/system/cache', icon: HardDrive },
    { title: 'HTTP Proxy', path: '/system/proxy', icon: Network }, // NEW
  ],
},
```

**Step 8.3: Run type check and dev server**

Run: `cd /home/fsd42/dev/north-cloud/dashboard && npm run type-check && npm run dev`
Expected: No errors, dev server starts

**Step 8.4: Commit**

```bash
git add dashboard/src/router/index.ts dashboard/src/config/navigation.ts
git commit -m "$(cat <<'EOF'
feat(dashboard): add proxy control route and navigation

Adds /system/proxy route and sidebar navigation item
under System Overview section.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Add Nginx Routing (if needed)

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/infrastructure/nginx/conf.d/default.conf` (if exists)

**Step 9.1: Check if nginx config needs update**

The proxy API needs to be accessible at `/api/proxy/`. Check if routing exists:

```nginx
# Add if not present:
location /api/proxy/ {
    proxy_pass http://nc-http-proxy:8055/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

**Step 9.2: Commit if changes made**

```bash
git add infrastructure/nginx/
git commit -m "$(cat <<'EOF'
feat(nginx): add proxy API routing

Routes /api/proxy/* to nc-http-proxy:8055

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Add AddOverrideDialog Component

**Files:**
- Create: `/home/fsd42/dev/north-cloud/dashboard/src/components/proxy/AddOverrideDialog.vue`
- Modify: `/home/fsd42/dev/north-cloud/dashboard/src/views/system/ProxyControlView.vue`

**Step 10.1: Create AddOverrideDialog**

Create `src/components/proxy/AddOverrideDialog.vue`:

```vue
<script setup lang="ts">
import { ref, watch } from 'vue'
import { X } from 'lucide-vue-next'
import type { ProxyMode } from '@/types'

const props = defineProps<{
  open: boolean
  isLoading?: boolean
}>()

const emit = defineEmits<{
  confirm: [domain: string, mode: ProxyMode]
  cancel: []
}>()

const domain = ref('')
const mode = ref<ProxyMode>('record')

// Reset form when dialog opens
watch(() => props.open, (isOpen) => {
  if (isOpen) {
    domain.value = ''
    mode.value = 'record'
  }
})

function handleSubmit() {
  if (domain.value.trim()) {
    emit('confirm', domain.value.trim(), mode.value)
  }
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-50 flex items-center justify-center"
    >
      <!-- Backdrop -->
      <div
        class="absolute inset-0 bg-black/50"
        @click="emit('cancel')"
      />

      <!-- Dialog -->
      <div class="relative bg-white rounded-lg shadow-xl max-w-md w-full mx-4 p-6">
        <!-- Close button -->
        <button
          class="absolute top-4 right-4 text-gray-400 hover:text-gray-600"
          @click="emit('cancel')"
        >
          <X class="h-5 w-5" />
        </button>

        <h3 class="text-lg font-semibold mb-4">Add Domain Override</h3>

        <form @submit.prevent="handleSubmit" class="space-y-4">
          <!-- Domain input -->
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">
              Domain
            </label>
            <input
              v-model="domain"
              type="text"
              placeholder="example.com"
              class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
              :disabled="isLoading"
            />
          </div>

          <!-- Mode select -->
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">
              Mode
            </label>
            <select
              v-model="mode"
              class="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
              :disabled="isLoading"
            >
              <option value="replay">Replay</option>
              <option value="record">Record</option>
              <option value="live">Live</option>
            </select>
          </div>

          <!-- Actions -->
          <div class="flex gap-3 pt-2">
            <button
              type="button"
              class="flex-1 px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 rounded-lg hover:bg-gray-200 transition-colors"
              :disabled="isLoading"
              @click="emit('cancel')"
            >
              Cancel
            </button>
            <button
              type="submit"
              class="flex-1 px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded-lg hover:bg-blue-700 transition-colors disabled:opacity-50"
              :disabled="isLoading || !domain.trim()"
            >
              {{ isLoading ? 'Adding...' : 'Add Override' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </Teleport>
</template>
```

**Step 10.2: Add dialog to ProxyControlView**

In `src/views/system/ProxyControlView.vue`, add:

```vue
<script setup lang="ts">
import AddOverrideDialog from '@/components/proxy/AddOverrideDialog.vue'

// Add handler function:
async function handleAddOverride(domain: string, mode: ProxyMode) {
  await proxy.addDomainOverride(domain, mode, 'dashboard-user')
}
</script>

<template>
  <!-- Add at the end of template, after ModeConfirmDialog: -->
  <AddOverrideDialog
    :open="proxy.ui.modals.addOverride"
    :is-loading="proxy.isMutating"
    @confirm="handleAddOverride"
    @cancel="proxy.ui.closeAddOverride"
  />
</template>
```

**Step 10.3: Commit**

```bash
git add dashboard/src/components/proxy/AddOverrideDialog.vue dashboard/src/views/system/ProxyControlView.vue
git commit -m "$(cat <<'EOF'
feat(dashboard): add AddOverrideDialog for domain overrides

Dialog for adding per-domain mode overrides with domain
input and mode selection.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: Integration Testing

**Files:**
- No file changes - verification only

**Step 11.1: Start services**

```bash
task docker:dev:up
task proxy:up
cd dashboard && npm run dev
```

**Step 11.2: Navigate to Proxy Control page**

Open browser to `http://localhost:3002/system/proxy`

**Step 11.3: Test mode switching**

1. Click "Record" mode button
2. Verify confirmation dialog appears
3. Click "Confirm"
4. Verify mode changes in status

**Step 11.4: Test domain override**

1. Click "Add Override"
2. Enter domain: `example.com`
3. Select mode: `record`
4. Click "Add Override"
5. Verify override appears in table

**Step 11.5: Test cache clear**

1. Click "Clear All" button
2. Verify confirmation (if implemented)
3. Verify cache count updates

**Step 11.6: Verify audit log**

Check that mode changes appear in the audit log table.

---

## Task 12: Update Feature Index and Documentation

**Files:**
- Modify: `/home/fsd42/dev/north-cloud/dashboard/src/features/index.ts`

**Step 12.1: Export proxy-control feature**

In `src/features/index.ts`, add:

```typescript
export * from './proxy-control'
```

**Step 12.2: Commit**

```bash
git add dashboard/src/features/index.ts
git commit -m "$(cat <<'EOF'
feat(dashboard): export proxy-control feature module

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Summary

This plan implements Phase 4 with 12 tasks:

| Task | Description | Files |
|------|-------------|-------|
| 1 | Add proxy types | src/types/proxy.ts |
| 2 | Add proxy API client | src/api/client.ts |
| 3 | Create feature module structure | src/features/proxy-control/ |
| 4 | Create proxy stores | stores/useProxyUIStore.ts |
| 5 | Create proxy composables | composables/*.ts |
| 6 | Create proxy components | components/proxy/*.vue |
| 7 | Create ProxyControlView | views/system/ProxyControlView.vue |
| 8 | Add route and navigation | router/index.ts, config/navigation.ts |
| 9 | Add nginx routing | infrastructure/nginx/ |
| 10 | Add AddOverrideDialog | components/proxy/AddOverrideDialog.vue |
| 11 | Integration testing | (verification only) |
| 12 | Update feature index | features/index.ts |

**Features Implemented:**
- Mode selector with visual feedback
- Confirmation dialogs for record/live modes
- Hybrid fallback toggle
- Domain override management (add/remove)
- Cache statistics display
- Cache clearing
- Audit log table

**Tech Stack:**
- Vue 3.5 Composition API
- TypeScript strict mode
- TanStack Query for server state
- Pinia for UI state
- Tailwind CSS 4
- Lucide icons

**Total: ~12 commits, following Vue.js best practices**
