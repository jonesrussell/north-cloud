<script setup lang="ts">
import { ref, computed } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { Loader2, FileText, ExternalLink, Trash2 } from 'lucide-vue-next'
import { publisherApi } from '@/api/client'
import { usePublishHistoryTable } from '@/composables'
import type { GroupedArticle } from '@/composables'
import { formatDate } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { BadgeList } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { ArticlesFilterBar } from '@/components/domain/articles'
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

// Group raw history items by article_id for display
const groupedArticles = computed<GroupedArticle[]>(() => {
  const articleMap = new Map<string, GroupedArticle>()
  for (const item of table.items.value) {
    const existing = articleMap.get(item.article_id)
    if (existing) {
      if (!existing.channels.includes(item.channel_name)) {
        existing.channels.push(item.channel_name)
      }
      existing.publish_count++
      if (new Date(item.published_at) > new Date(existing.published_at)) {
        existing.published_at = item.published_at
      }
    } else {
      articleMap.set(item.article_id, {
        article_id: item.article_id,
        title: item.article_title,
        url: item.article_url,
        quality_score: item.quality_score,
        topics: item.topics || [],
        channels: [item.channel_name],
        published_at: item.published_at,
        publish_count: 1,
      })
    }
  }
  return Array.from(articleMap.values()).sort(
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
          Recent Articles
        </h1>
        <p class="text-muted-foreground">
          Recently published articles across all channels
        </p>
      </div>
      <Button
        variant="outline"
        :disabled="groupedArticles.length === 0"
        @click="confirmClear"
      >
        <Trash2 class="mr-2 h-4 w-4" />
        Clear All
      </Button>
    </div>

    <!-- Filter Bar -->
    <ArticlesFilterBar
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
      v-if="table.isLoading.value && groupedArticles.length === 0"
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
          {{ table.error.value?.message || 'Unable to load recent articles.' }}
        </p>
      </CardContent>
    </Card>

    <!-- Empty State -->
    <Card v-else-if="groupedArticles.length === 0">
      <CardContent class="flex flex-col items-center justify-center py-12">
        <FileText class="h-12 w-12 text-muted-foreground mb-4" />
        <h3 class="text-lg font-medium mb-2">
          No recent articles
        </h3>
        <p class="text-muted-foreground">
          Articles will appear here once published to channels.
        </p>
      </CardContent>
    </Card>

    <!-- Articles Table -->
    <Card v-else>
      <CardHeader>
        <CardTitle>Published Articles</CardTitle>
        <CardDescription>
          Showing {{ groupedArticles.length }} unique articles on this page
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
                v-for="article in groupedArticles"
                :key="article.article_id"
                class="hover:bg-muted/50"
              >
                <td class="px-6 py-4">
                  <p class="text-sm font-medium truncate max-w-sm">
                    {{ article.title }}
                  </p>
                  <BadgeList
                    v-if="article.topics.length > 0"
                    :items="article.topics"
                    :max-visible="3"
                    variant="outline"
                    class="mt-1"
                  />
                </td>
                <td class="px-6 py-4">
                  <BadgeList
                    :items="article.channels"
                    :max-visible="2"
                    variant="secondary"
                  />
                </td>
                <td class="px-6 py-4 text-sm text-muted-foreground">
                  {{ article.quality_score }}/100
                </td>
                <td class="px-6 py-4 text-sm text-muted-foreground">
                  {{ formatDate(article.published_at) }}
                </td>
                <td class="px-6 py-4 text-right">
                  <a
                    :href="article.url"
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
      message="Are you sure you want to clear all publish history? This will delete all records of published articles. The router may re-publish articles that were previously sent."
      type="danger"
      confirm-text="Clear All History"
      :loading="clearing"
      @confirm="handleClearConfirm"
      @cancel="cancelClear"
    />
  </div>
</template>
