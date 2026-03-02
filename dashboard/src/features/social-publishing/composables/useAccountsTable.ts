/**
 * useAccountsTable - Server-paginated accounts for the Social Accounts table view.
 */

import { computed } from 'vue'
import { useServerPaginatedTable } from '@/composables/useServerPaginatedTable'
import { fetchAccountsPaginated } from '../api/socialPublisher'
import type { SocialAccount, AccountFilters } from '@/types/socialPublisher'

const ACCOUNTS_SORT_FIELDS = ['name', 'platform', 'project', 'created_at']

export function useAccountsTable() {
  const table = useServerPaginatedTable<SocialAccount, AccountFilters>({
    fetchFn: fetchAccountsPaginated,
    queryKeyPrefix: 'social-accounts',
    defaultLimit: 25,
    defaultSortBy: 'name',
    defaultSortOrder: 'asc',
    allowedSortFields: ACCOUNTS_SORT_FIELDS,
    allowedPageSizes: [10, 25, 50, 100],
  })

  const hasActiveFilters = computed(() => {
    const f = table.filters.value
    return !!f.platform
  })

  const activeFilterCount = computed(() => {
    const f = table.filters.value
    let count = 0
    if (f.platform) count++
    return count
  })

  return {
    items: table.items,
    total: table.total,
    isLoading: table.isLoading,
    isFetching: table.isRefetching,
    error: table.error,
    hasError: table.hasError,

    page: table.page,
    pageSize: table.pageSize,
    totalPages: table.totalPages,
    allowedPageSizes: table.allowedPageSizes,
    setPage: table.setPage,
    setPageSize: table.setPageSize,

    sortBy: table.sortBy,
    sortOrder: table.sortOrder,
    toggleSort: table.toggleSort,

    filters: table.filters,
    setFilter: (key: keyof AccountFilters, value: AccountFilters[keyof AccountFilters]) => {
      table.setFilters({ [key]: value } as Partial<AccountFilters>)
    },
    clearFilters: table.clearFilters,
    hasActiveFilters,
    activeFilterCount,

    refetch: table.refetch,
  }
}
