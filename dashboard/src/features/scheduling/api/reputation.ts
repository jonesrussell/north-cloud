/**
 * Reputation (Classifier Sources) API
 */

import { classifierApi } from '@/api/client'
import type { FetchParams, PaginatedResponse } from '@/types/table'

export interface SourceReputation {
  name: string
  reputation: number
  category: string
  total_classified: number
  avg_quality: number
  last_updated: string | null
}

export interface ReputationFilters {
  search?: string
  category?: string
}

export async function fetchReputationPaginated(
  params: FetchParams<ReputationFilters>
): Promise<PaginatedResponse<SourceReputation>> {
  const page = Math.floor(params.offset / params.limit) + 1
  const queryParams: Record<string, string | number> = {
    page,
    page_size: params.limit,
    sort_by: params.sortBy,
    sort_order: params.sortOrder,
  }
  if (params.filters?.search) queryParams.search = params.filters.search
  if (params.filters?.category) queryParams.category = params.filters.category

  const response = await classifierApi.sources.list(queryParams)
  const sources = response.data?.sources || []
  const total = response.data?.total ?? sources.length

  return {
    items: Array.isArray(sources) ? sources : [],
    total,
  }
}
