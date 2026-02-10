<script setup lang="ts">
import { formatDate } from '@/lib/utils'
import { CheckCircle2, XCircle, Clock } from 'lucide-vue-next'
import { DataTablePagination } from '@/components/common'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import type { PublishHistoryItem } from '@/types/publisher'

defineProps<{
  items: PublishHistoryItem[]
  total: number
  isLoading: boolean
  page: number
  pageSize: number
  totalPages: number
  allowedPageSizes: readonly number[]
  hasActiveFilters: boolean
  onPageChange: (page: number) => void
  onPageSizeChange: (size: number) => void
  onClearFilters: () => void
}>()

function getStatusIcon(status: string) {
  switch (status) {
    case 'delivered': return CheckCircle2
    case 'failed': return XCircle
    default: return Clock
  }
}
</script>

<template>
  <div class="space-y-4">
    <div class="rounded-md border">
      <table class="w-full">
        <thead>
          <tr class="border-b bg-muted/50">
            <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
              Status
            </th>
            <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
              Article
            </th>
            <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
              Channel
            </th>
            <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
              Quality
            </th>
            <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
              Time
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
                v-for="j in 5"
                :key="j"
                class="px-4 py-3"
              >
                <Skeleton class="h-4 w-24" />
              </td>
            </tr>
          </template>

          <tr
            v-else-if="items.length === 0"
            class="border-b"
          >
            <td
              colspan="5"
              class="px-4 py-12 text-center"
            >
              <p class="text-sm text-muted-foreground">
                {{ hasActiveFilters ? 'No logs match your filters' : 'No delivery logs' }}
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
            v-for="item in items"
            v-else
            :key="item.id"
            class="border-b transition-colors hover:bg-muted/50"
          >
            <td class="px-4 py-3">
              <div class="flex items-center gap-2">
                <component
                  :is="getStatusIcon('delivered')"
                  :class="['h-4 w-4', 'text-green-500']"
                />
                <Badge variant="success">
                  delivered
                </Badge>
              </div>
            </td>
            <td class="px-4 py-3 text-sm">
              <p class="truncate max-w-xs font-medium">
                {{ item.article_title }}
              </p>
              <p class="text-xs text-muted-foreground font-mono">
                {{ item.article_id }}
              </p>
            </td>
            <td class="px-4 py-3">
              <Badge variant="outline">
                {{ item.channel_name }}
              </Badge>
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ item.quality_score }}/100
            </td>
            <td class="px-4 py-3 text-sm text-muted-foreground">
              {{ formatDate(item.published_at) }}
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
      item-label="logs"
      @update:page="onPageChange"
      @update:page-size="onPageSizeChange"
    />
  </div>
</template>
