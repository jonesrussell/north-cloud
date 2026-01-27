/**
 * Jobs Query Composables
 *
 * TanStack Query hooks for fetching job data.
 * Provides automatic caching, refetching, and loading states.
 */

import { useQuery, useInfiniteQuery, type UseQueryOptions } from '@tanstack/vue-query'
import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import {
  jobsKeys,
  fetchJobs,
  fetchJob,
  fetchJobExecutions,
  fetchJobStats,
  fetchJobLogs,
  type JobsListResponse,
  type JobExecutionsResponse,
} from '../api/jobs'
import { useJobsQueryStore } from '../stores/useJobsQueryStore'
import type { Job, JobStats, JobFilters, JobStatus } from '@/types/crawler'

// ============================================================================
// Jobs List Query
// ============================================================================

/**
 * Fetch jobs list with filters from the query store
 *
 * @example
 * ```ts
 * const { data, isLoading, error } = useJobsListQuery()
 * const jobs = computed(() => data.value?.jobs || [])
 * ```
 */
export function useJobsListQuery(
  options?: Partial<UseQueryOptions<JobsListResponse, Error>>
) {
  const queryStore = useJobsQueryStore()

  return useQuery({
    queryKey: computed(() => jobsKeys.list(queryStore.filters)),
    queryFn: () => fetchJobs(queryStore.filters),
    // Refetch every 30 seconds for near-realtime updates
    refetchInterval: 30000,
    refetchIntervalInBackground: false,
    // Data is fresh for 10 seconds
    staleTime: 10000,
    ...options,
  })
}

/**
 * Fetch jobs list with explicit filters (not from store)
 */
export function useJobsListQueryWithFilters(
  filters: MaybeRefOrGetter<JobFilters | undefined>,
  options?: Partial<UseQueryOptions<JobsListResponse, Error>>
) {
  const filtersValue = computed(() => toValue(filters))

  return useQuery({
    queryKey: computed(() => jobsKeys.list(filtersValue.value)),
    queryFn: () => fetchJobs(filtersValue.value),
    staleTime: 10000,
    ...options,
  })
}

// ============================================================================
// Job Detail Query
// ============================================================================

/**
 * Fetch single job by ID
 *
 * @example
 * ```ts
 * const jobId = ref('job-123')
 * const { data: job, isLoading } = useJobQuery(jobId)
 * ```
 */
export function useJobQuery(
  jobId: MaybeRefOrGetter<string | undefined>,
  options?: Partial<UseQueryOptions<Job, Error>>
) {
  const id = computed(() => toValue(jobId))

  return useQuery({
    queryKey: computed(() => id.value ? jobsKeys.detail(id.value) : ['jobs', 'detail', null]),
    queryFn: async () => {
      if (!id.value) {
        throw new Error('Job ID is required')
      }
      return fetchJob(id.value)
    },
    enabled: computed(() => !!id.value),
    // Refetch every 10 seconds for active job detail views
    refetchInterval: 10000,
    staleTime: 5000,
    ...options,
  })
}

// ============================================================================
// Job Executions Query
// ============================================================================

/**
 * Fetch job executions with pagination
 */
export function useJobExecutionsQuery(
  jobId: MaybeRefOrGetter<string | undefined>,
  params?: MaybeRefOrGetter<{ limit?: number; offset?: number } | undefined>,
  options?: Partial<UseQueryOptions<JobExecutionsResponse, Error>>
) {
  const id = computed(() => toValue(jobId))
  const executionParams = computed(() => toValue(params))

  return useQuery({
    queryKey: computed(() =>
      id.value
        ? jobsKeys.executions(id.value, executionParams.value)
        : ['jobs', 'executions', null]
    ),
    queryFn: async () => {
      if (!id.value) {
        throw new Error('Job ID is required')
      }
      return fetchJobExecutions(id.value, executionParams.value)
    },
    enabled: computed(() => !!id.value),
    staleTime: 30000,
    ...options,
  })
}

/**
 * Infinite scroll query for job executions
 */
export function useJobExecutionsInfiniteQuery(
  jobId: MaybeRefOrGetter<string | undefined>,
  pageSize = 50
) {
  const id = computed(() => toValue(jobId))

  return useInfiniteQuery({
    queryKey: computed(() =>
      id.value ? [...jobsKeys.executions(id.value), 'infinite'] : ['jobs', 'executions', 'infinite', null]
    ),
    queryFn: async ({ pageParam = 0 }) => {
      if (!id.value) {
        throw new Error('Job ID is required')
      }
      const response = await fetchJobExecutions(id.value, {
        limit: pageSize,
        offset: pageParam,
      })
      return {
        executions: response.executions,
        total: response.total,
        nextOffset: pageParam + response.executions.length,
      }
    },
    getNextPageParam: (lastPage) => {
      return lastPage.executions.length === pageSize ? lastPage.nextOffset : undefined
    },
    initialPageParam: 0,
    enabled: computed(() => !!id.value),
  })
}

// ============================================================================
// Job Stats Query
// ============================================================================

/**
 * Fetch job statistics
 */
export function useJobStatsQuery(
  jobId: MaybeRefOrGetter<string | undefined>,
  options?: Partial<UseQueryOptions<JobStats, Error>>
) {
  const id = computed(() => toValue(jobId))

  return useQuery({
    queryKey: computed(() =>
      id.value ? jobsKeys.stats(id.value) : ['jobs', 'stats', null]
    ),
    queryFn: async () => {
      if (!id.value) {
        throw new Error('Job ID is required')
      }
      return fetchJobStats(id.value)
    },
    enabled: computed(() => !!id.value),
    // Stats don't change frequently
    staleTime: 2 * 60 * 1000,
    ...options,
  })
}

// ============================================================================
// Job Logs Query
// ============================================================================

/**
 * Fetch job logs
 */
export function useJobLogsQuery(
  jobId: MaybeRefOrGetter<string | undefined>,
  params?: MaybeRefOrGetter<{ limit?: number; offset?: number; execution?: string } | undefined>,
  options?: Partial<UseQueryOptions<unknown, Error>>
) {
  const id = computed(() => toValue(jobId))
  const logParams = computed(() => toValue(params))

  return useQuery({
    queryKey: computed(() =>
      id.value ? jobsKeys.logs(id.value, logParams.value) : ['jobs', 'logs', null]
    ),
    queryFn: async () => {
      if (!id.value) {
        throw new Error('Job ID is required')
      }
      return fetchJobLogs(id.value, logParams.value)
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
 * Compute status counts from jobs list
 */
export function useJobStatusCounts() {
  const { data } = useJobsListQuery()

  return computed(() => {
    const jobs = data.value?.jobs || []
    const counts: Record<JobStatus, number> = {
      pending: 0,
      scheduled: 0,
      running: 0,
      paused: 0,
      completed: 0,
      failed: 0,
      cancelled: 0,
    }

    for (const job of jobs) {
      if (job.status in counts) {
        counts[job.status]++
      }
    }

    return counts
  })
}

/**
 * Compute active jobs count
 */
export function useActiveJobsCount() {
  const statusCounts = useJobStatusCounts()

  return computed(() => {
    const counts = statusCounts.value
    return counts.running + counts.scheduled + counts.pending
  })
}

/**
 * Compute failed jobs count
 */
export function useFailedJobsCount() {
  const statusCounts = useJobStatusCounts()
  return computed(() => statusCounts.value.failed)
}
