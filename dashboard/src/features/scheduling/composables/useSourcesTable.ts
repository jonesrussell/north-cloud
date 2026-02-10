/**
 * useSourcesTable - Server-paginated sources for the Sources table view.
 */

import { computed } from 'vue'
import { useServerPaginatedTable } from '@/composables/useServerPaginatedTable'
import { fetchSourcesPaginated } from '../api/sources'
import type { Source, SourceFilters } from '../api/sources'

const SOURCES_SORT_FIELDS = ['name', 'url', 'enabled', 'created_at']

export function useSourcesTable() {
  const table = useServerPaginatedTable<Source, SourceFilters>({
    fetchFn: fetchSourcesPaginated,
    queryKeyPrefix: 'sources-table',
    defaultLimit: 25,
    defaultSortBy: 'name',
    defaultSortOrder: 'asc',
    allowedSortFields: SOURCES_SORT_FIELDS,
    allowedPageSizes: [10, 25, 50, 100],
  })

  const hasActiveFilters = computed(() => {
    const f = table.filters.value
    return !!(f.search || f.enabled !== undefined)
  })

  const activeFilterCount = computed(() => {
    const f = table.filters.value
    let count = 0
    if (f.search) count++
    if (f.enabled !== undefined) count++
    return count
  })

  return {
    sources: table.items,
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
    setFilter: (key: keyof SourceFilters, value: SourceFilters[keyof SourceFilters]) => {
      table.setFilters({ [key]: value } as Partial<SourceFilters>)
    },
    clearFilters: table.clearFilters,
    hasActiveFilters,
    activeFilterCount,

    refetch: table.refetch,
  }
}
