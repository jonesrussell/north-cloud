/**
 * useDiscoveredDomainsTable - Server-paginated discovered domains for DiscoveredDomainsView.
 */

import { computed } from 'vue'
import { useServerPaginatedTable } from '@/composables/useServerPaginatedTable'
import { fetchDiscoveredDomainsPaginated } from '../api/discoveredDomains'
import type { DiscoveredDomain, DiscoveredDomainFilters } from '../api/discoveredDomains'

const DISCOVERED_DOMAINS_SORT_FIELDS = ['link_count', 'source_count', 'last_seen', 'domain']

export function useDiscoveredDomainsTable() {
  const table = useServerPaginatedTable<DiscoveredDomain, DiscoveredDomainFilters>({
    fetchFn: fetchDiscoveredDomainsPaginated,
    queryKeyPrefix: 'discovered-domains',
    defaultLimit: 25,
    defaultSortBy: 'link_count',
    defaultSortOrder: 'desc',
    allowedSortFields: DISCOVERED_DOMAINS_SORT_FIELDS,
    allowedPageSizes: [10, 25, 50, 100],
  })

  const hasActiveFilters = computed(() => {
    const f = table.filters.value
    return !!(f.search || f.status || f.min_score || f.hide_existing)
  })

  const activeFilterCount = computed(() => {
    const f = table.filters.value
    let count = 0
    if (f.search) count++
    if (f.status) count++
    if (f.min_score) count++
    if (f.hide_existing) count++
    return count
  })

  return {
    domains: table.items,
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
      key: keyof DiscoveredDomainFilters,
      value: DiscoveredDomainFilters[keyof DiscoveredDomainFilters],
    ) => {
      table.setFilters({ [key]: value } as Partial<DiscoveredDomainFilters>)
    },
    clearFilters: table.clearFilters,
    hasActiveFilters,
    activeFilterCount,

    refetch: table.refetch,
  }
}
