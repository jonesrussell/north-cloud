import { ref, onMounted } from 'vue'
import { indexManagerApi } from '@/api/client'
import type { QualityBuckets } from '@/types/aggregation'

const DEFAULT_OVERVIEW = {
  total_documents: 0,
  quality_distribution: { high: 0, medium: 0, low: 0 },
} as const

function normalizeQuality(dist?: QualityBuckets | null): { high: number; medium: number; low: number } {
  return {
    high: Math.max(0, Number(dist?.high) || 0),
    medium: Math.max(0, Number(dist?.medium) || 0),
    low: Math.max(0, Number(dist?.low) || 0),
  }
}

export interface IntelligenceOverviewData {
  total_documents: number
  quality_distribution: { high: number; medium: number; low: number }
}

export function useIntelligenceOverview() {
  const data = ref<IntelligenceOverviewData>({ ...DEFAULT_OVERVIEW })
  const loading = ref(true)
  const hasLoaded = ref(false)
  const error = ref<Error | null>(null)

  async function fetch() {
    loading.value = true
    error.value = null
    try {
      const res = await indexManagerApi.aggregations.getOverview()
      data.value = {
        total_documents: res.data?.total_documents ?? 0,
        quality_distribution: normalizeQuality(res.data?.quality_distribution),
      }
    } catch (e) {
      error.value = e instanceof Error ? e : new Error(String(e))
      data.value = { ...DEFAULT_OVERVIEW }
    } finally {
      loading.value = false
      hasLoaded.value = true
    }
  }

  onMounted(() => {
    fetch()
  })

  return { data, loading, hasLoaded, error, fetch }
}
