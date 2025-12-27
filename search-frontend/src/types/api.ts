import type { AxiosRequestConfig, AxiosResponse } from 'axios'
import type { SearchRequest, SearchResponse } from './search'

/**
 * Search API client interface
 */
export interface SearchApi {
  /**
   * Execute search with filters (complex queries)
   */
  search: (payload: SearchRequest) => Promise<AxiosResponse<SearchResponse>>

  /**
   * Simple search via query parameters
   */
  simpleSearch: (params: Record<string, unknown>) => Promise<AxiosResponse<SearchResponse>>

  /**
   * Health check for search service
   */
  health: () => Promise<AxiosResponse<{ status: string }>>
}

/**
 * Axios request interceptor type
 */
export type RequestInterceptor = (config: AxiosRequestConfig) => AxiosRequestConfig | Promise<AxiosRequestConfig>

/**
 * Axios response interceptor type
 */
export type ResponseInterceptor = {
  onFulfilled?: (response: AxiosResponse) => AxiosResponse | Promise<AxiosResponse>
  onRejected?: (error: unknown) => unknown
}

