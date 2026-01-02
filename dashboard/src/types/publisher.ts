// Publisher API Types

export interface Source {
  id: number
  name: string
  index_pattern: string
  enabled: boolean
  created_at: string
  updated_at?: string
}

export interface Channel {
  id: number
  name: string
  description?: string
  enabled: boolean
  created_at: string
  updated_at?: string
}

export interface Route {
  id: number
  source_id: number
  channel_id: number
  source_name: string
  source_index_pattern: string
  channel_name: string
  min_quality_score: number
  topics: string[] | null
  enabled: boolean
  created_at: string
  updated_at?: string
}

export interface PublishHistoryItem {
  id: number
  article_id: string
  article_title: string
  article_url: string
  channel_name: string
  source_name?: string
  quality_score: number
  topics: string[] | null
  published_at: string
}

export interface StatsOverview {
  total_articles: number
  channel_count: number
  by_channel: Record<string, number>
}

export interface StatsChannels {
  [channelName: string]: number
}

export interface StatsRoutes {
  [routeId: string]: {
    source_name: string
    channel_name: string
    article_count: number
  }
}

// API Request Types
export interface CreateSourceRequest {
  name: string
  index_pattern: string
  enabled: boolean
}

export interface UpdateSourceRequest {
  name?: string
  index_pattern?: string
  enabled?: boolean
}

export interface CreateChannelRequest {
  name: string
  description?: string
  enabled: boolean
}

export interface UpdateChannelRequest {
  name?: string
  description?: string
  enabled?: boolean
}

export interface CreateRouteRequest {
  source_id: number
  channel_id: number
  min_quality_score: number
  topics: string[] | null
  enabled: boolean
}

export interface UpdateRouteRequest {
  source_id?: number
  channel_id?: number
  min_quality_score?: number
  topics?: string[] | null
  enabled?: boolean
}

// API Response Types
export interface SourcesListResponse {
  sources: Source[]
  count?: number
}

export interface ChannelsListResponse {
  channels: Channel[]
  count?: number
}

export interface RoutesListResponse {
  routes: Route[]
  count?: number
}

export interface PublishHistoryListResponse {
  history: PublishHistoryItem[]
  count?: number
  total?: number
  limit?: number
  offset?: number
}

export interface StatsOverviewResponse {
  total_articles: number
  channel_count: number
  by_channel: Record<string, number>
}

export type StatsPeriod = 'today' | 'week' | 'month' | 'all'

export interface HealthStatus {
  status: 'healthy' | 'degraded' | 'unhealthy'
  service: string
  version: string
  redis?: {
    connected: boolean
    error?: string
  }
  database?: {
    connected: boolean
  }
}

export interface ActiveChannel {
  name: string
  description?: string
  enabled: boolean
  has_published: boolean
  total_published: number
  last_published_at?: string
}

export interface ActiveChannelsResponse {
  channels: ActiveChannel[]
  count: number
  note: string
}

// Article types for recent articles view
export interface RecentArticle {
  id: string | number
  title: string
  url: string
  city: string
  posted_at: string
  // Additional fields from backend
  article_id?: string
  article_title?: string
  article_url?: string
  channel_name?: string
  published_at?: string
  quality_score?: number
  topics?: string[] | null
}

export interface RecentArticlesResponse {
  articles: RecentArticle[]
  count: number
}

// Article types for preview and testing
export interface PreviewArticle {
  title: string
  quality_score: number
  topics: string[]
  published_date: string
  url?: string
  source?: string
  route_id?: string
}

export interface TestCrawlArticle {
  title?: string
  body?: string
  url?: string
  published_date?: string
  author?: string
  quality_score?: number
}

