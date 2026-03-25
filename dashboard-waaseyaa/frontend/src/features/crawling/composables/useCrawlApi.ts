import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import type { Ref } from 'vue'
import { apiClient } from '@/shared/api/client'
import { endpoints } from '@/shared/api/endpoints'
import type { CrawlJob, CrawlJobsResponse, StartCrawlRequest, ControlJobAction } from '../types'

const JOBS_KEY = 'crawl-jobs'
const JOB_KEY = 'crawl-job'

const AUTO_REFRESH_LIST_MS = 10000
const AUTO_REFRESH_DETAIL_MS = 5000

export function useCrawlJobs() {
  return useQuery<CrawlJobsResponse>({
    queryKey: [JOBS_KEY],
    queryFn: async () => {
      const response = await apiClient.get<CrawlJobsResponse>(endpoints.crawler.jobs)
      return response.data
    },
    refetchInterval: AUTO_REFRESH_LIST_MS,
  })
}

export function useCrawlJob(id: Ref<string>) {
  return useQuery<CrawlJob>({
    queryKey: [JOB_KEY, id],
    queryFn: async () => {
      const response = await apiClient.get<CrawlJob>(endpoints.crawler.job(id.value))
      return response.data
    },
    refetchInterval: AUTO_REFRESH_DETAIL_MS,
    enabled: () => !!id.value,
  })
}

export function useStartCrawl() {
  const queryClient = useQueryClient()

  return useMutation<CrawlJob, Error, StartCrawlRequest>({
    mutationFn: async (request) => {
      const response = await apiClient.post<CrawlJob>(endpoints.crawler.jobs, request)
      return response.data
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: [JOBS_KEY] })
    },
  })
}

export function useControlJob() {
  const queryClient = useQueryClient()

  return useMutation<CrawlJob, Error, { id: string } & ControlJobAction>({
    mutationFn: async ({ id, action }) => {
      const url = `${endpoints.crawler.job(id)}/${action}`
      const response = await apiClient.post<CrawlJob>(url)
      return response.data
    },
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({ queryKey: [JOBS_KEY] })
      void queryClient.invalidateQueries({ queryKey: [JOB_KEY, variables.id] })
    },
  })
}
