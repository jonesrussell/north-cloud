import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { crawlerApi, publisherApi, classifierApi, indexManagerApi } from '@/api/client'
import type {
  PipelineStage,
  CrawlerMetrics,
  ClassifierMetrics,
  PublisherMetrics,
  IndexMetrics,
} from '@/types/metrics'

const DEFAULT_POLL_INTERVAL = 30000 // 30 seconds

export const useMetricsStore = defineStore('metrics', () => {
  // State
  const pipelineStages = ref<PipelineStage[]>([
    { name: 'Crawled', count: 0, status: 'healthy' },
    { name: 'Indexed', count: 0, status: 'healthy' },
    { name: 'Classified', count: 0, status: 'healthy' },
    { name: 'Routed', count: 0, status: 'healthy' },
    { name: 'Published', count: 0, status: 'healthy' },
  ])

  const crawler = ref<CrawlerMetrics | null>(null)
  const classifier = ref<ClassifierMetrics | null>(null)
  const publisher = ref<PublisherMetrics | null>(null)
  const index = ref<IndexMetrics | null>(null)

  const loading = ref(false)
  const error = ref<string | null>(null)
  const lastUpdate = ref<Date | null>(null)
  const isPolling = ref(false)

  // Private state
  let pollInterval: ReturnType<typeof setInterval> | null = null

  // Getters
  const totalCrawledToday = computed(() => crawler.value?.crawled_today ?? 0)
  const totalClassifiedToday = computed(() => classifier.value?.total_classified ?? 0)
  const totalPublishedToday = computed(() => publisher.value?.total_articles ?? 0)
  const avgQualityScore = computed(() => classifier.value?.avg_quality_score ?? 0)
  const activeRoutes = computed(() => publisher.value?.active_routes ?? 0)
  const totalRoutes = computed(() => publisher.value?.total_routes ?? 0)

  // Actions
  async function fetchCrawlerMetrics() {
    try {
      const response = await crawlerApi.getStats()
      if (response?.data) {
        crawler.value = {
          crawled_today: response.data.crawled_today || 0,
          indexed_today: response.data.indexed_today || 0,
          total_jobs: response.data.total_jobs || 0,
          active_jobs: response.data.active_jobs || 0,
          failed_jobs_24h: response.data.failed_jobs_24h || 0,
        }
        pipelineStages.value[0].count = crawler.value.crawled_today
        pipelineStages.value[1].count = crawler.value.indexed_today
      }
    } catch (err) {
      console.warn('Failed to fetch crawler metrics:', err)
    }
  }

  async function fetchClassifierMetrics() {
    try {
      const response = await classifierApi.stats.get({ date: 'today' })
      if (response?.data) {
        classifier.value = {
          total_classified: response.data.total_classified || 0,
          avg_quality_score: Math.round(response.data.avg_quality_score || 0),
          crime_related: response.data.crime_related || 0,
          by_topic: response.data.by_topic || {},
        }
        pipelineStages.value[2].count = classifier.value.total_classified
      }
    } catch (err) {
      console.warn('Failed to fetch classifier metrics:', err)
    }
  }

  async function fetchPublisherMetrics() {
    try {
      const [statsResponse, channelsResponse] = await Promise.all([
        publisherApi.stats.overview('today'),
        publisherApi.channels.list(false),
      ])

      const articles = statsResponse?.data?.total_articles || 0
      const channels = channelsResponse?.data?.channels || []

      publisher.value = {
        total_articles: articles,
        channel_count: statsResponse?.data?.channel_count || 0,
        by_channel: statsResponse?.data?.by_channel || {},
        active_routes: channels.filter((c: { enabled: boolean }) => c.enabled).length,
        total_routes: channels.length,
      }

      pipelineStages.value[3].count = articles // Routed
      pipelineStages.value[4].count = articles // Published
    } catch (err) {
      console.warn('Failed to fetch publisher metrics:', err)
    }
  }

  async function fetchIndexMetrics() {
    try {
      const response = await indexManagerApi.stats.get()
      if (response?.data) {
        index.value = {
          total_indexes: response.data.total_indexes || 0,
          total_documents: response.data.total_documents || 0,
          indexed_today: response.data.indexed_today || 0,
        }
        // Update indexed count if available from index manager
        if (index.value.indexed_today > 0) {
          pipelineStages.value[1].count = index.value.indexed_today
        }
      }
    } catch (err) {
      console.warn('Failed to fetch index metrics:', err)
    }
  }

  async function fetchAllMetrics() {
    loading.value = true
    error.value = null

    try {
      await Promise.all([
        fetchCrawlerMetrics(),
        fetchClassifierMetrics(),
        fetchPublisherMetrics(),
        fetchIndexMetrics(),
      ])
      lastUpdate.value = new Date()
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to fetch metrics'
    } finally {
      loading.value = false
    }
  }

  function startPolling(interval: number = DEFAULT_POLL_INTERVAL) {
    if (isPolling.value) return

    isPolling.value = true
    // Immediate first fetch
    fetchAllMetrics()
    // Then poll at interval
    pollInterval = setInterval(fetchAllMetrics, interval)
  }

  function stopPolling() {
    if (pollInterval) {
      clearInterval(pollInterval)
      pollInterval = null
    }
    isPolling.value = false
  }

  function $reset() {
    stopPolling()
    pipelineStages.value = pipelineStages.value.map((s) => ({ ...s, count: 0 }))
    crawler.value = null
    classifier.value = null
    publisher.value = null
    index.value = null
    lastUpdate.value = null
    error.value = null
    loading.value = false
  }

  return {
    // State
    pipelineStages,
    crawler,
    classifier,
    publisher,
    index,
    loading,
    error,
    lastUpdate,
    isPolling,

    // Getters
    totalCrawledToday,
    totalClassifiedToday,
    totalPublishedToday,
    avgQualityScore,
    activeRoutes,
    totalRoutes,

    // Actions
    fetchAllMetrics,
    fetchCrawlerMetrics,
    fetchClassifierMetrics,
    fetchPublisherMetrics,
    fetchIndexMetrics,
    startPolling,
    stopPolling,
    $reset,
  }
})
