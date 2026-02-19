/**
 * Search result from Elasticsearch
 */
export interface SearchResult {
  id: string
  title: string
  url: string
  click_url?: string
  body?: string
  raw_text?: string
  published_date?: string
  quality_score?: number
  topics?: string[]
  content_type?: string
  source?: string
  source_name?: string
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
 * Single facet bucket (backend API shape)
 */
export interface FacetBucketItem {
  key: string
  count: number
}

/**
 * Facets as returned by the search API
 */
export interface FacetsFromApi {
  topics?: FacetBucketItem[]
  content_types?: FacetBucketItem[]
  sources?: FacetBucketItem[]
  quality_ranges?: FacetBucketItem[]
}

/**
 * Facet aggregation from Elasticsearch (legacy / raw)
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
  facets?: FacetsFromApi | null
  took?: number
  [key: string]: unknown
}

/**
 * Suggest API response
 */
export interface SuggestResponse {
  suggestions: string[]
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
  facets: FacetsFromApi | null
  totalHits: number
  currentPage: number
  pageSize: number
  loading: boolean
  error: string | null
  filters: SearchFilters
  sortBy: string
  sortOrder: 'asc' | 'desc'
}

