// Index Types
export type IndexType = 'raw_content' | 'classified_content' | 'article' | 'page'

export type IndexStatus = 'active' | 'archived' | 'deleted'

export type HealthStatus = 'green' | 'yellow' | 'red'

// Main Index Interface
export interface Index {
  name: string
  type: IndexType
  source_name?: string
  health?: HealthStatus
  status?: IndexStatus
  document_count?: number
  size?: string
  mapping_version?: string
  created_at?: string
  updated_at?: string
}

// Request Types
export interface CreateIndexRequest {
  index_name: string
  index_type: IndexType
  source_name?: string
  mapping?: Record<string, unknown>
}

export interface CreateSourceIndexesRequest {
  index_types?: IndexType[]
}

// Response Types
export interface ListIndexesResponse {
  indices: Index[]
  count: number
}

export interface GetIndexResponse extends Index {}

export interface IndexHealthResponse {
  index_name: string
  health: {
    status: HealthStatus
    number_of_shards: number
    number_of_replicas: number
    active_primary_shards: number
    active_shards: number
    relocating_shards: number
    initializing_shards: number
    unassigned_shards: number
  }
}

export interface CreateSourceIndexesResponse {
  source_name: string
  indices: Index[]
  count: number
}

export interface IndexStats {
  total_indexes: number
  indexes_by_type: Record<string, number>
  total_documents: number
  indexed_today?: number
  cluster_health: HealthStatus
  indexes_by_health: Record<string, number>
}

// Predefined index types for UI
export const INDEX_TYPE_OPTIONS = [
  { value: 'raw_content', label: 'Raw Content', description: 'Minimally-processed crawled content' },
  { value: 'classified_content', label: 'Classified Content', description: 'Enriched classified content' },
  { value: 'article', label: 'Article (Deprecated)', description: 'Legacy article format' },
  { value: 'page', label: 'Page (Deprecated)', description: 'Legacy page format' },
] as const

// Document Types
export interface Document {
  id: string
  title?: string
  url?: string
  source_name?: string
  published_date?: string
  crawled_at?: string
  created_at?: string
  updated_at?: string
  quality_score?: number
  content_type?: string
  topics?: string[]
  is_crime_related?: boolean
  body?: string
  raw_text?: string
  raw_html?: string
  meta?: {
    twitter_card?: string
    twitter_site?: string
    og_image_width?: number
    og_image_height?: number
    og_site_name?: string
    created_at?: string
    updated_at?: string
    article_opinion?: boolean
    article_content_tier?: string
    [key: string]: unknown
  }
}

export interface DocumentFilters {
  title?: string
  url?: string
  content_type?: string
  min_quality_score?: number
  max_quality_score?: number
  topics?: string[]
  from_date?: string
  to_date?: string
  from_crawled_at?: string
  to_crawled_at?: string
  is_crime_related?: boolean
}

export interface DocumentPagination {
  page: number
  size: number
}

export interface DocumentSort {
  field: string // relevance, published_date, crawled_at, quality_score, title
  order: string // asc, desc
}

export interface DocumentQueryRequest {
  query?: string
  filters?: DocumentFilters
  pagination?: DocumentPagination
  sort?: DocumentSort
}

export interface DocumentQueryResponse {
  documents: Document[]
  total_hits: number
  total_pages: number
  current_page: number
  page_size: number
}

export interface BulkDeleteRequest {
  document_ids: string[]
}
