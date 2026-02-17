/**
 * useFrontierTable - Server-paginated frontier URLs for FrontierView.
 */

import { computed } from 'vue'
import { useServerPaginatedTable } from '@/composables/useServerPaginatedTable'
import { fetchFrontierPaginated } from '../api/frontier'
import type { FrontierURL, FrontierFilters } from '../api/frontier'

const FRONTIER_SORT_FIELDS = ['priority', 'next_fetch_at', 'created_at']

export function useFrontierTable() {
  const table = useServerPaginatedTable<FrontierURL, FrontierFilters>({
    fetchFn: fetchFrontierPaginated,
    queryKeyPrefix: 'frontier',
    defaultLimit: 25,
    defaultSortBy: 'priority',
    defaultSortOrder: 'desc',
    allowedSortFields: FRONTIER_SORT_FIELDS,
    allowedPageSizes: [10, 25, 50, 100],
  })

  const hasActiveFilters = computed(() => {
    const f = table.filters.value
    return !!(f.search || f.status || f.source_id || f.host || f.origin)
  })

  const activeFilterCount = computed(() => {
    const f = table.filters.value
    let count = 0
    if (f.search) count++
    if (f.status) count++
    if (f.source_id) count++
    if (f.host) count++
    if (f.origin) count++
    return count
  })

  return {
    urls: table.items,
    total: table.total,
    isLoading: table.isLoading,
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
    setFilter: (
      key: keyof FrontierFilters,
      value: FrontierFilters[keyof FrontierFilters]
    ) => {
      table.setFilters({ [key]: value } as Partial<FrontierFilters>)
    },
    clearFilters: table.clearFilters,
    hasActiveFilters,
    activeFilterCount,

    refetch: table.refetch,
  }
}
