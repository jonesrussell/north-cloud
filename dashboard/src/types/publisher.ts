// Publisher API Types - Routing V2

// ============================================================================
// Channel Rules (JSONB stored in channels table)
// ============================================================================

export interface ChannelRules {
  include_topics?: string[]
  exclude_topics?: string[]
  min_quality_score?: number
  content_types?: string[]
}

// ============================================================================
// Channel (Layer 2 - Custom channels with rules)
// ============================================================================

export interface Channel {
  id: string // UUID
  name: string
  slug: string
  redis_channel: string
  description?: string
  rules: ChannelRules
  rules_version: number
  enabled: boolean
  created_at: string
  updated_at?: string
}

export interface CreateChannelRequest {
  name: string
  slug: string
  redis_channel: string
  description?: string
  rules?: ChannelRules
  enabled?: boolean
}

export interface UpdateChannelRequest {
  name?: string
  slug?: string
  redis_channel?: string
  description?: string
  rules?: ChannelRules
  enabled?: boolean
}

export interface ChannelsListResponse {
  channels: Channel[]
  count: number
}

export interface ChannelPreviewResponse {
  channel: Channel
  rules_summary: {
    include_topics: string[] | null
    exclude_topics: string[] | null
    min_quality: number
    content_types: string[] | null
    rules_is_empty: boolean
    rules_version: number
  }
  matching_count: number
  sample_articles: PreviewArticle[]
  note: string
}

// ============================================================================
// Topics (Layer 1 - Automatic topic-based channels)
// ============================================================================

export interface TopicInfo {
  name: string
  layer1_channel: string
}

export interface TopicsResponse {
  topics: TopicInfo[]
  count: number
  note: string
}

// ============================================================================
// Indexes (Discovered Elasticsearch indexes)
// ============================================================================

export interface IndexInfo {
  name: string
  source: string
  health?: string
  status?: string
  docs_count?: string
}

export interface IndexesResponse {
  indexes: IndexInfo[]
  count: number
  note: string
}

// ============================================================================
// Publish History
// ============================================================================

export interface PublishHistoryItem {
  id: string
  channel_id?: string // Layer 2 channel ID if applicable
  article_id: string
  article_title: string
  article_url: string
  channel_name: string
  quality_score: number
  topics: string[] | null
  published_at: string
}

export interface PublishHistoryListResponse {
  history: PublishHistoryItem[]
  count: number
  total: number
  limit?: number
  offset?: number
}

// ============================================================================
// Stats
// ============================================================================

export interface StatsOverview {
  period: string
  total_articles: number
  channel_count: number
  by_channel: Record<string, number>
  generated_at: string
}

export interface StatsOverviewResponse extends StatsOverview {}

export type StatsPeriod = 'today' | 'week' | 'month' | 'all'

export interface ChannelStats {
  channel_id: string
  name: string
  slug: string
  redis_channel: string
  description?: string
  rules: ChannelRules
  article_count: number
}

export interface ChannelStatsResponse {
  channels: ChannelStats[]
  since: string
  count: number
}

export interface ActiveChannel {
  id?: string
  name: string
  slug?: string
  redis_channel: string
  description?: string
  rules?: ChannelRules
  enabled: boolean
  has_published: boolean
  total_published: number
  last_published_at?: string
  layer?: 'layer1' | 'layer2'
}

export interface ActiveChannelsResponse {
  channels: ActiveChannel[]
  count: number
  note: string
}

// ============================================================================
// Articles
// ============================================================================

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

export interface PreviewArticle {
  title: string
  quality_score: number
  topics: string[]
  published_date: string
  url?: string
  source?: string
}

// ============================================================================
// Health
// ============================================================================

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
