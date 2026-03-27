import { type ComputedRef, computed } from 'vue'
import { useQueries } from '@tanstack/vue-query'
import { apiClient } from '@/shared/api/client'
import { endpoints } from '@/shared/api/endpoints'

interface PaginatedResponse {
  total: number
}

interface VerificationStats {
  pending: number
}

export interface StatResult {
  value: number | string
  isLoading: boolean
  isError: boolean
  refetch: () => void
}

export interface HomeStats {
  sourceCount: ComputedRef<StatResult>
  runningJobs: ComputedRef<StatResult>
  pendingReview: ComputedRef<StatResult>
  channelCount: ComputedRef<StatResult>
}

export function useHomeStats(): HomeStats {
  const results = useQueries({
    queries: [
      {
        queryKey: ['home', 'sourceCount'],
        queryFn: async () => {
          const response = await apiClient.get<PaginatedResponse>(endpoints.sources.list, {
            params: { per_page: 1 },
          })
          return response.data.total
        },
        staleTime: 30_000,
      },
      {
        queryKey: ['home', 'runningJobs'],
        queryFn: async () => {
          const response = await apiClient.get<{ jobs: unknown[]; total: number }>(
            endpoints.crawler.jobs,
            { params: { status: 'running', limit: 1 } },
          )
          return response.data.total ?? 0
        },
        staleTime: 15_000,
      },
      {
        queryKey: ['home', 'pendingReview'],
        queryFn: async () => {
          const response = await apiClient.get<VerificationStats>(
            `${endpoints.sources.list}/verification/stats`,
          )
          return response.data.pending
        },
        retry: false,
        staleTime: 60_000,
      },
      {
        queryKey: ['home', 'channelCount'],
        queryFn: async () => {
          const response = await apiClient.get<{ channels: unknown[]; count: number }>(
            endpoints.publisher.channels,
          )
          return response.data.count ?? 0
        },
        staleTime: 60_000,
      },
    ],
  })

  function toStat(index: number, fallbackOnError?: string): ComputedRef<StatResult> {
    return computed(() => {
      const query = results.value[index]
      const usesFallback = fallbackOnError !== undefined && query.isError
      return {
        value: usesFallback ? fallbackOnError : (query.data ?? 0),
        isLoading: query.isLoading,
        isError: usesFallback ? false : query.isError,
        refetch: () => { void query.refetch() },
      }
    })
  }

  return {
    sourceCount: toStat(0),
    runningJobs: toStat(1),
    pendingReview: toStat(2, 'N/A'),
    channelCount: toStat(3),
  }
}
