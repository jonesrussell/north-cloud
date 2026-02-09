import { ref, computed, watch } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { publisherApi } from '@/api/client'
import { usePolling } from './usePolling'
import type { PublishHistoryItem, ActiveChannel } from '@/types/publisher'

export interface GroupedArticle {
  article_id: string
  title: string
  url: string
  quality_score: number
  topics: string[]
  channels: string[]
  published_at: string // Most recent publish time
  publish_count: number
}

export interface PublishHistoryFilters {
  channel_name?: string
}

const POLLING_INTERVAL = 30000 // 30 seconds
const DEFAULT_LIMIT = 100 // Fetch more to account for grouping

/**
 * Composable for managing publish history data with filtering, polling, and article grouping.
 *
 * Groups articles by article_id to show unique articles with their publish channels.
 */
export function usePublishHistory() {
  // Raw data from API
  const rawHistory = ref<PublishHistoryItem[]>([])
  const channels = ref<ActiveChannel[]>([])

  // UI state
  const loading = ref(true)
  const error = ref<string | null>(null)
  const channelsLoading = ref(false)

  // Filters
  const filters = ref<PublishHistoryFilters>({})

  // Group articles by article_id
  const groupedArticles = computed<GroupedArticle[]>(() => {
    const articleMap = new Map<string, GroupedArticle>()

    for (const item of rawHistory.value) {
      const existing = articleMap.get(item.article_id)

      if (existing) {
        // Add channel if not already present
        if (!existing.channels.includes(item.channel_name)) {
          existing.channels.push(item.channel_name)
        }
        existing.publish_count++
        // Keep the most recent publish time
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

    // Sort by most recent publish time
    return Array.from(articleMap.values()).sort(
      (a, b) => new Date(b.published_at).getTime() - new Date(a.published_at).getTime()
    )
  })

  // Fetch publish history (non-fatal - sets error state but doesn't throw)
  async function fetchHistory() {
    try {
      error.value = null
      const params: Record<string, string | number> = { limit: DEFAULT_LIMIT }

      if (filters.value.channel_name) {
        params.channel_name = filters.value.channel_name
      }

      const response = await publisherApi.history.list(params)
      rawHistory.value = response.data?.history || []
    } catch (err) {
      console.error('Failed to fetch publish history:', err)
      error.value = 'Unable to load recent articles.'
      // Non-fatal - error state is set, UI will show error message
    }
  }

  // Fetch available channels for filter dropdown
  async function fetchChannels() {
    try {
      channelsLoading.value = true
      const response = await publisherApi.stats.activeChannels()
      channels.value = response.data?.channels || []
    } catch (err) {
      console.error('Failed to fetch channels:', err)
      // Non-fatal - filters will just be unavailable
    } finally {
      channelsLoading.value = false
    }
  }

  // Initial load
  async function loadData() {
    loading.value = true
    try {
      await Promise.all([fetchHistory(), fetchChannels()])
    } finally {
      loading.value = false
    }
  }

  // Set up polling
  const polling = usePolling(
    async () => {
      await fetchHistory()
    },
    POLLING_INTERVAL,
    { immediate: false }
  )

  // Filter methods
  function setChannelFilter(channelName: string | undefined) {
    filters.value.channel_name = channelName
  }

  function clearFilters() {
    filters.value = {}
  }

  // Computed for filter state
  const hasActiveFilters = computed(() => Boolean(filters.value.channel_name))

  const activeFilterCount = computed(() => {
    let count = 0
    if (filters.value.channel_name) count++
    return count
  })

  // Debounced fetch to prevent race conditions when filters change rapidly
  const debouncedFetchHistory = useDebounceFn(fetchHistory, 300)

  // Re-fetch when filters change (debounced to prevent race conditions)
  watch(filters, () => {
    debouncedFetchHistory()
  }, { deep: true })

  // Clear all history
  async function clearAllHistory(): Promise<{ deleted: number }> {
    const response = await publisherApi.history.clearAll()
    // Refresh after clearing
    await fetchHistory()
    return { deleted: response.data?.deleted || 0 }
  }

  // Start loading and polling on composable init
  loadData()
    .then(() => {
      polling.start()
    })
    .catch((err) => {
      console.error('Failed to initialize publish history:', err)
    })

  return {
    // Data
    articles: groupedArticles,
    rawHistory,
    channels,

    // State
    loading,
    error,
    channelsLoading,

    // Filters
    filters,
    hasActiveFilters,
    activeFilterCount,
    setChannelFilter,
    clearFilters,

    // Polling
    isPolling: polling.isPolling,
    isPaused: polling.isPaused,
    lastUpdate: polling.lastUpdate,

    // Actions
    refresh: fetchHistory,
    clearAllHistory,
  }
}
