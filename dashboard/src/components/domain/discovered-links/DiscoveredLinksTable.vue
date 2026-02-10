<script setup lang="ts">
import { formatDate } from '@/lib/utils'
import { Trash2 } from 'lucide-vue-next'
import { DataTablePagination, SortableColumnHeader } from '@/components/common'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import type { DiscoveredLink } from '@/features/intake/api/discoveredLinks'

defineProps<{
  links: DiscoveredLink[]
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
  onDelete?: (id: string) => void
}>()

const sortableColumns = [
  { key: 'priority', label: 'Priority' },
  { key: 'discovered_at', label: 'Discovered' },
] as const

const nonSortableColumns = [
  { key: 'url', label: 'URL' },
  { key: 'source_name', label: 'Source' },
  { key: 'depth', label: 'Depth' },
  { key: 'status', label: 'Status' },
] as const

function getStatusVariant(status: string) {
  switch (status) {
    case 'pending': return 'secondary'
    case 'processing': return 'warning'
    case 'completed': return 'success'
    case 'failed': return 'destructive'
    default: return 'outline'
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
              v-if="onDelete"
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
                v-for="j in sortableColumns.length + nonSortableColumns.length + (onDelete ? 1 : 0)"
                :key="j"
                class="px-4 py-3"
              >
                <Skeleton class="h-4 w-24" />
              </td>
            </tr>
          </template>

          <tr
            v-else-if="links.length === 0"
            class="border-b"
          >
            <td
              :colspan="sortableColumns.length + nonSortableColumns.length + (onDelete ? 1 : 0)"
              class="px-4 py-12 text-center"
            >
              <p class="text-sm text-muted-foreground">
                {{ hasActiveFilters ? 'No links match your filters' : 'No discovered links' }}
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
            v-for="link in links"
            v-else
            :key="link.id"
            class="border-b transition-colors hover:bg-muted/50"
          >
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ link.priority }}
            </td>
            <td class="px-4 py-3 text-sm">
              <a
                :href="link.url"
                target="_blank"
                class="text-primary hover:underline truncate block max-w-md"
              >
                {{ link.url }}
              </a>
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ link.source_name }}
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ link.depth }}
            </td>
            <td class="px-4 py-3">
              <Badge :variant="getStatusVariant(link.status)">
                {{ link.status }}
              </Badge>
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ formatDate(link.discovered_at) }}
            </td>
            <td
              v-if="onDelete"
              class="px-4 py-3 text-right"
            >
              <Button
                variant="ghost"
                size="icon"
                @click="onDelete(link.id)"
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
      item-label="links"
      @update:page="onPageChange"
      @update:page-size="onPageSizeChange"
    />
  </div>
</template>
