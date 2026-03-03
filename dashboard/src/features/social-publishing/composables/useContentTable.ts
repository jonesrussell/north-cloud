/**
 * useContentTable - Server-paginated content for the Social Content table view.
 */

import { computed } from 'vue'
import { useServerPaginatedTable } from '@/composables/useServerPaginatedTable'
import { fetchContentPaginated } from '../api/socialPublisher'
import type { SocialContent, ContentFilters } from '@/types/socialPublisher'

const CONTENT_SORT_FIELDS = ['created_at', 'type', 'title', 'source']

export function useContentTable() {
  const table = useServerPaginatedTable<SocialContent, ContentFilters>({
    fetchFn: fetchContentPaginated,
    queryKeyPrefix: 'social-content',
    defaultLimit: 25,
    defaultSortBy: 'created_at',
    defaultSortOrder: 'desc',
    allowedSortFields: CONTENT_SORT_FIELDS,
    allowedPageSizes: [10, 25, 50, 100],
  })

  const hasActiveFilters = computed(() => {
    const f = table.filters.value
    return !!(f.status || f.type)
  })

  const activeFilterCount = computed(() => {
    const f = table.filters.value
    let count = 0
    if (f.status) count++
    if (f.type) count++
    return count
  })

  return {
    items: table.items,
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
    setFilter: (key: keyof ContentFilters, value: ContentFilters[keyof ContentFilters]) => {
      table.setFilters({ [key]: value } as Partial<ContentFilters>)
    },
    clearFilters: table.clearFilters,
    hasActiveFilters,
    activeFilterCount,

    refetch: table.refetch,
  }
}
