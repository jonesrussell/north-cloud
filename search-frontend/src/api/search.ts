import axios, { type AxiosInstance, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios'
import type { SearchRequest, SearchResponse, SuggestResponse } from '@/types/search'
import type { SearchApi } from '@/types/api'

const DEBUG = import.meta.env.DEV

const searchClient: AxiosInstance = axios.create({
  baseURL: '/api/search',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// Debug interceptors (development only)
if (DEBUG) {
  searchClient.interceptors.request.use((config: InternalAxiosRequestConfig) => {
    console.log('[Search API] Request:', config.method?.toUpperCase(), config.url, config.data)
    return config
  })

  searchClient.interceptors.response.use(
    (response: AxiosResponse) => {
      console.log('[Search API] Response:', response.status, response.data)
      return response
    },
    (error: unknown) => {
      if (axios.isAxiosError(error)) {
        console.error('[Search API] Error:', error.response?.status, error.response?.data || error.message)
      } else {
        console.error('[Search API] Error:', error)
      }
      return Promise.reject(error)
    }
  )
}

export const searchApi: SearchApi = {
  /**
   * Execute search with filters (complex queries)
   */
  search: (payload: SearchRequest): Promise<AxiosResponse<SearchResponse>> => {
    return searchClient.post<SearchResponse>('', payload)
  },

  /**
   * Simple search via query parameters
   */
  simpleSearch: (params: Record<string, unknown>): Promise<AxiosResponse<SearchResponse>> => {
    return searchClient.get<SearchResponse>('', { params })
  },

  /**
   * Autocomplete suggestions
   */
  suggest: (query: string): Promise<AxiosResponse<SuggestResponse>> => {
    return searchClient.get<SuggestResponse>('/suggest', { params: { q: query } })
  },

  /**
   * Health check for search service
   */
  health: (): Promise<AxiosResponse<{ status: string }>> => {
    return axios.get<{ status: string }>('/api/health/search')
  },
}

export default searchApi

