/**
 * Social Publisher API Functions
 *
 * Query key factory and fetch functions for the Social Publishing domain.
 */

import { socialPublisherApi } from '@/api/client'
import type { FetchParams, PaginatedResponse } from '@/types/table'
import type {
  SocialContent,
  SocialAccount,
  ContentFilters,
  AccountFilters,
} from '@/types/socialPublisher'

// ============================================================================
// Query Key Factory
// ============================================================================

export const socialPublisherKeys = {
  all: ['social-publisher'] as const,
  content: () => [...socialPublisherKeys.all, 'content'] as const,
  contentList: (filters?: ContentFilters) => [...socialPublisherKeys.content(), 'list', filters] as const,
  contentStatus: (id: string) => [...socialPublisherKeys.content(), 'status', id] as const,
  accounts: () => [...socialPublisherKeys.all, 'accounts'] as const,
  accountsList: () => [...socialPublisherKeys.accounts(), 'list'] as const,
  accountDetail: (id: string) => [...socialPublisherKeys.accounts(), 'detail', id] as const,
}

// ============================================================================
// Fetch Functions
// ============================================================================

/** Fetch content with pagination and filters — for ContentTable. */
export async function fetchContentPaginated(
  params: FetchParams<ContentFilters>
): Promise<PaginatedResponse<SocialContent>> {
  const queryParams: Record<string, string | number> = {
    limit: params.limit,
    offset: params.offset,
  }
  if (params.filters?.status) {
    queryParams.status = params.filters.status
  }
  if (params.filters?.type) {
    queryParams.type = params.filters.type
  }

  const response = await socialPublisherApi.content.list(queryParams)
  return {
    items: response.data?.items ?? [],
    total: response.data?.total ?? 0,
  }
}

/** Fetch accounts list — for AccountsTable. */
export async function fetchAccountsPaginated(
  _params: FetchParams<AccountFilters>
): Promise<PaginatedResponse<SocialAccount>> {
  const response = await socialPublisherApi.accounts.list()
  return {
    items: response.data?.items ?? [],
    total: response.data?.count ?? 0,
  }
}
