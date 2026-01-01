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
