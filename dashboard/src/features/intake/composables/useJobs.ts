/**
 * Jobs Composable
 *
 * Main composable for the Jobs feature. Uses useServerPaginatedTable
 * for pagination/sorting and adds job-specific mutations.
 *
 * @example
 * ```ts
 * const jobs = useJobs()
 *
 * // Access data
 * jobs.jobs           // Job list from server (current page)
 * jobs.totalJobs      // Total count from server
 * jobs.isLoading      // Loading state
 *
 * // Pagination
 * jobs.page           // Current page
 * jobs.setPage(2)     // Go to page 2
 * jobs.setPageSize(50)// Change page size
 *
 * // Sorting
 * jobs.sortBy         // Current sort field
 * jobs.toggleSort('status') // Toggle sort on column
 *
 * // Filters
 * jobs.setFilters({ status: 'running' })
 *
 * // Mutations
 * jobs.createJob(data)
 * jobs.pauseJob(id)
 * ```
 */

import { computed } from 'vue'
import { useServerPaginatedTable } from '@/composables/useServerPaginatedTable'
import { useJobsUIStore } from '../stores/useJobsUIStore'
import { fetchJobs } from '../api/jobs'
import {
  useJobQuery,
  useJobExecutionsQuery,
  useJobStatsQuery,
} from './useJobsQuery'
import {
  useCreateJobMutation,
  useUpdateJobMutation,
  useDeleteJobMutation,
  usePauseJobMutation,
  useResumeJobMutation,
  useCancelJobMutation,
  useRetryJobMutation,
  useForceRunJobMutation,
  useBulkPauseJobsMutation,
  useBulkDeleteJobsMutation,
} from './useJobMutations'
import type { Job, JobFilters, JobStatus, CreateJobRequest, UpdateJobRequest } from '@/types/crawler'

// Allowed sort fields for jobs table
const JOBS_SORT_FIELDS = [
  'created_at',
  'updated_at',
  'status',
  'source_name',
  'next_run_at',
  'last_run_at',
]

// ============================================================================
// Main Jobs Composable
// ============================================================================

/**
 * Main composable for jobs list view
 */
export function useJobs() {
  // Use the server-paginated table composable
  const table = useServerPaginatedTable<Job, JobFilters>({
    fetchFn: fetchJobs,
    queryKeyPrefix: 'jobs',
    defaultLimit: 25,
    defaultSortBy: 'created_at',
    defaultSortOrder: 'desc',
    allowedSortFields: JOBS_SORT_FIELDS,
    allowedPageSizes: [10, 25, 50, 100],
    refetchInterval: 30_000,
  })

  // UI store
  const uiStore = useJobsUIStore()

  // Mutations
  const createMutation = useCreateJobMutation()
  const updateMutation = useUpdateJobMutation()
  const deleteMutation = useDeleteJobMutation()
  const pauseMutation = usePauseJobMutation()
  const resumeMutation = useResumeJobMutation()
  const cancelMutation = useCancelJobMutation()
  const retryMutation = useRetryJobMutation()
  const forceRunMutation = useForceRunJobMutation()
  const bulkPauseMutation = useBulkPauseJobsMutation()
  const bulkDeleteMutation = useBulkDeleteJobsMutation()

  // ---------------------------------------------------------------------------
  // Computed: Status Counts (from current page data)
  // ---------------------------------------------------------------------------

  const statusCounts = computed(() => {
    const counts: Record<JobStatus, number> = {
      pending: 0,
      scheduled: 0,
      running: 0,
      paused: 0,
      completed: 0,
      failed: 0,
      cancelled: 0,
    }

    for (const job of table.items.value) {
      if (job.status in counts) {
        counts[job.status as JobStatus]++
      }
    }

    return counts
  })

  const activeJobsCount = computed(() => {
    const counts = statusCounts.value
    return counts.running + counts.scheduled + counts.pending
  })

  const failedJobsCount = computed(() => statusCounts.value.failed)

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

  async function forceRunJob(id: string) {
    return forceRunMutation.mutateAsync(id)
  }

  async function bulkPauseJobs(ids: string[]) {
    return bulkPauseMutation.mutateAsync(ids)
  }

  async function bulkDeleteJobs(ids: string[]) {
    return bulkDeleteMutation.mutateAsync(ids)
  }

  // ---------------------------------------------------------------------------
  // Filter Helpers
  // ---------------------------------------------------------------------------

  function toggleStatusFilter(status: JobStatus) {
    const currentStatus = table.filters.value.status

    if (!currentStatus) {
      table.setFilters({ status: [status] })
    } else if (Array.isArray(currentStatus)) {
      const index = currentStatus.indexOf(status)
      if (index > -1) {
        const newStatus = [...currentStatus]
        newStatus.splice(index, 1)
        table.setFilters({ status: newStatus.length > 0 ? newStatus : undefined })
      } else {
        table.setFilters({ status: [...currentStatus, status] })
      }
    } else if (currentStatus === status) {
      table.setFilters({ status: undefined })
    } else {
      table.setFilters({ status: [currentStatus, status] })
    }
  }

  function isStatusActive(status: JobStatus): boolean {
    const currentStatus = table.filters.value.status
    if (!currentStatus) return false
    if (Array.isArray(currentStatus)) {
      return currentStatus.includes(status)
    }
    return currentStatus === status
  }

  function clearAllFilters() {
    table.clearFilters()
  }

  // ---------------------------------------------------------------------------
  // Return
  // ---------------------------------------------------------------------------

  return {
    // Data (from table composable)
    jobs: table.items,
    totalJobs: table.total,
    isLoading: table.isLoading,
    isFetching: table.isRefetching,
    error: table.error,

    // Pagination (from table composable)
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

    // Sorting (from table composable)
    sortBy: table.sortBy,
    sortOrder: table.sortOrder,
    setSort: table.setSort,
    toggleSort: table.toggleSort,

    // Filters
    filters: table.filters,
    setFilters: table.setFilters,
    clearAllFilters,
    toggleStatusFilter,
    isStatusActive,
    hasActiveFilters: computed(() => Object.keys(table.filters.value).length > 0),

    // Status counts (computed from current page)
    statusCounts,
    activeJobsCount,
    failedJobsCount,

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
    forceRunJob,
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
    isForceRunning: computed(() => forceRunMutation.isPending.value),

    // Utilities
    refetch: table.refetch,
    reset: table.reset,
  }
}

// ============================================================================
// Job Detail Composable
// ============================================================================

/**
 * Composable for job detail view
 * Combines job data, executions, stats, and mutations for a single job
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
  const forceRunMutation = useForceRunJobMutation()
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
    forceRunJob: () => forceRunMutation.mutateAsync(jobId),
    deleteJob: () => deleteMutation.mutateAsync(jobId),

    // Mutation states
    isPausing: computed(() => pauseMutation.isPending.value),
    isResuming: computed(() => resumeMutation.isPending.value),
    isCancelling: computed(() => cancelMutation.isPending.value),
    isRetrying: computed(() => retryMutation.isPending.value),
    isForceRunning: computed(() => forceRunMutation.isPending.value),
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
