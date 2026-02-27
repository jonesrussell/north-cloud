/**
 * Discovered Domains API
 */

import { crawlerApi } from '@/api/client'
import type { FetchParams, PaginatedResponse } from '@/types/table'

export type DomainStatus = 'active' | 'ignored' | 'reviewing' | 'promoted'

export interface DiscoveredDomain {
  domain: string
  status: DomainStatus
  link_count: number
  source_count: number
  referring_sources: string[]
  ok_ratio: number | null
  html_ratio: number | null
  avg_depth: number
  first_seen: string
  last_seen: string
  quality_score: number
  notes: string | null
}

export interface DiscoveredDomainLink {
  id: string
  url: string
  path: string
  http_status: number | null
  content_type: string | null
  depth: number
  source_id: string
  source_name: string
  discovered_at: string
  status: string
}

export interface PathCluster {
  pattern: string
  count: number
}

export interface DiscoveredDomainFilters {
  search?: string
  status?: DomainStatus
}

export interface DomainLinksResponse {
  links: DiscoveredDomainLink[]
  path_clusters: PathCluster[]
  total: number
}

export const discoveredDomainsKeys = {
  all: ['discovered-domains'] as const,
  lists: () => [...discoveredDomainsKeys.all, 'list'] as const,
  list: (filters: Record<string, unknown>) =>
    [...discoveredDomainsKeys.lists(), filters] as const,
  details: () => [...discoveredDomainsKeys.all, 'detail'] as const,
  detail: (domain: string) => [...discoveredDomainsKeys.details(), domain] as const,
  links: (domain: string) => [...discoveredDomainsKeys.all, 'links', domain] as const,
}

export async function fetchDiscoveredDomainsPaginated(
  params: FetchParams<DiscoveredDomainFilters>,
): Promise<PaginatedResponse<DiscoveredDomain>> {
  const queryParams: Record<string, string | number | boolean> = {
    limit: params.limit,
    offset: params.offset,
    sort: params.sortBy,
    order: params.sortOrder,
  }

  if (params.filters?.search) queryParams.search = params.filters.search
  if (params.filters?.status) queryParams.status = params.filters.status

  const response = await crawlerApi.discoveredDomains.list(queryParams)
  const domains = response.data?.domains || []
  const total = response.data?.total ?? domains.length

  return {
    items: Array.isArray(domains) ? domains : [],
    total,
  }
}

export async function fetchDomainDetail(domain: string): Promise<DiscoveredDomain> {
  const response = await crawlerApi.discoveredDomains.get(domain)
  return response.data as DiscoveredDomain
}

export async function fetchDomainLinks(
  domain: string,
  params: { limit: number; offset: number },
): Promise<DomainLinksResponse> {
  const response = await crawlerApi.discoveredDomains.getLinks(domain, params)
  return response.data as DomainLinksResponse
}
