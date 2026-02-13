export interface CrimeAggregation {
  by_sub_label: Record<string, number>
  by_relevance: Record<string, number>
  by_crime_type: Record<string, number>
  total_crime_related: number
  total_documents: number
}

export interface MiningAggregation {
  by_relevance: Record<string, number>
  by_mining_stage: Record<string, number>
  by_commodity: Record<string, number>
  by_location: Record<string, number>
  total_mining: number
  total_documents: number
}

export interface LocationAggregation {
  by_country: Record<string, number>
  by_province: Record<string, number>
  by_city: Record<string, number>
  by_specificity: Record<string, number>
}

export interface QualityBuckets {
  high: number
  medium: number
  low: number
}

export interface OverviewAggregation {
  total_documents: number
  total_crime_related: number
  top_cities: string[]
  top_crime_types: string[]
  quality_distribution: QualityBuckets
}

export interface MLServiceHealth {
  reachable: boolean
  model_version?: string
  latency_ms?: number
  last_checked_at: string
  error?: string
}

export interface MLHealthResponse {
  crime_ml?: MLServiceHealth
  mining_ml?: MLServiceHealth
  pipeline_mode: PipelineMode
}

export interface PipelineMode {
  crime: 'hybrid' | 'rules-only' | 'disabled'
  mining: 'hybrid' | 'rules-only' | 'disabled'
}

export interface SourceHealth {
  source: string
  raw_count: number
  classified_count: number
  backlog: number
  delta_24h: number
  avg_quality: number
}

export interface SourceHealthResponse {
  sources: SourceHealth[]
  total: number
}

export interface AggregationFilters {
  crime_relevance?: string[]
  crime_sub_labels?: string[]
  crime_types?: string[]
  cities?: string[]
  provinces?: string[]
  countries?: string[]
  sources?: string[]
  min_quality?: number
}

export interface ClassificationDriftAggregation {
  by_content_type: Record<string, number>
  by_crime_relevance: Record<string, number>
  content_type_x_crime: Record<string, Record<string, number>>
  total_documents: number
}

export interface ClassificationDriftTimeseriesBucket {
  date: string
  article_count: number
  page_count: number
  other_count: number
  total: number
}

export interface ClassificationDriftTimeseriesResponse {
  buckets: ClassificationDriftTimeseriesBucket[]
}

export interface ContentTypeMismatchCount {
  count: number
}

export interface SuspectedMisclassificationDoc {
  id: string
  title: string
  canonical_url: string
  content_type: string
  crime_relevance: string
  confidence?: number
  crawled_at?: string
}

export interface SuspectedMisclassificationResponse {
  documents: SuspectedMisclassificationDoc[]
  total: number
}
