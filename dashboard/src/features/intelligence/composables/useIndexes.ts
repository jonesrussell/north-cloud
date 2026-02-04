import { computed } from 'vue'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { useServerPaginatedTable } from '@/composables/useServerPaginatedTable'
import { fetchIndexes, fetchIndexStats, deleteIndex, indexesKeys } from '../api/indexes'
import type { Index, IndexFilters } from '@/types/indexManager'

const INDEXES_SORT_FIELDS = ['name', 'document_count', 'size', 'health', 'type']

export function useIndexes() {
  const queryClient = useQueryClient()

  // Server-paginated table
  const table = useServerPaginatedTable<Index, IndexFilters>({
    fetchFn: fetchIndexes,
    queryKeyPrefix: 'indexes',
    defaultLimit: 25,
    defaultSortBy: 'name',
    defaultSortOrder: 'asc',
    allowedSortFields: INDEXES_SORT_FIELDS,
    allowedPageSizes: [10, 25, 50, 100],
  })

  // Stats query (for cards)
  const statsQuery = useQuery({
    queryKey: indexesKeys.stats(),
    queryFn: fetchIndexStats,
    staleTime: 30_000, // 30 seconds
  })

  // Delete mutation
  const deleteMutation = useMutation({
    mutationFn: deleteIndex,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: indexesKeys.lists() })
      queryClient.invalidateQueries({ queryKey: indexesKeys.stats() })
    },
  })

  // Helper: Set a single filter
  function setFilter<K extends keyof IndexFilters>(key: K, value: IndexFilters[K]) {
    table.setFilters({ [key]: value } as Partial<IndexFilters>)
  }

  // Helper: Check if filters are active
  const hasActiveFilters = computed(() => {
    const f = table.filters.value
    return !!(f.search || f.type || f.health || f.source)
  })

  // Helper: Count active filters
  const activeFilterCount = computed(() => {
    const f = table.filters.value
    let count = 0
    if (f.search) count++
    if (f.type) count++
    if (f.health) count++
    if (f.source) count++
    return count
  })

  return {
    // Data
    indexes: table.items,
    totalIndexes: table.total,
    stats: computed(() => statsQuery.data.value),
    statsLoading: computed(() => statsQuery.isLoading.value),
    isLoading: table.isLoading,
    isFetching: table.isRefetching,
    error: table.error,
    hasError: table.hasError,

    // Pagination
    page: table.page,
    pageSize: table.pageSize,
    totalPages: table.totalPages,
    hasNextPage: table.hasNextPage,
    hasPreviousPage: table.hasPreviousPage,
    allowedPageSizes: table.allowedPageSizes,
    setPage: table.setPage,
    setPageSize: table.setPageSize,
    nextPage: table.nextPage,
    prevPage: table.prevPage,

    // Sorting
    sortBy: table.sortBy,
    sortOrder: table.sortOrder,
    toggleSort: table.toggleSort,

    // Filters
    filters: table.filters,
    setFilter,
    setFilters: table.setFilters,
    clearFilters: table.clearFilters,
    hasActiveFilters,
    activeFilterCount,

    // Mutations
    deleteIndex: (name: string) => deleteMutation.mutateAsync(name),
    isDeleting: computed(() => deleteMutation.isPending.value),

    // Utilities
    refetch: table.refetch,
    reset: table.reset,
  }
}
