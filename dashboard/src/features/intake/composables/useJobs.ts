/**
 * Jobs Composable
 *
 * Main composable for the Jobs feature. Combines query state, UI state, and mutations
 * into a single convenient API for components.
 *
 * @example
 * ```ts
 * const jobs = useJobs()
 *
 * // Access data
 * jobs.jobs           // Job list from server
 * jobs.isLoading      // Loading state
 * jobs.error          // Error state
 *
 * // Query parameters
 * jobs.filters        // Current filters
 * jobs.setFilter('status', 'running')
 *
 * // UI state
 * jobs.ui.modals.create
 * jobs.ui.openModal('create')
 *
 * // Mutations
 * jobs.createJob(data)
 * jobs.pauseJob(id)
 * ```
 */

import { computed, toRef } from 'vue'
import { useJobsQueryStore } from '../stores/useJobsQueryStore'
import { useJobsUIStore } from '../stores/useJobsUIStore'
import {
  useJobsListQuery,
  useJobQuery,
  useJobExecutionsQuery,
  useJobStatsQuery,
  useJobStatusCounts,
  useActiveJobsCount,
  useFailedJobsCount,
} from './useJobsQuery'
import {
  useCreateJobMutation,
  useUpdateJobMutation,
  useDeleteJobMutation,
  usePauseJobMutation,
  useResumeJobMutation,
  useCancelJobMutation,
  useRetryJobMutation,
  useBulkPauseJobsMutation,
  useBulkDeleteJobsMutation,
} from './useJobMutations'
import type { CreateJobRequest, UpdateJobRequest } from '@/types/crawler'

// ============================================================================
// Main Jobs Composable
// ============================================================================

/**
 * Main composable for jobs list view
 */
export function useJobs() {
  // Stores
  const queryStore = useJobsQueryStore()
  const uiStore = useJobsUIStore()

  // Queries
  const listQuery = useJobsListQuery()
  const statusCounts = useJobStatusCounts()
  const activeJobsCount = useActiveJobsCount()
  const failedJobsCount = useFailedJobsCount()

  // Mutations
  const createMutation = useCreateJobMutation()
  const updateMutation = useUpdateJobMutation()
  const deleteMutation = useDeleteJobMutation()
  const pauseMutation = usePauseJobMutation()
  const resumeMutation = useResumeJobMutation()
  const cancelMutation = useCancelJobMutation()
  const retryMutation = useRetryJobMutation()
  const bulkPauseMutation = useBulkPauseJobsMutation()
  const bulkDeleteMutation = useBulkDeleteJobsMutation()

  // ---------------------------------------------------------------------------
  // Computed: Data
  // ---------------------------------------------------------------------------

  const jobs = computed(() => listQuery.data.value?.jobs || [])
  const totalJobs = computed(() => listQuery.data.value?.total || 0)
  const isLoading = computed(() => listQuery.isLoading.value)
  const isFetching = computed(() => listQuery.isFetching.value)
  const error = computed(() => listQuery.error.value)

  // ---------------------------------------------------------------------------
  // Computed: Pagination
  // ---------------------------------------------------------------------------

  const totalPages = computed(() => {
    if (totalJobs.value === 0) return 1
    return Math.ceil(totalJobs.value / queryStore.pageSize)
  })

  const hasNextPage = computed(() => queryStore.page < totalPages.value)
  const hasPreviousPage = computed(() => queryStore.page > 1)

  // Client-side pagination of filtered results
  const paginatedJobs = computed(() => {
    const start = (queryStore.page - 1) * queryStore.pageSize
    const end = start + queryStore.pageSize
    return jobs.value.slice(start, end)
  })

  // ---------------------------------------------------------------------------
  // Mutation Actions
  // ---------------------------------------------------------------------------

  async function createJob(data: CreateJobRequest) {
    return createMutation.mutateAsync(data)
  }

  async function updateJob(id: string, data: UpdateJobRequest) {
    return updateMutation.mutateAsync({ id, data })
  }

  async function deleteJob(id: string) {
    return deleteMutation.mutateAsync(id)
  }

  async function pauseJob(id: string) {
    return pauseMutation.mutateAsync(id)
  }

  async function resumeJob(id: string) {
    return resumeMutation.mutateAsync(id)
  }

  async function cancelJob(id: string) {
    return cancelMutation.mutateAsync(id)
  }

  async function retryJob(id: string) {
    return retryMutation.mutateAsync(id)
  }

  async function bulkPauseJobs(ids: string[]) {
    return bulkPauseMutation.mutateAsync(ids)
  }

  async function bulkDeleteJobs(ids: string[]) {
    return bulkDeleteMutation.mutateAsync(ids)
  }

  // ---------------------------------------------------------------------------
  // Utility Actions
  // ---------------------------------------------------------------------------

  function refetch() {
    return listQuery.refetch()
  }

  function resetAllState() {
    queryStore.$reset()
    uiStore.$reset()
  }

  // ---------------------------------------------------------------------------
  // Return
  // ---------------------------------------------------------------------------

  return {
    // Data
    jobs,
    paginatedJobs,
    totalJobs,
    totalPages,
    isLoading,
    isFetching,
    error,

    // Pagination computed
    hasNextPage,
    hasPreviousPage,

    // Status counts
    statusCounts,
    activeJobsCount,
    failedJobsCount,

    // Query store (filters, pagination, sorting)
    filters: toRef(queryStore, 'filters'),
    page: toRef(queryStore, 'page'),
    pageSize: toRef(queryStore, 'pageSize'),
    sortField: toRef(queryStore, 'sortField'),
    sortOrder: toRef(queryStore, 'sortOrder'),
    hasActiveFilters: computed(() => queryStore.hasActiveFilters),
    activeFilterCount: computed(() => queryStore.activeFilterCount),

    // Query store actions
    setFilter: queryStore.setFilter,
    setMultipleFilters: queryStore.setMultipleFilters,
    clearFilter: queryStore.clearFilter,
    clearAllFilters: queryStore.clearAllFilters,
    toggleStatusFilter: queryStore.toggleStatusFilter,
    isStatusActive: queryStore.isStatusActive,
    setPage: queryStore.setPage,
    setPageSize: queryStore.setPageSize,
    nextPage: queryStore.nextPage,
    previousPage: queryStore.previousPage,
    goToFirstPage: queryStore.goToFirstPage,
    sortBy: queryStore.sortBy,

    // UI store
    ui: uiStore,

    // Mutations
    createJob,
    updateJob,
    deleteJob,
    pauseJob,
    resumeJob,
    cancelJob,
    retryJob,
    bulkPauseJobs,
    bulkDeleteJobs,

    // Mutation states
    isCreating: computed(() => createMutation.isPending.value),
    isUpdating: computed(() => updateMutation.isPending.value),
    isDeleting: computed(() => deleteMutation.isPending.value),
    isPausing: computed(() => pauseMutation.isPending.value),
    isResuming: computed(() => resumeMutation.isPending.value),
    isCancelling: computed(() => cancelMutation.isPending.value),
    isRetrying: computed(() => retryMutation.isPending.value),

    // Utilities
    refetch,
    resetAllState,

    // Raw query for advanced usage
    query: listQuery,
  }
}

// ============================================================================
// Job Detail Composable
// ============================================================================

/**
 * Composable for single job detail view
 */
export function useJobDetail(jobId: string) {
  const uiStore = useJobsUIStore()

  const jobQuery = useJobQuery(jobId)
  const executionsQuery = useJobExecutionsQuery(jobId, { limit: 50, offset: 0 })
  const statsQuery = useJobStatsQuery(jobId)

  const pauseMutation = usePauseJobMutation()
  const resumeMutation = useResumeJobMutation()
  const cancelMutation = useCancelJobMutation()
  const retryMutation = useRetryJobMutation()
  const deleteMutation = useDeleteJobMutation()

  return {
    // Data
    job: jobQuery.data,
    executions: computed(() => executionsQuery.data.value?.executions || []),
    totalExecutions: computed(() => executionsQuery.data.value?.total || 0),
    stats: statsQuery.data,

    // Loading states
    isLoadingJob: jobQuery.isLoading,
    isLoadingExecutions: executionsQuery.isLoading,
    isLoadingStats: statsQuery.isLoading,

    // Errors
    jobError: jobQuery.error,
    executionsError: executionsQuery.error,
    statsError: statsQuery.error,

    // Actions
    pauseJob: () => pauseMutation.mutateAsync(jobId),
    resumeJob: () => resumeMutation.mutateAsync(jobId),
    cancelJob: () => cancelMutation.mutateAsync(jobId),
    retryJob: () => retryMutation.mutateAsync(jobId),
    deleteJob: () => deleteMutation.mutateAsync(jobId),

    // Mutation states
    isPausing: computed(() => pauseMutation.isPending.value),
    isResuming: computed(() => resumeMutation.isPending.value),
    isCancelling: computed(() => cancelMutation.isPending.value),
    isRetrying: computed(() => retryMutation.isPending.value),
    isDeleting: computed(() => deleteMutation.isPending.value),

    // UI
    ui: uiStore,

    // Refetch functions
    refetchJob: () => jobQuery.refetch(),
    refetchExecutions: () => executionsQuery.refetch(),
    refetchStats: () => statsQuery.refetch(),

    // Raw queries
    jobQuery,
    executionsQuery,
    statsQuery,
  }
}
