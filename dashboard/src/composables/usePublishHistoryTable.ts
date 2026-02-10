/**
 * usePublishHistoryTable - Server-paginated publish history for DeliveryLogsView and ArticlesView.
 */

import { computed } from 'vue'
import { publisherApi } from '@/api/client'
import { useServerPaginatedTable } from './useServerPaginatedTable'
import type { PublishHistoryItem } from '@/types/publisher'
import type { FetchParams, PaginatedResponse } from '@/types/table'

export interface PublishHistoryFilters {
  channel_name?: string
}

async function fetchPublishHistoryPaginated(
  params: FetchParams<PublishHistoryFilters>
): Promise<PaginatedResponse<PublishHistoryItem>> {
  const queryParams: Record<string, string | number> = {
    limit: params.limit,
    offset: params.offset,
  }
  if (params.filters?.channel_name) {
    queryParams.channel_name = params.filters.channel_name
  }

  const response = await publisherApi.history.list(queryParams)
  const history = response.data?.history || []
  const total = response.data?.total ?? history.length

  return {
    items: Array.isArray(history) ? history : [],
    total,
  }
}

const PUBLISH_HISTORY_SORT_FIELDS: string[] = []
// Publisher API doesn't support sort yet - use default (published_at desc from API)

export function usePublishHistoryTable(options?: { refetchInterval?: number }) {
  const table = useServerPaginatedTable<PublishHistoryItem, PublishHistoryFilters>({
    fetchFn: fetchPublishHistoryPaginated,
    queryKeyPrefix: 'publish-history',
    defaultLimit: 25,
    defaultSortBy: 'published_at',
    defaultSortOrder: 'desc',
    allowedSortFields: PUBLISH_HISTORY_SORT_FIELDS.length > 0 ? PUBLISH_HISTORY_SORT_FIELDS : ['published_at'],
    allowedPageSizes: [10, 25, 50, 100],
    refetchInterval: options?.refetchInterval,
  })

  const hasActiveFilters = computed(() => Boolean(table.filters.value.channel_name))

  const activeFilterCount = computed(() => (table.filters.value.channel_name ? 1 : 0))

  return {
    items: table.items,
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
      key: keyof PublishHistoryFilters,
      value: PublishHistoryFilters[keyof PublishHistoryFilters]
    ) => {
      table.setFilters({ [key]: value } as Partial<PublishHistoryFilters>)
    },
    clearFilters: table.clearFilters,
    hasActiveFilters,
    activeFilterCount,

    refetch: table.refetch,

    clearAllHistory: async (): Promise<{ deleted: number }> => {
      const response = await publisherApi.history.clearAll()
      table.refetch()
      return { deleted: response.data?.deleted || 0 }
    },
  }
}
