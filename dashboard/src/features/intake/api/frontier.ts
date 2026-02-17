/**
 * Frontier API
 */

import { crawlerApi } from '@/api/client'
import type { FetchParams, PaginatedResponse } from '@/types/table'

export interface FrontierURL {
  id: string
  url: string
  url_hash: string
  host: string
  source_id: string
  origin: string
  parent_url: string | null
  depth: number
  priority: number
  status: string
  next_fetch_at: string
  last_fetched_at: string | null
  fetch_count: number
  content_hash: string | null
  etag: string | null
  last_modified: string | null
  retry_count: number
  last_error: string | null
  discovered_at: string
  created_at: string
  updated_at: string
}

export interface FrontierFilters {
  search?: string
  status?: string
  source_id?: string
  host?: string
  origin?: string
}

export interface FrontierStats {
  total_pending: number
  total_fetching: number
  total_fetched: number
  total_failed: number
  total_dead: number
}

export async function fetchFrontierPaginated(
  params: FetchParams<FrontierFilters>
): Promise<PaginatedResponse<FrontierURL>> {
  const queryParams: Record<string, string | number> = {
    limit: params.limit,
    offset: params.offset,
    sort_by: params.sortBy,
  }
  if (params.filters?.search) queryParams.search = params.filters.search
  if (params.filters?.status) queryParams.status = params.filters.status
  if (params.filters?.source_id) queryParams.source_id = params.filters.source_id
  if (params.filters?.host) queryParams.host = params.filters.host
  if (params.filters?.origin) queryParams.origin = params.filters.origin

  const response = await crawlerApi.frontier.list(queryParams)
  const urls = response.data?.urls || []
  const total = response.data?.total ?? urls.length

  return {
    items: Array.isArray(urls) ? urls : [],
    total,
  }
}

export async function fetchFrontierStats(): Promise<FrontierStats> {
  const response = await crawlerApi.frontier.stats()
  return response.data
}
