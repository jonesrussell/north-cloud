/**
 * useReputationTable - Server-paginated source reputation for ReputationView.
 */

import { computed } from 'vue'
import { useServerPaginatedTable } from '@/composables/useServerPaginatedTable'
import { fetchReputationPaginated } from '../api/reputation'
import type { SourceReputation, ReputationFilters } from '../api/reputation'

const REPUTATION_SORT_FIELDS = [
  'name',
  'reputation',
  'category',
  'total_classified',
  'total_articles',
  'last_classified_at',
  'last_updated',
]

export function useReputationTable() {
  const table = useServerPaginatedTable<SourceReputation, ReputationFilters>({
    fetchFn: fetchReputationPaginated,
    queryKeyPrefix: 'reputation',
    defaultLimit: 25,
    defaultSortBy: 'reputation',
    defaultSortOrder: 'desc',
    allowedSortFields: REPUTATION_SORT_FIELDS,
    allowedPageSizes: [10, 25, 50, 100],
  })

  const hasActiveFilters = computed(() => {
    const f = table.filters.value
    return !!(f.search || f.category)
  })

  const activeFilterCount = computed(() => {
    const f = table.filters.value
    let count = 0
    if (f.search) count++
    if (f.category) count++
    return count
  })

  return {
    sources: table.items,
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
    setFilter: (key: keyof ReputationFilters, value: ReputationFilters[keyof ReputationFilters]) => {
      table.setFilters({ [key]: value } as Partial<ReputationFilters>)
    },
    clearFilters: table.clearFilters,
    hasActiveFilters,
    activeFilterCount,

    refetch: table.refetch,
  }
}
