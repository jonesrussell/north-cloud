<script setup lang="ts">
import { formatDate } from '@/lib/utils'
import { Trash2, RotateCcw } from 'lucide-vue-next'
import { DataTablePagination, SortableColumnHeader } from '@/components/common'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import type { FrontierURL } from '@/features/intake/api/frontier'

defineProps<{
  urls: FrontierURL[]
  total: number
  isLoading: boolean
  page: number
  pageSize: number
  totalPages: number
  allowedPageSizes: readonly number[]
  sortBy: string
  sortOrder: 'asc' | 'desc'
  hasActiveFilters: boolean
  onSort: (key: string) => void
  onPageChange: (page: number) => void
  onPageSizeChange: (size: number) => void
  onClearFilters: () => void
  onRetry?: (id: string) => void
  onDelete?: (id: string) => void
}>()

const sortableColumns = [
  { key: 'priority', label: 'Priority' },
  { key: 'next_fetch_at', label: 'Next Fetch' },
  { key: 'created_at', label: 'Created' },
] as const

const nonSortableColumns = [
  { key: 'url', label: 'URL' },
  { key: 'host', label: 'Host' },
  { key: 'origin', label: 'Origin' },
  { key: 'status', label: 'Status' },
  { key: 'retry_count', label: 'Retries' },
  { key: 'last_error', label: 'Last error' },
] as const

const maxErrorDisplayLength = 80

function truncateError(err: string | null | undefined): string {
  if (err == null || err === '') return '—'
  return err.length <= maxErrorDisplayLength ? err : `${err.slice(0, maxErrorDisplayLength)}…`
}

function getStatusVariant(status: string) {
  switch (status) {
    case 'pending': return 'secondary'
    case 'fetching': return 'warning'
    case 'fetched': return 'success'
    case 'failed': return 'destructive'
    case 'dead': return 'outline'
    default: return 'secondary'
  }
}

function getOriginVariant(origin: string) {
  switch (origin) {
    case 'feed': return 'default'
    case 'sitemap': return 'secondary'
    case 'spider': return 'outline'
    case 'manual': return 'warning'
    default: return 'secondary'
  }
}
</script>

<template>
  <div class="space-y-4">
    <div class="rounded-md border">
      <table class="w-full">
        <thead>
          <tr class="border-b bg-muted/50">
            <SortableColumnHeader
              v-for="col in sortableColumns"
              :key="col.key"
              :label="col.label"
              :sort-key="col.key"
              :current-sort-by="sortBy"
              :current-sort-order="sortOrder"
              @sort="onSort(col.key)"
            />
            <th
              v-for="col in nonSortableColumns"
              :key="col.key"
              class="px-4 py-3 text-left text-sm font-medium text-muted-foreground"
            >
              {{ col.label }}
            </th>
            <th
              v-if="onDelete || onRetry"
              class="px-4 py-3 text-right text-sm font-medium text-muted-foreground"
            >
              Actions
            </th>
          </tr>
        </thead>
        <tbody>
          <template v-if="isLoading">
            <tr
              v-for="i in 5"
              :key="i"
              class="border-b"
            >
              <td
                v-for="j in sortableColumns.length + nonSortableColumns.length + (onDelete || onRetry ? 1 : 0)"
                :key="j"
                class="px-4 py-3"
              >
                <Skeleton class="h-4 w-24" />
              </td>
            </tr>
          </template>

          <tr
            v-else-if="urls.length === 0"
            class="border-b"
          >
            <td
              :colspan="sortableColumns.length + nonSortableColumns.length + (onDelete || onRetry ? 1 : 0)"
              class="px-4 py-12 text-center"
            >
              <p class="text-sm text-muted-foreground">
                {{ hasActiveFilters ? 'No URLs match your filters' : 'No URLs in frontier' }}
              </p>
              <Button
                v-if="hasActiveFilters"
                variant="outline"
                size="sm"
                class="mt-2"
                @click="onClearFilters"
              >
                Clear filters
              </Button>
            </td>
          </tr>

          <tr
            v-for="url in urls"
            v-else
            :key="url.id"
            class="border-b transition-colors hover:bg-muted/50"
          >
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ url.priority }}
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ formatDate(url.next_fetch_at) }}
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ formatDate(url.created_at) }}
            </td>
            <td class="px-4 py-3 text-sm">
              <a
                :href="url.url"
                target="_blank"
                class="text-primary hover:underline truncate block max-w-sm"
                :title="url.url"
              >
                {{ url.url }}
              </a>
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ url.host }}
            </td>
            <td class="px-4 py-3">
              <Badge :variant="getOriginVariant(url.origin)">
                {{ url.origin }}
              </Badge>
            </td>
            <td class="px-4 py-3">
              <Badge :variant="getStatusVariant(url.status)">
                {{ url.status }}
              </Badge>
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ url.retry_count }}
            </td>
            <td
              class="px-4 py-3 text-sm text-muted-foreground max-w-[240px]"
              :title="url.last_error ?? undefined"
            >
              <span class="truncate block" :title="url.last_error ?? undefined">
                {{ truncateError(url.last_error) }}
              </span>
            </td>
            <td
              v-if="onDelete || onRetry"
              class="px-4 py-3 text-right"
            >
              <Button
                v-if="onRetry && url.status === 'dead'"
                variant="ghost"
                size="icon"
                title="Reset for retry (re-queue)"
                @click="onRetry(url.id)"
              >
                <RotateCcw class="h-4 w-4 text-muted-foreground" />
              </Button>
              <Button
                v-if="onDelete"
                variant="ghost"
                size="icon"
                title="Delete"
                @click="onDelete(url.id)"
              >
                <Trash2 class="h-4 w-4 text-destructive" />
              </Button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <DataTablePagination
      :page="page"
      :page-size="pageSize"
      :total="total"
      :total-pages="totalPages"
      :allowed-page-sizes="allowedPageSizes"
      item-label="URLs"
      @update:page="onPageChange"
      @update:page-size="onPageSizeChange"
    />
  </div>
</template>
