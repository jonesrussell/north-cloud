/**
 * Sources Query Composables
 *
 * TanStack Query hooks for fetching source data.
 */

import { useQuery, type UseQueryOptions } from '@tanstack/vue-query'
import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import {
  sourcesKeys,
  fetchSources,
  fetchSource,
  type Source,
  type SourcesListResponse,
} from '../api/sources'

// ============================================================================
// Sources List Query
// ============================================================================

/**
 * Fetch all sources
 */
export function useSourcesListQuery(
  options?: Partial<UseQueryOptions<SourcesListResponse, Error>>
) {
  return useQuery({
    queryKey: sourcesKeys.lists(),
    queryFn: fetchSources,
    staleTime: 60000, // 1 minute
    ...options,
  })
}

// ============================================================================
// Source Detail Query
// ============================================================================

/**
 * Fetch single source by ID
 */
export function useSourceQuery(
  sourceId: MaybeRefOrGetter<string | undefined>,
  options?: Partial<UseQueryOptions<Source, Error>>
) {
  const id = computed(() => toValue(sourceId))

  return useQuery({
    queryKey: computed(() => id.value ? sourcesKeys.detail(id.value) : ['sources', 'detail', null]),
    queryFn: async () => {
      if (!id.value) {
        throw new Error('Source ID is required')
      }
      return fetchSource(id.value)
    },
    enabled: computed(() => !!id.value),
    staleTime: 60000,
    ...options,
  })
}

// ============================================================================
// Derived Computations
// ============================================================================

/**
 * Get enabled sources only
 */
export function useEnabledSources() {
  const { data } = useSourcesListQuery()

  return computed(() => {
    const sources = data.value?.sources || []
    return sources.filter((s) => s.is_enabled)
  })
}

/**
 * Get source options for dropdowns
 */
export function useSourceOptions() {
  const { data } = useSourcesListQuery()

  return computed(() => {
    const sources = data.value?.sources || []
    return sources.map((s) => ({ id: s.id, name: s.name, url: s.url }))
  })
}

/**
 * Get source by ID from cache
 */
export function useSourceById(sourceId: MaybeRefOrGetter<string | undefined>) {
  const { data } = useSourcesListQuery()
  const id = computed(() => toValue(sourceId))

  return computed(() => {
    if (!id.value) return undefined
    return data.value?.sources.find((s) => s.id === id.value)
  })
}
