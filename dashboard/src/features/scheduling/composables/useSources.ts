/**
 * Sources Composable
 *
 * Main composable for the Sources feature. Combines query state and mutations
 * into a single convenient API for components.
 */

import { computed, type MaybeRefOrGetter } from 'vue'
import {
  useSourcesListQuery,
  useSourceQuery,
  useSourceOptions,
  useEnabledSources,
  useSourceById,
} from './useSourcesQuery'
import {
  useCreateSourceMutation,
  useUpdateSourceMutation,
  useDeleteSourceMutation,
  useToggleSourceMutation,
} from './useSourceMutations'
import type { CreateSourceRequest, UpdateSourceRequest } from '../api/sources'

// ============================================================================
// Main Sources Composable
// ============================================================================

/**
 * Main composable for sources list view
 */
export function useSources() {
  // Queries
  const listQuery = useSourcesListQuery()
  const sourceOptions = useSourceOptions()
  const enabledSources = useEnabledSources()

  // Mutations
  const createMutation = useCreateSourceMutation()
  const updateMutation = useUpdateSourceMutation()
  const deleteMutation = useDeleteSourceMutation()
  const toggleMutation = useToggleSourceMutation()

  // Computed: Data
  const sources = computed(() => listQuery.data.value?.sources || [])
  const totalSources = computed(() => listQuery.data.value?.total || 0)
  const isLoading = computed(() => listQuery.isLoading.value)
  const isFetching = computed(() => listQuery.isFetching.value)
  const error = computed(() => listQuery.error.value)

  // Computed: Counts
  const enabledCount = computed(() => enabledSources.value.length)
  const disabledCount = computed(() => sources.value.length - enabledSources.value.length)

  // Actions
  async function createSource(data: CreateSourceRequest) {
    return createMutation.mutateAsync(data)
  }

  async function updateSource(id: string, data: UpdateSourceRequest) {
    return updateMutation.mutateAsync({ id, data })
  }

  async function deleteSource(id: string) {
    return deleteMutation.mutateAsync(id)
  }

  async function toggleSourceEnabled(id: string, is_enabled: boolean) {
    return toggleMutation.mutateAsync({ id, is_enabled })
  }

  function getSourceById(id: string) {
    return sources.value.find((s) => s.id === id)
  }

  function getSourceByName(name: string) {
    return sources.value.find((s) => s.name.toLowerCase() === name.toLowerCase())
  }

  function refetch() {
    return listQuery.refetch()
  }

  return {
    // Data
    sources,
    totalSources,
    enabledSources,
    isLoading,
    isFetching,
    error,

    // Counts
    enabledCount,
    disabledCount,

    // Options for dropdowns
    sourceOptions,

    // Mutations
    createSource,
    updateSource,
    deleteSource,
    toggleSourceEnabled,

    // Mutation states
    isCreating: computed(() => createMutation.isPending.value),
    isUpdating: computed(() => updateMutation.isPending.value),
    isDeleting: computed(() => deleteMutation.isPending.value),
    isToggling: computed(() => toggleMutation.isPending.value),

    // Helpers
    getSourceById,
    getSourceByName,
    refetch,

    // Raw query for advanced usage
    query: listQuery,
  }
}

// ============================================================================
// Source Detail Composable
// ============================================================================

/**
 * Composable for single source detail view
 */
export function useSourceDetail(sourceId: MaybeRefOrGetter<string>) {
  const sourceQuery = useSourceQuery(sourceId)
  const sourceFromCache = useSourceById(sourceId)

  const updateMutation = useUpdateSourceMutation()
  const deleteMutation = useDeleteSourceMutation()
  const toggleMutation = useToggleSourceMutation()

  // Use cached data or fetched data
  const source = computed(() => sourceQuery.data.value || sourceFromCache.value)

  return {
    // Data
    source,
    isLoading: sourceQuery.isLoading,
    error: sourceQuery.error,

    // Actions
    updateSource: (data: UpdateSourceRequest) =>
      updateMutation.mutateAsync({ id: sourceQuery.data.value?.id || '', data }),
    deleteSource: () =>
      deleteMutation.mutateAsync(sourceQuery.data.value?.id || ''),
    toggleEnabled: (is_enabled: boolean) =>
      toggleMutation.mutateAsync({ id: sourceQuery.data.value?.id || '', is_enabled }),

    // Mutation states
    isUpdating: computed(() => updateMutation.isPending.value),
    isDeleting: computed(() => deleteMutation.isPending.value),
    isToggling: computed(() => toggleMutation.isPending.value),

    // Refetch
    refetch: () => sourceQuery.refetch(),

    // Raw query
    query: sourceQuery,
  }
}
