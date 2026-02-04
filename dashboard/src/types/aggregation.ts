export interface CrimeAggregation {
  by_sub_label: Record<string, number>
  by_relevance: Record<string, number>
  by_crime_type: Record<string, number>
  total_crime_related: number
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
