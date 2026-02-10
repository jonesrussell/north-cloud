/**
 * Discovered Links API
 */

import { crawlerApi } from '@/api/client'
import type { FetchParams, PaginatedResponse } from '@/types/table'

export interface DiscoveredLink {
  id: string
  source_id: string
  source_name: string
  url: string
  parent_url: string | null
  depth: number
  discovered_at: string
  status: string
  priority: number
}

export interface DiscoveredLinkFilters {
  search?: string
  status?: string
  source_id?: string
}

export async function fetchDiscoveredLinksPaginated(
  params: FetchParams<DiscoveredLinkFilters>
): Promise<PaginatedResponse<DiscoveredLink>> {
  const queryParams: Record<string, string | number> = {
    limit: params.limit,
    offset: params.offset,
    sort: params.sortBy,
    order: params.sortOrder,
  }
  if (params.filters?.search) queryParams.search = params.filters.search
  if (params.filters?.status) queryParams.status = params.filters.status
  if (params.filters?.source_id) queryParams.source_id = params.filters.source_id

  const response = await crawlerApi.discoveredLinks.list(queryParams)
  const links = response.data?.links || []
  const total = response.data?.total ?? links.length

  return {
    items: Array.isArray(links) ? links : [],
    total,
  }
}
