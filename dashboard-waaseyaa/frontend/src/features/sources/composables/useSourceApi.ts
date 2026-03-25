import { computed, type Ref } from 'vue'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { apiClient } from '@/shared/api/client'
import { endpoints } from '@/shared/api/endpoints'
import type { PaginatedResponse } from '@/shared/api/types'
import type {
  Source,
  SourceFormData,
  SourceListParams,
  TestCrawlResult,
  SourceMetadata,
} from '../types'

const SOURCES_KEY = 'sources'

export function useSourceList(params?: Ref<SourceListParams>) {
  return useQuery({
    queryKey: computed(() => [SOURCES_KEY, 'list', params?.value]),
    queryFn: async () => {
      const response = await apiClient.get<PaginatedResponse<Source>>(endpoints.sources.list, {
        params: params?.value,
      })
      return response.data
    },
  })
}

export function useSource(id: Ref<string>) {
  return useQuery({
    queryKey: computed(() => [SOURCES_KEY, 'detail', id.value]),
    queryFn: async () => {
      const response = await apiClient.get<Source>(endpoints.sources.detail(id.value))
      return response.data
    },
    enabled: computed(() => !!id.value),
  })
}

export function useCreateSource() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (data: SourceFormData) => {
      const response = await apiClient.post<Source>(endpoints.sources.create, data)
      return response.data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [SOURCES_KEY] })
    },
  })
}

export function useUpdateSource() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ id, data }: { id: string; data: SourceFormData }) => {
      const response = await apiClient.put<Source>(endpoints.sources.update(id), data)
      return response.data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [SOURCES_KEY] })
    },
  })
}

export function useDeleteSource() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async (id: string) => {
      await apiClient.delete(endpoints.sources.delete(id))
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [SOURCES_KEY] })
    },
  })
}

export function useToggleSource() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: async ({ id, enabled }: { id: string; enabled: boolean }) => {
      const endpoint = enabled ? endpoints.sources.enable(id) : endpoints.sources.disable(id)
      const response = await apiClient.patch<Source>(endpoint)
      return response.data
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [SOURCES_KEY] })
    },
  })
}

export function useTestCrawl() {
  return useMutation({
    mutationFn: async (data: { url: string; selectors: Record<string, unknown> }) => {
      const response = await apiClient.post<TestCrawlResult>(endpoints.sources.testCrawl, data)
      return response.data
    },
  })
}

export function useFetchMetadata() {
  return useMutation({
    mutationFn: async (url: string) => {
      const response = await apiClient.post<SourceMetadata>(endpoints.sources.fetchMetadata, {
        url,
      })
      return response.data
    },
  })
}
