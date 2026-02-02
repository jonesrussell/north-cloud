/**
 * Jobs Query Composables
 *
 * TanStack Query hooks for fetching individual job data.
 * For job lists with pagination/sorting, use useJobs composable instead.
 */

import { useQuery, useInfiniteQuery, type UseQueryOptions } from '@tanstack/vue-query'
import { computed, toValue, type MaybeRefOrGetter } from 'vue'
import {
  jobsKeys,
  fetchJob,
  fetchJobExecutions,
  fetchJobStats,
  fetchJobLogs,
  type JobExecutionsResponse,
} from '../api/jobs'
import type { Job, JobStats } from '@/types/crawler'

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

