<script setup lang="ts">
import { ref } from 'vue'
import { Loader2, FileText, ExternalLink, Trash2 } from 'lucide-vue-next'
import { usePublishHistory } from '@/composables'
import { formatDate } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { BadgeList } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { ArticlesFilterBar } from '@/components/domain/articles'
import { ConfirmModal } from '@/components/common'

const history = usePublishHistory()

// Clear all state
const clearModalOpen = ref(false)
const clearing = ref(false)
const clearError = ref<string | null>(null)

const confirmClear = () => {
  clearError.value = null
  clearModalOpen.value = true
}

const cancelClear = () => {
  clearModalOpen.value = false
  clearError.value = null
}

const handleClearConfirm = async () => {
  try {
    clearing.value = true
    clearError.value = null
    await history.clearAllHistory()
    clearModalOpen.value = false
  } catch (err) {
    console.error('Failed to clear publish history:', err)
    clearError.value = 'Failed to clear publish history. Please try again.'
  } finally {
    clearing.value = false
  }
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
        :disabled="history.articles.value.length === 0"
        @click="confirmClear"
      >
        <Trash2 class="mr-2 h-4 w-4" />
        Clear All
      </Button>
    </div>

    <!-- Filter Bar -->
    <ArticlesFilterBar
      :channels="history.channels.value"
      :filters="history.filters.value"
      :has-active-filters="history.hasActiveFilters.value"
      :active-filter-count="history.activeFilterCount.value"
      :is-polling="history.isPolling.value"
      :is-paused="history.isPaused.value"
      :loading="history.loading.value"
      @update:channel="history.setChannelFilter"
      @clear-filters="history.clearFilters"
      @refresh="history.refresh"
    />

    <!-- Loading State -->
    <div
      v-if="history.loading.value"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <!-- Error State -->
    <Card
      v-else-if="history.error.value"
      class="border-destructive"
    >
      <CardContent class="pt-6">
        <p class="text-destructive">
          {{ history.error.value }}
        </p>
      </CardContent>
    </Card>

    <!-- Empty State -->
    <Card v-else-if="history.articles.value.length === 0">
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
          Showing {{ history.articles.value.length }} unique articles
          <span
            v-if="history.hasActiveFilters.value"
            class="text-primary"
          >
            (filtered)
          </span>
        </CardDescription>
      </CardHeader>
      <CardContent class="p-0">
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
              v-for="article in history.articles.value"
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
