export type ProblemKind = 'crawler' | 'publisher' | 'index' | 'system'
export type ProblemSeverity = 'error' | 'warning'

export interface Problem {
  id: string
  kind: ProblemKind
  severity: ProblemSeverity
  title: string
  action: string
  link?: string
  count?: number
  sourceIds?: string[]
}

export interface CrawlerMetrics {
  failedJobs: number
  staleJobs: number
  failedJobUrls: string[]
}

export interface IndexMetrics {
  clusterHealth: 'green' | 'yellow' | 'red'
  sources: SourceMetrics[]
}

export interface SourceMetrics {
  source: string
  rawCount: number
  classifiedCount: number
  backlog: number
  delta24h: number
  avgQuality: number
  active: boolean
}

export interface PublisherMetrics {
  publishedToday: number
  inactiveChannels: number
  inactiveChannelNames: string[]
}

export interface PipelineMetrics {
  crawler: CrawlerMetrics | null
  indexes: IndexMetrics | null
  publisher: PublisherMetrics | null
}
