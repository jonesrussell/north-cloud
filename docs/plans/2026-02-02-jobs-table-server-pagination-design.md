# Jobs Table Server-Side Pagination & Sorting Design

**Date:** 2026-02-02
**Status:** Draft
**Author:** Claude (with user collaboration)

## Problem Statement

The dashboard's Jobs view at `/dashboard/intake/jobs` has critical bugs when displaying large datasets:

1. **Incorrect total count** - Shows "50 total jobs" when 320+ exist in database
2. **Broken pagination** - Shows 14 pages but only first 2 have results
3. **No sorting** - Column headers are not sortable
4. **No page size control** - Users cannot adjust items per page

### Root Cause

The current implementation uses **client-side pagination on partial data**:

1. Backend returns max 50 jobs (hardcoded default limit)
2. Frontend receives 50 jobs, unaware 270+ more exist
3. Frontend slices those 50 into pages of 25 (client-side)
4. Stats card counts returned array length, not true total
5. Backend doesn't support `sort_by` or `sort_order` params

## Solution Overview

Implement true **server-side pagination and sorting** with:

1. **Backend API enhancements** - Add sorting params, validate inputs, return accurate totals
2. **Reusable composable** - `useServerPaginatedTable()` for all data tables
3. **UI improvements** - Sortable columns, page size selector, accurate stats

## Architecture

### Design Principles

- **Server-side pagination** - Backend handles limit/offset, frontend never slices
- **Server-side sorting** - Backend handles ORDER BY, frontend sends params
- **Single source of truth** - Composable owns all table state
- **Fail-safe behavior** - Invalid inputs degrade gracefully, never crash
- **DRY** - Reusable composable for jobs, sources, executions tables

### Component Responsibilities

| Layer | Responsibility |
|-------|----------------|
| `useServerPaginatedTable` | State management, query coordination |
| TanStack Query | Caching, refetching, loading states |
| API client | HTTP requests, param serialization |
| Backend handler | Validation, SQL execution, response shaping |
| Vue components | Presentation, user interaction |

---

## Detailed Design

### 1. Composable API Surface

```typescript
// dashboard/src/composables/useServerPaginatedTable.ts

interface UseServerPaginatedTableOptions<T, F = Record<string, unknown>> {
  // Core
  fetchFn: (params: FetchParams<F>) => Promise<PaginatedResponse<T>>
  queryKeyPrefix: string  // e.g., 'jobs' for cache invalidation

  // Defaults
  defaultLimit?: number              // 25
  defaultSortBy?: string             // 'created_at'
  defaultSortOrder?: 'asc' | 'desc'  // 'desc'

  // Constraints
  allowedSortFields: string[]        // whitelist for safety
  allowedPageSizes?: number[]        // [10, 25, 50, 100]

  // Optional features
  syncToUrl?: boolean                // sync state to query params
  refetchInterval?: number           // auto-refresh in ms
  debounceMs?: number                // debounce filter changes
}

interface FetchParams<F> {
  limit: number
  offset: number
  sortBy: string
  sortOrder: 'asc' | 'desc'
  filters?: F
}

interface PaginatedResponse<T> {
  items: T[]
  total: number
}
```

**Returned API:**

```typescript
interface UseServerPaginatedTableReturn<T, F> {
  // Data state
  items: ComputedRef<T[]>
  total: ComputedRef<number>
  isLoading: ComputedRef<boolean>
  isRefetching: ComputedRef<boolean>
  error: ComputedRef<Error | null>
  hasError: ComputedRef<boolean>
  initialLoadDone: ComputedRef<boolean>

  // Pagination state
  page: Ref<number>
  pageSize: Ref<number>
  totalPages: ComputedRef<number>
  hasNextPage: ComputedRef<boolean>
  hasPreviousPage: ComputedRef<boolean>

  // Sorting state
  sortBy: Ref<string>
  sortOrder: Ref<'asc' | 'desc'>

  // Filter state
  filters: Ref<F>

  // Actions
  setPage: (page: number) => void
  setPageSize: (size: number) => void
  nextPage: () => void
  prevPage: () => void
  setSort: (field: string, order?: 'asc' | 'desc') => void
  toggleSort: (field: string) => void
  setFilters: (filters: Partial<F>) => void
  replaceFilters: (filters: F) => void
  clearFilters: () => void
  reset: () => void
  refetch: () => Promise<void>
}
```

### 2. Internal State Model

```typescript
// Pagination state (reactive refs)
const page = ref(1)
const pageSize = ref(options.defaultLimit ?? 25)
const sortBy = ref(options.defaultSortBy ?? 'created_at')
const sortOrder = ref<'asc' | 'desc'>(options.defaultSortOrder ?? 'desc')
const filters = ref<F>({} as F)

// Computed query params (derived, for TanStack Query key)
const queryParams = computed<FetchParams<F>>(() => ({
  limit: pageSize.value,
  offset: (page.value - 1) * pageSize.value,
  sortBy: sortBy.value,
  sortOrder: sortOrder.value,
  filters: normalizeFilters(filters.value),
}))

// TanStack Query integration (server state)
const { data, isLoading, isFetching, error, refetch } = useQuery({
  queryKey: computed(() => [options.queryKeyPrefix, 'list', queryParams.value]),
  queryFn: () => options.fetchFn(queryParams.value),
  staleTime: 10_000,
  refetchInterval: options.refetchInterval,
  placeholderData: keepPreviousData,  // Prevents flicker
})
```

**Key decisions:**
- Offset-based pagination (simpler for stable job datasets)
- Query key includes full params (automatic cache separation)
- Page resets to 1 when filters, sort, or pageSize change

### 3. Action Methods

```typescript
function setPage(newPage: number) {
  const clamped = Math.max(1, Math.min(newPage, totalPages.value))
  if (page.value === clamped) return  // No-op guard
  page.value = clamped
}

function setPageSize(newSize: number) {
  if (!options.allowedPageSizes?.includes(newSize)) return
  if (pageSize.value === newSize) return  // No-op guard
  pageSize.value = newSize
  page.value = 1  // Reset to prevent out-of-bounds
}

function setSort(field: string, order?: 'asc' | 'desc') {
  if (!options.allowedSortFields.includes(field)) {
    if (import.meta.env.DEV) console.warn(`Invalid sort field: ${field}`)
    return
  }

  // Toggle if same field without explicit order
  if (sortBy.value === field && !order) {
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortBy.value = field
    sortOrder.value = order ?? 'asc'
  }
  page.value = 1
}

function setFilters(newFilters: Partial<F>) {
  const merged = { ...filters.value, ...newFilters }
  filters.value = normalizeFilters(merged)
  page.value = 1
}

function replaceFilters(next: F) {
  filters.value = normalizeFilters(next)
  page.value = 1
}

function clearFilters() {
  filters.value = {} as F
  page.value = 1
}

function reset() {
  page.value = 1
  pageSize.value = options.defaultLimit ?? 25
  sortBy.value = options.defaultSortBy ?? 'created_at'
  sortOrder.value = options.defaultSortOrder ?? 'desc'
  filters.value = {} as F
}

// Helper: Remove undefined/empty values to prevent cache fragmentation
function normalizeFilters<T extends Record<string, unknown>>(obj: T): T {
  return Object.fromEntries(
    Object.entries(obj).filter(([_, v]) => v !== undefined && v !== '')
  ) as T
}
```

### 4. Backend API Contract

**Endpoint:** `GET /api/v1/jobs`

**Query Parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `limit` | int | 50 | Items per page (max 250) |
| `offset` | int | 0 | Skip N items |
| `sort_by` | string | `created_at` | Column to sort |
| `sort_order` | string | `desc` | `asc` or `desc` |
| `status` | string | - | Filter by status |
| `source_id` | string | - | Filter by source |
| `search` | string | - | Text search (source_name, url) |

**Allowed Sort Fields:**

```go
var allowedSortFields = map[string]string{
    "created_at":  "created_at",
    "updated_at":  "updated_at",
    "status":      "status",
    "source_name": "source_name",
    "next_run_at": "next_run_at",
    "last_run_at": "last_run_at",
}
```

**Response Shape:**

```json
{
  "jobs": [...],
  "total": 324,
  "limit": 50,
  "offset": 0,
  "sort_by": "next_run_at",
  "sort_order": "asc"
}
```

**SQL Pattern:**

```sql
SELECT * FROM jobs
WHERE ($1 = '' OR status = $1)
  AND ($2 = '' OR source_id = $2)
  AND ($3 = '' OR source_name ILIKE '%' || $3 || '%' OR url ILIKE '%' || $3 || '%')
ORDER BY $sort_column $sort_order NULLS LAST
LIMIT $4 OFFSET $5
```

**Validation Rules:**
- Sort field validated against whitelist (prevents SQL injection)
- Limit capped at `MaxPageSize` (250)
- Invalid sort fields fall back to `created_at`
- `total` is COUNT(*) with same WHERE clause

### 5. End-to-End Flow

```
1. User Action (Dashboard)
   User clicks "Next Run" column header
     → JobsTable emits @sort event
     → JobsView calls table.setSort('next_run_at')

2. Composable State Update
   setSort('next_run_at')
     → sortBy.value = 'next_run_at'
     → sortOrder.value = 'asc' (toggled)
     → page.value = 1 (reset)
     → queryParams computed updates

3. TanStack Query Reacts
   queryKey changes: ['jobs', 'list', { limit: 25, offset: 0, sortBy: 'next_run_at', ... }]
     → Query marked stale
     → fetchFn called with new params
     → keepPreviousData shows old data during fetch

4. API Request
   GET /api/v1/jobs?limit=25&offset=0&sort_by=next_run_at&sort_order=asc
     → Backend validates sort_by against whitelist
     → Builds SQL with ORDER BY next_run_at ASC NULLS LAST
     → Returns { jobs: [...], total: 324, limit: 25, offset: 0 }

5. UI Updates
   Query resolves
     → items.value = response.jobs
     → total.value = response.total
     → totalPages = Math.ceil(324/25) = 13
     → Pagination shows "Page 1 of 13"
     → Column header shows sort indicator ↑
```

### 6. Error Handling & Resilience

**Composable Layer (fail-safe):**
```typescript
function setSort(field: string) {
  if (!allowedSortFields.includes(field)) {
    if (import.meta.env.DEV) console.warn(`Invalid sort field: ${field}`)
    return  // No state change, no refetch
  }
}
```

**Network Layer (TanStack Query):**
```typescript
const query = useQuery({
  retry: 2,
  retryDelay: (attempt) => Math.min(1000 * 2 ** attempt, 10000),
  placeholderData: keepPreviousData,  // Show stale data during refetch
})
```

**Backend Layer (Go):**
```go
if limit > MaxPageSize {
    limit = MaxPageSize
}
if _, ok := allowedSortFields[sortBy]; !ok {
    sortBy = "created_at"  // Fall back to default
}
```

**UI Layer (Vue):**
```vue
<Alert v-if="table.hasError" variant="destructive">
  Failed to load jobs. <Button @click="table.refetch()">Retry</Button>
</Alert>
<JobsTable :items="table.items" :loading="table.isRefetching" />
```

**Resilience Principles:**
- Never lose user context (errors don't reset page/sort/filters)
- Show stale data instead of empty state during failures
- Automatic retries for transient network errors
- Invalid params degrade to defaults, never crash

---

## Implementation Checklist

### Backend (Crawler Service)

- [ ] Add `sort_by`, `sort_order` query param parsing to `ListJobs` handler
- [ ] Create `allowedSortFields` whitelist map
- [ ] Add `NULLS LAST` to ORDER BY clause
- [ ] Update repository `List()` to accept sort params
- [ ] Update repository `Count()` to use same WHERE clause
- [ ] Add `MaxPageSize` constant (250)
- [ ] Echo `sort_by`, `sort_order` in response
- [ ] Add `search` filter (ILIKE on source_name, url)

### Frontend (Dashboard)

- [ ] Create `useServerPaginatedTable.ts` composable
- [ ] Create `types/table.ts` with `FetchParams`, `PaginatedResponse`
- [ ] Update `features/intake/api/jobs.ts` to pass all params
- [ ] Refactor `useJobs.ts` to use new composable
- [ ] Update `JobsTable.vue` with sortable column headers
- [ ] Add page size selector to table footer
- [ ] Add sort indicators to headers
- [ ] Update `JobStatsCard.vue` to use `total` from API response
- [ ] Remove client-side `paginatedJobs` slicing

### Documentation

- [ ] Document API changes in crawler README
- [ ] Document composable API in dashboard README
- [ ] Add JSDoc to composable functions

### Testing

- [ ] Backend: Test sort field validation
- [ ] Backend: Test limit clamping
- [ ] Backend: Test total count accuracy with filters
- [ ] Frontend: Test page reset on sort change
- [ ] Frontend: Test error state preservation
- [ ] E2E: Verify 320+ jobs paginate correctly

---

## Files to Modify

| File | Changes |
|------|---------|
| `crawler/internal/api/jobs_handler.go` | Add sort params, validation |
| `crawler/internal/api/helpers.go` | Add `parseSortParams` helper |
| `crawler/internal/database/job_repository.go` | Update `List()` signature |
| `dashboard/src/composables/useServerPaginatedTable.ts` | New file |
| `dashboard/src/types/table.ts` | New file |
| `dashboard/src/features/intake/api/jobs.ts` | Pass pagination params |
| `dashboard/src/features/intake/composables/useJobs.ts` | Use new composable |
| `dashboard/src/components/domain/jobs/JobsTable.vue` | Sortable headers, page size |
| `dashboard/src/components/domain/jobs/JobStatsCard.vue` | Use API total |

---

## Future Considerations

1. **URL sync** - Persist table state in query params for shareable links
2. **Column visibility** - Let users hide/show columns
3. **Saved views** - Store filter/sort presets
4. **Export** - Download filtered results as CSV
5. **Infinite scroll** - Alternative to pagination for some views

---

## References

- [TanStack Query Pagination](https://tanstack.com/query/latest/docs/framework/vue/guides/paginated-queries)
- [Vue 3 Composables](https://vuejs.org/guide/reusability/composables.html)
- Existing patterns: `dashboard/src/features/intake/composables/useJobs.ts`
