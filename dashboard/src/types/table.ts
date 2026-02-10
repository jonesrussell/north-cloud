import type { ComputedRef, Ref } from 'vue'

/**
 * Unified interface for paginated table controllers.
 * Both useServerPaginatedTable and domain composables (useJobs, useIndexes, etc.)
 * expose this shape so FilterBars and Tables are drop-in interchangeable.
 */
export interface PaginatedTableController<T, F = Record<string, unknown>> {
  items: Ref<T[]>
  total: Ref<number>
  page: Ref<number>
  pageSize: Ref<number>
  totalPages: Ref<number>
  allowedPageSizes: readonly number[]

  sortBy: Ref<string>
  sortOrder: Ref<'asc' | 'desc'>
  toggleSort: (key: string) => void

  filters: Ref<F>
  setFilter: (key: keyof F, value: F[keyof F]) => void
  clearFilters: () => void
  hasActiveFilters: ComputedRef<boolean>
  activeFilterCount: ComputedRef<number>

  setPage: (n: number) => void
  setPageSize: (n: number) => void

  isLoading: Ref<boolean>
  error: Ref<Error | null>
  refetch: () => void
}

/**
 * Server-side paginated table types.
 * Used by useServerPaginatedTable composable.
 */

/**
 * Parameters sent to the fetch function.
 */
export interface FetchParams<F = Record<string, unknown>> {
  limit: number
  offset: number
  sortBy: string
  sortOrder: 'asc' | 'desc'
  filters?: F
}

/**
 * Expected response shape from paginated API endpoints.
 */
export interface PaginatedResponse<T> {
  items: T[]
  total: number
}

/**
 * Configuration options for useServerPaginatedTable.
 */
export interface UseServerPaginatedTableOptions<T, F = Record<string, unknown>> {
  /** Function to fetch data from the server */
  fetchFn: (params: FetchParams<F>) => Promise<PaginatedResponse<T>>

  /** Prefix for TanStack Query cache keys (e.g., 'jobs') */
  queryKeyPrefix: string

  /** Default items per page (default: 25) */
  defaultLimit?: number

  /** Default sort column (default: 'created_at') */
  defaultSortBy?: string

  /** Default sort direction (default: 'desc') */
  defaultSortOrder?: 'asc' | 'desc'

  /** Whitelist of sortable field names */
  allowedSortFields: string[]

  /** Allowed page size options (default: [10, 25, 50, 100]) */
  allowedPageSizes?: number[]

  /** Auto-refresh interval in ms (optional) */
  refetchInterval?: number

  /** Debounce filter changes in ms (optional) */
  debounceMs?: number
}

/**
 * Sort state for a table column.
 */
export interface SortState {
  field: string
  order: 'asc' | 'desc'
}
