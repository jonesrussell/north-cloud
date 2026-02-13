import { ref, computed, onMounted } from 'vue'
import { crawlerApi, publisherApi, indexManagerApi, sourcesApi } from '@/api/client'
import { normalizeSourceNameForIndex } from '@/lib/normalizeSourceNameForIndex'
import { detectProblems } from '../problems/rules'
import type {
  PipelineMetrics,
  CrawlerMetrics,
  IndexMetrics,
  PublisherMetrics,
  SourceMetrics,
  Problem,
} from '../problems/types'
import type { SourceHealthResponse } from '@/types/aggregation'
import type { IndexStats } from '@/types/indexManager'

export function usePipelineHealth() {
  const metrics = ref<PipelineMetrics>({ crawler: null, indexes: null, publisher: null })
  const loading = ref(true)
  const problems = computed<Problem[]>(() => detectProblems(metrics.value))

  async function fetchCrawlerMetrics(): Promise<CrawlerMetrics | null> {
    try {
      const [statusRes, failedRes] = await Promise.all([
        crawlerApi.jobs.statusCounts(),
        crawlerApi.jobs.list({ status: 'failed', limit: 100 }),
      ])
      const counts = statusRes.data as Record<string, number>
      const failedJobs = (failedRes.data as { jobs?: Array<{ url: string; next_run_at?: string }> })?.jobs ?? []

      return {
        failedJobs: counts.failed ?? 0,
        staleJobs: 0,
        failedJobUrls: failedJobs.map((j) => j.url),
      }
    } catch {
      return null
    }
  }

  async function fetchIndexMetrics(): Promise<IndexMetrics | null> {
    try {
      let enabledKeys: Set<string> | null = null
      try {
        const sourcesRes = await sourcesApi.list()
        const list = sourcesRes.data?.sources ?? sourcesRes.data
        const sourcesList = Array.isArray(list) ? list : []
        const enabled = sourcesList.filter(
          (s: { enabled?: boolean; is_enabled?: boolean }) =>
            s.enabled === true || s.is_enabled === true,
        )
        enabledKeys = new Set(
          enabled.map((s: { name: string }) => normalizeSourceNameForIndex(s.name)),
        )
      } catch {
        // Fallback: source-manager unavailable → treat all sources as active
      }

      const [sourceHealthRes, statsRes] = await Promise.all([
        indexManagerApi.aggregations.getSourceHealth(),
        indexManagerApi.stats.get(),
      ])
      const statsData = statsRes.data as IndexStats
      const sourceHealthData = sourceHealthRes.data as SourceHealthResponse

      const sources: SourceMetrics[] = (sourceHealthData.sources ?? []).map((s) => ({
        source: s.source,
        rawCount: s.raw_count,
        classifiedCount: s.classified_count,
        backlog: s.backlog,
        delta24h: s.delta_24h,
        avgQuality: s.avg_quality,
        active: enabledKeys !== null ? enabledKeys.has(s.source) : true,
      }))

      return {
        clusterHealth: statsData.cluster_health ?? 'green',
        sources,
      }
    } catch {
      return null
    }
  }

  async function fetchPublisherMetrics(): Promise<PublisherMetrics | null> {
    try {
      const [statsRes, channelsRes] = await Promise.all([
        publisherApi.stats.overview('today'),
        publisherApi.channels.list(),
      ])
      const channels = channelsRes.data?.channels ?? []
      const inactive = channels.filter((c) => !c.enabled)

      return {
        publishedToday: statsRes.data?.total_articles ?? 0,
        inactiveChannels: inactive.length,
        inactiveChannelNames: inactive.map((c) => c.name),
      }
    } catch {
      return null
    }
  }

  async function fetchContentTypeMismatchCount(): Promise<number> {
    try {
      const res = await indexManagerApi.aggregations.getContentTypeMismatch({ hours: 24 })
      return res.data?.count ?? 0
    } catch {
      return 0
    }
  }

  async function fetch() {
    loading.value = true
    const [crawler, indexes, publisher, mismatchCount] = await Promise.all([
      fetchCrawlerMetrics(),
      fetchIndexMetrics(),
      fetchPublisherMetrics(),
      fetchContentTypeMismatchCount(),
    ])
    metrics.value = { crawler, indexes, publisher, contentTypeMismatchCount: mismatchCount }
    loading.value = false
  }

  onMounted(() => {
    fetch()
  })

  return { metrics, loading, problems, refresh: fetch }
}
