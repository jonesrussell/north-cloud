/** Matches source-manager/internal/models/source.go Source struct */
export interface Source {
  id: string
  name: string
  url: string
  rate_limit: string
  max_depth: number
  time: string[]
  selectors: SelectorConfig
  enabled: boolean
  feed_url?: string | null
  sitemap_url?: string | null
  ingestion_mode: string
  feed_poll_interval_minutes: number
  feed_disabled_at?: string | null
  feed_disable_reason?: string | null
  allow_source_discovery: boolean
  identity_key?: string | null
  extraction_profile?: Record<string, unknown> | null
  template_hint?: string | null
  render_mode: string
  type: string
  indigenous_region?: string | null
  disabled_at?: string | null
  disable_reason?: string | null
  data_format?: string | null
  update_frequency?: string | null
  license_type?: string | null
  attribution_text?: string | null
  created_at: string
  updated_at: string
}

export interface SelectorConfig {
  article: ArticleSelectors
  list: ListSelectors
  page: PageSelectors
}

export interface ArticleSelectors {
  container?: string
  title?: string
  body?: string
  intro?: string
  link?: string
  image?: string
  byline?: string
  published_time?: string
  time_ago?: string
  section?: string
  category?: string
  article_id?: string
  json_ld?: string
  keywords?: string
  description?: string
  og_title?: string
  og_description?: string
  og_image?: string
  og_url?: string
  og_type?: string
  og_site_name?: string
  canonical?: string
  author?: string
  exclude?: string[]
}

export interface ListSelectors {
  container?: string
  article_cards?: string
  article_list?: string
  exclude_from_list?: string[]
}

export interface PageSelectors {
  container?: string
  title?: string
  content?: string
  description?: string
  keywords?: string
  og_title?: string
  og_description?: string
  og_image?: string
  og_url?: string
  canonical?: string
  exclude?: string[]
}

export interface SourceFormData {
  name: string
  url: string
  rate_limit: string
  max_depth: number
  type: string
  enabled: boolean
  feed_url?: string
  sitemap_url?: string
  ingestion_mode: string
  feed_poll_interval_minutes: number
  render_mode: string
  allow_source_discovery: boolean
  selectors: SelectorConfig
}

export interface TestCrawlRequest {
  url: string
  selectors: SelectorConfig
}

export interface TestCrawlResult {
  articles_found: number
  success_rate: number
  warnings: string[]
  sample_articles: SampleArticle[]
}

export interface SampleArticle {
  title: string
  body: string
  url: string
  published_date: string
  author: string
  quality_score: number
}

export interface SourceMetadata {
  title: string
  description: string
  feed_url?: string
  sitemap_url?: string
  selectors?: Partial<SelectorConfig>
}

export interface SourceListParams {
  search?: string
  enabled?: boolean
  feed_active?: boolean
  page?: number
  limit?: number
  sort_by?: string
  sort_order?: 'asc' | 'desc'
}

export const SOURCE_TYPES = [
  'news',
  'indigenous',
  'government',
  'mining',
  'community',
  'structured',
  'api',
] as const

export type SourceType = (typeof SOURCE_TYPES)[number]

export const INGESTION_MODES = ['crawl', 'feed', 'sitemap', 'api'] as const

export const RENDER_MODES = ['static', 'dynamic'] as const
