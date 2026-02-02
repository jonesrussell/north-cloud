/**
 * Server-side Paginated Table Composable
 *
 * Manages pagination, sorting, and filtering state for server-driven tables.
 * Integrates with TanStack Query for caching and automatic refetching.
 *
 * @example
 * ```ts
 * const table = useServerPaginatedTable({
 *   fetchFn: fetchJobs,
 *   queryKeyPrefix: 'jobs',
 *   allowedSortFields: ['created_at', 'status', 'next_run_at'],
 * })
 *
 * // In template
 * <tr v-for="item in table.items.value" :key="item.id">
 * <Pagination :page="table.page.value" @change="table.setPage" />
 * ```
 */

import { ref, computed } from 'vue'
import { useQuery, keepPreviousData } from '@tanstack/vue-query'
import type {
  FetchParams,
  UseServerPaginatedTableOptions,
} from '@/types/table'

// ============================================================================
// Constants
// ============================================================================

const DEFAULT_LIMIT = 25
const DEFAULT_SORT_BY = 'created_at'
const DEFAULT_SORT_ORDER = 'desc' as const
const DEFAULT_PAGE_SIZES = [10, 25, 50, 100]

// ============================================================================
// Helper Functions
// ============================================================================

/**
 * Remove undefined and empty string values from an object.
 * Prevents cache fragmentation in TanStack Query.
 */
function normalizeFilters<T extends Record<string, unknown>>(obj: T): T {
  return Object.fromEntries(
    Object.entries(obj).filter(([, v]) => v !== undefined && v !== '')
  ) as T
}

// ============================================================================
// Composable
// ============================================================================

export function useServerPaginatedTable<T, F extends Record<string, unknown> = Record<string, unknown>>(
  options: UseServerPaginatedTableOptions<T, F>
) {
  // ---------------------------------------------------------------------------
  // State: Pagination
  // ---------------------------------------------------------------------------

  const page = ref(1)
  const pageSize = ref(options.defaultLimit ?? DEFAULT_LIMIT)
  const sortBy = ref(options.defaultSortBy ?? DEFAULT_SORT_BY)
  const sortOrder = ref<'asc' | 'desc'>(options.defaultSortOrder ?? DEFAULT_SORT_ORDER)
  const filters = ref<F>({} as F)

  const allowedPageSizes = options.allowedPageSizes ?? DEFAULT_PAGE_SIZES

  // ---------------------------------------------------------------------------
  // Computed: Query Params
  // ---------------------------------------------------------------------------

  const queryParams = computed<FetchParams<F>>(() => ({
    limit: pageSize.value,
    offset: (page.value - 1) * pageSize.value,
    sortBy: sortBy.value,
    sortOrder: sortOrder.value,
    filters: normalizeFilters(filters.value),
  }))

  // ---------------------------------------------------------------------------
  // TanStack Query
  // ---------------------------------------------------------------------------

  const {
    data,
    isLoading,
    isFetching,
    error,
    refetch,
  } = useQuery({
    queryKey: computed(() => [options.queryKeyPrefix, 'list', queryParams.value]),
    queryFn: () => options.fetchFn(queryParams.value),
    staleTime: 10_000,
    refetchInterval: options.refetchInterval,
    placeholderData: keepPreviousData,
  })

  // ---------------------------------------------------------------------------
  // Computed: Derived State
  // ---------------------------------------------------------------------------

  const items = computed<T[]>(() => data.value?.items ?? [])
  const total = computed(() => data.value?.total ?? 0)

  const totalPages = computed(() => {
    if (total.value === 0) return 1
    return Math.ceil(total.value / pageSize.value)
  })

  const hasNextPage = computed(() => page.value < totalPages.value)
  const hasPreviousPage = computed(() => page.value > 1)
  const hasError = computed(() => !!error.value)
  const initialLoadDone = computed(() => !isLoading.value && data.value !== undefined)

  // ---------------------------------------------------------------------------
  // Actions: Pagination
  // ---------------------------------------------------------------------------

  function setPage(newPage: number) {
    const clamped = Math.max(1, Math.min(newPage, totalPages.value || 1))
    if (page.value === clamped) return
    page.value = clamped
  }

  function setPageSize(newSize: number) {
    if (!allowedPageSizes.includes(newSize)) return
    if (pageSize.value === newSize) return
    pageSize.value = newSize
    page.value = 1
  }

  function nextPage() {
    setPage(page.value + 1)
  }

  function prevPage() {
    setPage(page.value - 1)
  }

  // ---------------------------------------------------------------------------
  // Actions: Sorting
  // ---------------------------------------------------------------------------

  function setSort(field: string, order?: 'asc' | 'desc') {
    if (!options.allowedSortFields.includes(field)) {
      if (import.meta.env.DEV) {
        console.warn(`[useServerPaginatedTable] Invalid sort field: ${field}`)
      }
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

  function toggleSort(field: string) {
    setSort(field)
  }

  // ---------------------------------------------------------------------------
  // Actions: Filtering
  // ---------------------------------------------------------------------------

  function setFilters(newFilters: Partial<F>) {
    const merged = { ...filters.value, ...newFilters }
    filters.value = normalizeFilters(merged) as F
    page.value = 1
  }

  function replaceFilters(next: F) {
    filters.value = normalizeFilters(next) as F
    page.value = 1
  }

  function clearFilters() {
    filters.value = {} as F
    page.value = 1
  }

  // ---------------------------------------------------------------------------
  // Actions: Reset
  // ---------------------------------------------------------------------------

  function reset() {
    page.value = 1
    pageSize.value = options.defaultLimit ?? DEFAULT_LIMIT
    sortBy.value = options.defaultSortBy ?? DEFAULT_SORT_BY
    sortOrder.value = options.defaultSortOrder ?? DEFAULT_SORT_ORDER
    filters.value = {} as F
  }

  // ---------------------------------------------------------------------------
  // Return
  // ---------------------------------------------------------------------------

  return {
    // Data state
    items,
    total,
    isLoading,
    isRefetching: isFetching,
    error,
    hasError,
    initialLoadDone,

    // Pagination state
    page,
    pageSize,
    totalPages,
    hasNextPage,
    hasPreviousPage,
    allowedPageSizes,

    // Sorting state
    sortBy,
    sortOrder,

    // Filter state
    filters,

    // Pagination actions
    setPage,
    setPageSize,
    nextPage,
    prevPage,

    // Sorting actions
    setSort,
    toggleSort,

    // Filter actions
    setFilters,
    replaceFilters,
    clearFilters,

    // Utilities
    reset,
    refetch,

    // Raw query for advanced usage
    query: { data, isLoading, isFetching, error, refetch },
  }
}
