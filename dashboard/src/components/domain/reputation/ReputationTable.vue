<script setup lang="ts">
import { formatDateShort } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { DataTablePagination, SortableColumnHeader } from '@/components/common'
import { Skeleton } from '@/components/ui/skeleton'
import type { SourceReputation } from '@/features/scheduling/api/reputation'

defineProps<{
  sources: SourceReputation[]
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
}>()

const sortableColumns = [
  { key: 'name', label: 'Source' },
  { key: 'category', label: 'Category' },
  { key: 'reputation', label: 'Reputation' },
  { key: 'total_classified', label: 'Total Classified' },
  { key: 'last_updated', label: 'Last Updated' },
] as const

function getScoreVariant(score: number) {
  if (score >= 80) return 'success'
  if (score >= 60) return 'warning'
  return 'destructive'
}

function formatLastUpdated(date: string | null | undefined): string {
  if (!date) return 'Never'
  return formatDateShort(date)
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
            <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
              Avg Quality
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
                v-for="j in 6"
                :key="j"
                class="px-4 py-3"
              >
                <Skeleton class="h-4 w-24" />
              </td>
            </tr>
          </template>

          <tr
            v-else-if="sources.length === 0"
            class="border-b"
          >
            <td
              colspan="6"
              class="px-4 py-12 text-center"
            >
              <p class="text-sm text-muted-foreground">
                {{ hasActiveFilters ? 'No sources match your filters' : 'No reputation data' }}
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
            v-for="source in sources"
            v-else
            :key="source.name"
            class="border-b transition-colors hover:bg-muted/50"
          >
            <td class="px-4 py-3 text-sm font-medium">
              {{ source.name }}
            </td>
            <td class="px-4 py-3">
              <Badge variant="outline">
                {{ source.category || 'unknown' }}
              </Badge>
            </td>
            <td class="px-4 py-3">
              <Badge :variant="getScoreVariant(source.reputation)">
                {{ source.reputation }}/100
              </Badge>
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ source.total_classified?.toLocaleString() || 0 }}
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ formatLastUpdated(source.last_updated) }}
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ source.avg_quality?.toFixed(1) || '0' }}
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
      item-label="sources"
      @update:page="onPageChange"
      @update:page-size="onPageSizeChange"
    />
  </div>
</template>
