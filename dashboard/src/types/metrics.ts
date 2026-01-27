// Pipeline metrics types

export interface PipelineStage {
  name: string
  count: number
  change?: number
  status: 'healthy' | 'degraded' | 'unhealthy'
}

export interface PipelineMetrics {
  stages: PipelineStage[]
  lastUpdate: string | null
}

export interface CrawlerMetrics {
  crawled_today: number
  indexed_today: number
  total_jobs: number
  active_jobs: number
  failed_jobs_24h: number
}

export interface ClassifierMetrics {
  total_classified: number
  avg_quality_score: number
  crime_related: number
  by_topic: Record<string, number>
}

export interface PublisherMetrics {
  total_articles: number
  channel_count: number
  by_channel: Record<string, number>
  active_routes: number
  total_routes: number
}

export interface IndexMetrics {
  total_indexes: number
  total_documents: number
  indexed_today: number
}

export interface AggregatedMetrics {
  pipeline: PipelineMetrics
  crawler: CrawlerMetrics | null
  classifier: ClassifierMetrics | null
  publisher: PublisherMetrics | null
  index: IndexMetrics | null
  lastUpdate: string | null
}

// Quick actions for the dashboard
export interface QuickAction {
  label: string
  path: string
  icon: 'plus' | 'chart' | 'refresh' | 'settings'
}

export const DEFAULT_QUICK_ACTIONS: QuickAction[] = [
  { label: 'New Crawl Job', path: '/intake/jobs?create=true', icon: 'plus' },
  { label: 'Add Source', path: '/scheduling/sources/new', icon: 'plus' },
  { label: 'New Route', path: '/distribution/routes/new', icon: 'plus' },
  { label: 'View Analytics', path: '/intelligence/stats', icon: 'chart' },
]
