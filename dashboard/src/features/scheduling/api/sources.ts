/**
 * Sources API Functions
 *
 * Query key factory and API functions for the Sources domain.
 */

import { sourcesApi } from '@/api/client'

// ============================================================================
// Types
// ============================================================================

export interface Source {
  id: string
  name: string
  url: string
  is_enabled: boolean
  created_at: string
  updated_at?: string
  crawl_config?: {
    selectors?: Record<string, unknown>
    interval_minutes?: number
  }
}

export interface SourcesListResponse {
  sources: Source[]
  total?: number
}

export interface CreateSourceRequest {
  name: string
  url: string
  is_enabled?: boolean
  crawl_config?: Source['crawl_config']
}

export interface UpdateSourceRequest {
  name?: string
  url?: string
  is_enabled?: boolean
  crawl_config?: Source['crawl_config']
}

// ============================================================================
// Query Key Factory
// ============================================================================

export const sourcesKeys = {
  all: ['sources'] as const,
  lists: () => [...sourcesKeys.all, 'list'] as const,
  list: (filters?: { enabled?: boolean }) => [...sourcesKeys.lists(), filters] as const,
  details: () => [...sourcesKeys.all, 'detail'] as const,
  detail: (id: string) => [...sourcesKeys.details(), id] as const,
}

// ============================================================================
// API Functions
// ============================================================================

export async function fetchSources(): Promise<SourcesListResponse> {
  const response = await sourcesApi.list()
  const data = response.data?.sources || response.data || []
  return {
    sources: data,
    total: data.length,
  }
}

export async function fetchSource(id: string): Promise<Source> {
  const response = await sourcesApi.get(id)
  return response.data
}

export async function createSource(data: CreateSourceRequest): Promise<Source> {
  const response = await sourcesApi.create(data)
  return response.data
}

export async function updateSource(id: string, data: UpdateSourceRequest): Promise<Source> {
  const response = await sourcesApi.update(id, data)
  return response.data
}

export async function deleteSource(id: string): Promise<void> {
  await sourcesApi.delete(id)
}
