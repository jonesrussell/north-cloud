import { indexManagerApi } from '@/api/client'
import type { Index, IndexStats, HealthStatus, IndexType, IndexFilters } from '@/types/indexManager'
import type { FetchParams, PaginatedResponse } from '@/types/table'

// Query key factory for TanStack Query cache management
export const indexesKeys = {
  all: ['indexes'] as const,
  lists: () => [...indexesKeys.all, 'list'] as const,
  list: (filters?: IndexFilters) => [...indexesKeys.lists(), filters] as const,
  details: () => [...indexesKeys.all, 'detail'] as const,
  detail: (name: string) => [...indexesKeys.details(), name] as const,
  stats: () => [...indexesKeys.all, 'stats'] as const,
}

// Fetch indexes with pagination/sorting/filtering
export async function fetchIndexes(
  params: FetchParams<IndexFilters>
): Promise<PaginatedResponse<Index>> {
  const queryParams: Record<string, unknown> = {
    limit: params.limit,
    offset: params.offset,
    sortBy: params.sortBy,
    sortOrder: params.sortOrder,
  }

  // Add filters
  if (params.filters?.search) queryParams.search = params.filters.search
  if (params.filters?.type) queryParams.type = params.filters.type
  if (params.filters?.health) queryParams.health = params.filters.health
  if (params.filters?.source) queryParams.source = params.filters.source

  const response = await indexManagerApi.indexes.list(queryParams as Parameters<typeof indexManagerApi.indexes.list>[0])

  return {
    items: response.data?.indices ?? [],
    total: response.data?.total ?? 0,
  }
}

// Fetch stats for cards
export async function fetchIndexStats(): Promise<IndexStats> {
  const response = await indexManagerApi.stats.get()
  return response.data
}

// Delete index
export async function deleteIndex(name: string): Promise<void> {
  await indexManagerApi.indexes.delete(name)
}

// Re-export types for convenience
export type { Index, IndexStats, HealthStatus, IndexType, IndexFilters }
