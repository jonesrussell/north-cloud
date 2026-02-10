/**
 * useDiscoveredLinksTable - Server-paginated discovered links for DiscoveredLinksView.
 */

import { computed } from 'vue'
import { useServerPaginatedTable } from '@/composables/useServerPaginatedTable'
import { fetchDiscoveredLinksPaginated } from '../api/discoveredLinks'
import type { DiscoveredLink, DiscoveredLinkFilters } from '../api/discoveredLinks'

const DISCOVERED_LINKS_SORT_FIELDS = ['priority', 'queued_at', 'discovered_at']

export function useDiscoveredLinksTable() {
  const table = useServerPaginatedTable<DiscoveredLink, DiscoveredLinkFilters>({
    fetchFn: fetchDiscoveredLinksPaginated,
    queryKeyPrefix: 'discovered-links',
    defaultLimit: 25,
    defaultSortBy: 'priority',
    defaultSortOrder: 'desc',
    allowedSortFields: DISCOVERED_LINKS_SORT_FIELDS,
    allowedPageSizes: [10, 25, 50, 100],
  })

  const hasActiveFilters = computed(() => {
    const f = table.filters.value
    return !!(f.search || f.status || f.source_id)
  })

  const activeFilterCount = computed(() => {
    const f = table.filters.value
    let count = 0
    if (f.search) count++
    if (f.status) count++
    if (f.source_id) count++
    return count
  })

  return {
    links: table.items,
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
      key: keyof DiscoveredLinkFilters,
      value: DiscoveredLinkFilters[keyof DiscoveredLinkFilters]
    ) => {
      table.setFilters({ [key]: value } as Partial<DiscoveredLinkFilters>)
    },
    clearFilters: table.clearFilters,
    hasActiveFilters,
    activeFilterCount,

    refetch: table.refetch,
  }
}
