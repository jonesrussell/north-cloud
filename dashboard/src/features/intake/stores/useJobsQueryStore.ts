/**
 * Jobs Query Store
 *
 * Manages query parameters for jobs: filters, pagination, and sorting.
 * This is CLIENT-SIDE state that controls what we fetch from the server.
 * Actual job data lives in TanStack Query cache.
 */

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { JobStatus, JobFilters } from '@/types/crawler'

// ============================================================================
// Constants
// ============================================================================

const DEFAULT_PAGE = 1
const DEFAULT_PAGE_SIZE = 25
const DEFAULT_SORT_FIELD = 'created_at' as const
const DEFAULT_SORT_ORDER = 'desc' as const

// ============================================================================
// Types
// ============================================================================

export type SortField = 'created_at' | 'updated_at' | 'status' | 'source_name' | 'next_run_at'
export type SortOrder = 'asc' | 'desc'

export interface JobsQueryParams {
  status?: JobStatus | JobStatus[]
  source_id?: string
  schedule_enabled?: boolean
  search?: string
  page: number
  pageSize: number
  sortField: SortField
  sortOrder: SortOrder
}

// ============================================================================
// Store
// ============================================================================

export const useJobsQueryStore = defineStore('jobs-query', () => {
  // ---------------------------------------------------------------------------
  // State: Filters
  // ---------------------------------------------------------------------------

  const filters = ref<JobFilters>({
    status: undefined,
    source_id: undefined,
    schedule_enabled: undefined,
    search: undefined,
  })

  // ---------------------------------------------------------------------------
  // State: Pagination
  // ---------------------------------------------------------------------------

  const page = ref(DEFAULT_PAGE)
  const pageSize = ref(DEFAULT_PAGE_SIZE)

  // ---------------------------------------------------------------------------
  // State: Sorting
  // ---------------------------------------------------------------------------

  const sortField = ref<SortField>(DEFAULT_SORT_FIELD)
  const sortOrder = ref<SortOrder>(DEFAULT_SORT_ORDER)

  // ---------------------------------------------------------------------------
  // Computed: Query Parameters
  // ---------------------------------------------------------------------------

  /**
   * Complete query parameters for TanStack Query key
   */
  const queryParams = computed<JobsQueryParams>(() => ({
    status: filters.value.status,
    source_id: filters.value.source_id,
    schedule_enabled: filters.value.schedule_enabled,
    search: filters.value.search,
    page: page.value,
    pageSize: pageSize.value,
    sortField: sortField.value,
    sortOrder: sortOrder.value,
  }))

  /**
   * Stable query key for TanStack Query
   */
  const queryKey = computed(() => ['jobs', 'list', queryParams.value] as const)

  /**
   * Check if any filters are active
   */
  const hasActiveFilters = computed(() => {
    return !!(
      filters.value.status ||
      filters.value.source_id ||
      filters.value.schedule_enabled !== undefined ||
      filters.value.search
    )
  })

  /**
   * Count of active filters
   */
  const activeFilterCount = computed(() => {
    let count = 0
    if (filters.value.status) count++
    if (filters.value.source_id) count++
    if (filters.value.schedule_enabled !== undefined) count++
    if (filters.value.search) count++
    return count
  })

  // ---------------------------------------------------------------------------
  // Actions: Filter Management
  // ---------------------------------------------------------------------------

  function setFilter<K extends keyof JobFilters>(key: K, value: JobFilters[K]) {
    filters.value[key] = value
    page.value = DEFAULT_PAGE // Reset to first page
  }

  function setMultipleFilters(newFilters: Partial<JobFilters>) {
    filters.value = { ...filters.value, ...newFilters }
    page.value = DEFAULT_PAGE
  }

  function clearFilter<K extends keyof JobFilters>(key: K) {
    filters.value[key] = undefined
  }

  function clearAllFilters() {
    filters.value = {
      status: undefined,
      source_id: undefined,
      schedule_enabled: undefined,
      search: undefined,
    }
    page.value = DEFAULT_PAGE
  }

  function toggleStatusFilter(status: JobStatus) {
    const currentStatus = filters.value.status

    if (!currentStatus) {
      filters.value.status = [status]
    } else if (Array.isArray(currentStatus)) {
      const index = currentStatus.indexOf(status)
      if (index > -1) {
        const newStatus = [...currentStatus]
        newStatus.splice(index, 1)
        filters.value.status = newStatus.length > 0 ? newStatus : undefined
      } else {
        filters.value.status = [...currentStatus, status]
      }
    } else if (currentStatus === status) {
      filters.value.status = undefined
    } else {
      filters.value.status = [currentStatus, status]
    }

    page.value = DEFAULT_PAGE
  }

  function isStatusActive(status: JobStatus): boolean {
    const currentStatus = filters.value.status
    if (!currentStatus) return false
    if (Array.isArray(currentStatus)) {
      return currentStatus.includes(status)
    }
    return currentStatus === status
  }

  // ---------------------------------------------------------------------------
  // Actions: Pagination
  // ---------------------------------------------------------------------------

  function setPage(newPage: number) {
    page.value = newPage
  }

  function setPageSize(newSize: number) {
    pageSize.value = newSize
    page.value = DEFAULT_PAGE
  }

  function nextPage() {
    page.value += 1
  }

  function previousPage() {
    if (page.value > 1) {
      page.value -= 1
    }
  }

  function goToFirstPage() {
    page.value = DEFAULT_PAGE
  }

  // ---------------------------------------------------------------------------
  // Actions: Sorting
  // ---------------------------------------------------------------------------

  function setSort(field: SortField, order?: SortOrder) {
    sortField.value = field
    sortOrder.value = order || DEFAULT_SORT_ORDER
    page.value = DEFAULT_PAGE
  }

  function toggleSortOrder() {
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  }

  function sortBy(field: SortField) {
    if (sortField.value === field) {
      toggleSortOrder()
    } else {
      setSort(field, 'asc')
    }
  }

  // ---------------------------------------------------------------------------
  // Actions: Reset
  // ---------------------------------------------------------------------------

  function $reset() {
    clearAllFilters()
    page.value = DEFAULT_PAGE
    pageSize.value = DEFAULT_PAGE_SIZE
    sortField.value = DEFAULT_SORT_FIELD
    sortOrder.value = DEFAULT_SORT_ORDER
  }

  return {
    // State
    filters,
    page,
    pageSize,
    sortField,
    sortOrder,

    // Computed
    queryParams,
    queryKey,
    hasActiveFilters,
    activeFilterCount,

    // Filter Actions
    setFilter,
    setMultipleFilters,
    clearFilter,
    clearAllFilters,
    toggleStatusFilter,
    isStatusActive,

    // Pagination Actions
    setPage,
    setPageSize,
    nextPage,
    previousPage,
    goToFirstPage,

    // Sorting Actions
    setSort,
    toggleSortOrder,
    sortBy,

    // Reset
    $reset,
  }
})
