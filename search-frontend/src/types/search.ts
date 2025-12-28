/**
 * Search result from Elasticsearch
 */
export interface SearchResult {
  id: string
  title: string
  url: string
  body?: string
  raw_text?: string
  published_date?: string
  quality_score?: number
  topics?: string[]
  content_type?: string
  source?: string
  highlight?: {
    title?: string[]
    body?: string[]
    raw_text?: string[]
    [key: string]: string[] | undefined
  }
  [key: string]: unknown
}

/**
 * Search filters
 */
export interface SearchFilters {
  topics?: string[]
  content_type?: string | null
  min_quality_score?: number
  from_date?: string | null
  to_date?: string | null
  source_names?: string[]
}

/**
 * Facet aggregation from Elasticsearch
 */
export interface Facet {
  [key: string]: {
    buckets: Array<{
      key: string
      doc_count: number
    }>
  }
}

/**
 * Search response from API
 */
export interface SearchResponse {
  hits: SearchResult[]
  total_hits: number
  facets?: Facet | null
  took?: number
  [key: string]: unknown
}

/**
 * Search request payload
 */
export interface SearchRequest {
  query: string
  filters?: SearchFilters
  pagination?: {
    page: number
    size: number
  }
  sort?: {
    field: string
    order: 'asc' | 'desc'
  }
  options?: {
    include_highlights?: boolean
    include_facets?: boolean
  }
}

/**
 * Search state interface
 */
export interface SearchState {
  query: string
  results: SearchResult[]
  facets: Facet | null
  totalHits: number
  currentPage: number
  pageSize: number
  loading: boolean
  error: string | null
  filters: SearchFilters
  sortBy: string
  sortOrder: 'asc' | 'desc'
}

