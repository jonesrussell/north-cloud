<script setup lang="ts">
import { ref } from 'vue'
import { formatDateShort } from '@/lib/utils'
import { Loader2, RotateCcw, ChevronDown, ChevronRight } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { DataTablePagination, SortableColumnHeader } from '@/components/common'
import DeliverySummaryBadges from '@/components/domain/social-publishing/DeliverySummaryBadges.vue'
import { socialPublisherApi } from '@/api/client'
import type { SocialContent, Delivery } from '@/types/socialPublisher'

interface Props {
  items: SocialContent[]
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
  onRetry: () => void
}

const props = defineProps<Props>()

const expandedId = ref<string | null>(null)
const deliveries = ref<Delivery[]>([])
const loadingDeliveries = ref(false)
const retryingId = ref<string | null>(null)

const sortableColumns = [
  { key: 'type', label: 'Type' },
  { key: 'title', label: 'Title' },
  { key: 'source', label: 'Source' },
  { key: 'created_at', label: 'Created' },
] as const

async function toggleExpand(id: string) {
  if (expandedId.value === id) {
    expandedId.value = null
    deliveries.value = []
    return
  }
  expandedId.value = id
  loadingDeliveries.value = true
  try {
    const response = await socialPublisherApi.content.status(id)
    deliveries.value = response.data?.deliveries ?? []
  } catch {
    deliveries.value = []
  } finally {
    loadingDeliveries.value = false
  }
}

async function retryDelivery(deliveryId: string) {
  retryingId.value = deliveryId
  try {
    await socialPublisherApi.content.retry(deliveryId)
    if (expandedId.value) {
      const response = await socialPublisherApi.content.status(expandedId.value)
      deliveries.value = response.data?.deliveries ?? []
    }
    props.onRetry()
  } catch {
    // Error is visible in the delivery status
  } finally {
    retryingId.value = null
  }
}
</script>

<template>
  <div class="space-y-4">
    <div class="rounded-md border">
      <table class="w-full">
        <thead>
          <tr class="border-b bg-muted/50">
            <th class="w-8 px-2 py-3" />
            <SortableColumnHeader
              v-for="col in sortableColumns"
              :key="col.key"
              :label="col.label"
              :sort-key="col.key"
              :current-sort-by="sortBy"
              :current-sort-order="sortOrder"
              @sort="onSort(col.key)"
            />
            <th class="px-4 py-3 text-sm font-medium text-muted-foreground">
              Deliveries
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
              <td class="px-2 py-3" />
              <td class="px-4 py-3">
                <Skeleton class="h-5 w-16" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-4 w-48" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-4 w-24" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-4 w-20" />
              </td>
              <td class="px-4 py-3">
                <Skeleton class="h-5 w-32" />
              </td>
            </tr>
          </template>

          <tr
            v-else-if="items.length === 0"
            class="border-b"
          >
            <td
              colspan="6"
              class="px-4 py-12 text-center"
            >
              <p class="text-sm text-muted-foreground">
                {{ hasActiveFilters ? 'No content matches your filters' : 'No content found' }}
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

          <template
            v-for="item in items"
            v-else
            :key="item.id"
          >
            <tr
              class="border-b transition-colors hover:bg-muted/50 cursor-pointer"
              @click="toggleExpand(item.id)"
            >
              <td class="px-2 py-3 text-center">
                <ChevronDown
                  v-if="expandedId === item.id"
                  class="h-4 w-4 text-muted-foreground"
                />
                <ChevronRight
                  v-else
                  class="h-4 w-4 text-muted-foreground"
                />
              </td>
              <td class="px-4 py-3">
                <Badge variant="secondary">
                  {{ item.type }}
                </Badge>
              </td>
              <td class="px-4 py-3 text-sm font-medium">
                {{ item.title || item.summary || '(untitled)' }}
              </td>
              <td class="px-4 py-3 text-sm text-muted-foreground">
                {{ item.source || '\u2014' }}
              </td>
              <td class="px-4 py-3 text-sm text-muted-foreground">
                {{ formatDateShort(item.created_at) }}
              </td>
              <td class="px-4 py-3">
                <DeliverySummaryBadges
                  v-if="item.delivery_summary"
                  :summary="item.delivery_summary"
                />
                <span
                  v-else
                  class="text-xs text-muted-foreground"
                >
                  \u2014
                </span>
              </td>
            </tr>

            <!-- Expanded delivery detail row -->
            <tr
              v-if="expandedId === item.id"
              class="border-b bg-muted/20"
            >
              <td
                colspan="6"
                class="px-6 py-4"
              >
                <div
                  v-if="loadingDeliveries"
                  class="flex items-center gap-2 text-sm text-muted-foreground"
                >
                  <Loader2 class="h-4 w-4 animate-spin" />
                  Loading deliveries...
                </div>
                <div
                  v-else-if="deliveries.length === 0"
                  class="text-sm text-muted-foreground"
                >
                  No deliveries for this content.
                </div>
                <table
                  v-else
                  class="w-full text-sm"
                >
                  <thead>
                    <tr class="text-left text-muted-foreground">
                      <th class="pb-2 font-medium">
                        Platform
                      </th>
                      <th class="pb-2 font-medium">
                        Account
                      </th>
                      <th class="pb-2 font-medium">
                        Status
                      </th>
                      <th class="pb-2 font-medium">
                        Attempts
                      </th>
                      <th class="pb-2 font-medium">
                        Error
                      </th>
                      <th class="pb-2 text-right font-medium">
                        Actions
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr
                      v-for="delivery in deliveries"
                      :key="delivery.id"
                      class="border-t border-muted"
                    >
                      <td class="py-2">
                        <Badge variant="outline">
                          {{ delivery.platform }}
                        </Badge>
                      </td>
                      <td class="py-2">
                        {{ delivery.account }}
                      </td>
                      <td class="py-2">
                        <Badge
                          :variant="delivery.status === 'delivered' ? 'success' : delivery.status === 'failed' ? 'destructive' : 'secondary'"
                        >
                          {{ delivery.status }}
                        </Badge>
                      </td>
                      <td class="py-2">
                        {{ delivery.attempts }}/{{ delivery.max_attempts }}
                      </td>
                      <td class="py-2 text-destructive">
                        {{ delivery.error || '\u2014' }}
                      </td>
                      <td class="py-2 text-right">
                        <Button
                          v-if="delivery.status === 'failed'"
                          variant="ghost"
                          size="sm"
                          :disabled="retryingId === delivery.id"
                          @click.stop="retryDelivery(delivery.id)"
                        >
                          <Loader2
                            v-if="retryingId === delivery.id"
                            class="mr-1 h-3 w-3 animate-spin"
                          />
                          <RotateCcw
                            v-else
                            class="mr-1 h-3 w-3"
                          />
                          Retry
                        </Button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <DataTablePagination
      :page="page"
      :page-size="pageSize"
      :total="total"
      :total-pages="totalPages"
      :allowed-page-sizes="allowedPageSizes"
      item-label="content items"
      @update:page="onPageChange"
      @update:page-size="onPageSizeChange"
    />
  </div>
</template>
