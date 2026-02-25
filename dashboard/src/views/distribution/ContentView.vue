<script setup lang="ts">
import { ref, computed } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { Loader2, FileText, ExternalLink, Trash2 } from 'lucide-vue-next'
import { publisherApi } from '@/api/client'
import { usePublishHistoryTable } from '@/composables'
import type { GroupedItem } from '@/composables'
import { formatDate } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { BadgeList } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { ContentFilterBar } from '@/components/domain/content'
import { ConfirmModal, DataTablePagination } from '@/components/common'

const POLLING_INTERVAL = 30_000 // 30 seconds

const table = usePublishHistoryTable({ refetchInterval: POLLING_INTERVAL })

const { data: channelsData } = useQuery({
  queryKey: ['publisher', 'active-channels'],
  queryFn: async () => {
    const res = await publisherApi.stats.activeChannels()
    return res.data
  },
})

const channels = computed(() => channelsData.value?.channels ?? [])

// Group raw history items by content_id for display
const groupedItems = computed<GroupedItem[]>(() => {
  const itemMap = new Map<string, GroupedItem>()
  for (const item of table.items.value) {
    const existing = itemMap.get(item.content_id)
    if (existing) {
      if (!existing.channels.includes(item.channel_name)) {
        existing.channels.push(item.channel_name)
      }
      existing.publish_count++
      if (new Date(item.published_at) > new Date(existing.published_at)) {
        existing.published_at = item.published_at
      }
    } else {
      itemMap.set(item.content_id, {
        content_id: item.content_id,
        title: item.content_title,
        url: item.content_url,
        quality_score: item.quality_score,
        topics: item.topics || [],
        channels: [item.channel_name],
        published_at: item.published_at,
        publish_count: 1,
      })
    }
  }
  return Array.from(itemMap.values()).sort(
    (a, b) => new Date(b.published_at).getTime() - new Date(a.published_at).getTime()
  )
})

// Clear all state
const clearModalOpen = ref(false)
const clearing = ref(false)
const clearError = ref<string | null>(null)

function confirmClear() {
  clearError.value = null
  clearModalOpen.value = true
}

function cancelClear() {
  clearModalOpen.value = false
  clearError.value = null
}

async function handleClearConfirm() {
  try {
    clearing.value = true
    clearError.value = null
    await table.clearAllHistory()
    clearModalOpen.value = false
  } catch (err) {
    console.error('Failed to clear publish history:', err)
    clearError.value = 'Failed to clear publish history. Please try again.'
  } finally {
    clearing.value = false
  }
}

function onChannelChange(channelName: string | undefined) {
  table.setFilter('channel_name', channelName || undefined)
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          Recent Content
        </h1>
        <p class="text-muted-foreground">
          Recently published content across all channels
        </p>
      </div>
      <Button
        variant="outline"
        :disabled="groupedItems.length === 0"
        @click="confirmClear"
      >
        <Trash2 class="mr-2 h-4 w-4" />
        Clear All
      </Button>
    </div>

    <!-- Filter Bar -->
    <ContentFilterBar
      :channels="channels"
      :filters="{ channel_name: table.filters.value.channel_name }"
      :has-active-filters="table.hasActiveFilters.value"
      :active-filter-count="table.activeFilterCount.value"
      :is-polling="true"
      :is-paused="false"
      :loading="table.isLoading.value"
      @update:channel="onChannelChange"
      @clear-filters="table.clearFilters"
      @refresh="table.refetch"
    />

    <!-- Loading State -->
    <div
      v-if="table.isLoading.value && groupedItems.length === 0"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <!-- Error State -->
    <Card
      v-else-if="table.error.value"
      class="border-destructive"
    >
      <CardContent class="pt-6">
        <p class="text-destructive">
          {{ table.error.value?.message || 'Unable to load recent content.' }}
        </p>
      </CardContent>
    </Card>

    <!-- Empty State -->
    <Card v-else-if="groupedItems.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <FileText class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No recent content
        </h3>
        <p class="text-muted-foreground">
          Content will appear here once published to channels.
        </p>
      </CardContent>
    </Card>

    <!-- Content Table -->
    <Card v-else>
      <CardHeader>
        <CardTitle>Published Content</CardTitle>
        <CardDescription>
          Showing {{ groupedItems.length }} unique items on this page
          <span
            v-if="table.hasActiveFilters.value"
            class="text-primary"
          >
            (filtered)
          </span>
        </CardDescription>
      </CardHeader>
      <CardContent class="p-0">
        <div class="space-y-4">
          <table class="w-full">
            <thead class="border-b bg-muted/50">
              <tr>
                <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                  Title
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                  Channels
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                  Quality
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                  Published
                </th>
                <th class="px-6 py-3 text-right text-xs font-medium text-muted-foreground uppercase">
                  Link
                </th>
              </tr>
            </thead>
            <tbody class="divide-y">
              <tr
                v-for="item in groupedItems"
                :key="item.content_id"
                class="hover:bg-muted/50"
              >
                <td class="px-6 py-4">
                  <p class="text-sm font-medium truncate max-w-sm">
                    {{ item.title }}
                  </p>
                  <BadgeList
                    v-if="item.topics.length > 0"
                    :items="item.topics"
                    :max-visible="3"
                    variant="outline"
                    class="mt-1"
                  />
                </td>
                <td class="px-6 py-4">
                  <BadgeList
                    :items="item.channels"
                    :max-visible="2"
                    variant="secondary"
                  />
                </td>
                <td class="px-6 py-4 text-sm text-muted-foreground">
                  {{ item.quality_score }}/100
                </td>
                <td class="px-6 py-4 text-sm text-muted-foreground">
                  {{ formatDate(item.published_at) }}
                </td>
                <td class="px-6 py-4 text-right">
                  <a
                    :href="item.url"
                    target="_blank"
                    class="text-primary hover:text-primary/80"
                  >
                    <ExternalLink class="h-4 w-4" />
                  </a>
                </td>
              </tr>
            </tbody>
          </table>

          <DataTablePagination
            :page="table.page.value"
            :page-size="table.pageSize.value"
            :total="table.total.value"
            :total-pages="table.totalPages.value"
            :allowed-page-sizes="table.allowedPageSizes"
            item-label="records"
            @update:page="table.setPage"
            @update:page-size="table.setPageSize"
          />
        </div>
      </CardContent>
    </Card>

    <!-- Clear Confirmation Modal -->
    <ConfirmModal
      :show="clearModalOpen"
      title="Clear Publish History"
      message="Are you sure you want to clear all publish history? This will delete all records of published content. The router may re-publish content that was previously sent."
      type="danger"
      confirm-text="Clear All History"
      :loading="clearing"
      @confirm="handleClearConfirm"
      @cancel="cancelClear"
    />
  </div>
</template>
