// Classifier Types

// Rule Types
export type RuleType = 'content_type' | 'topic' | 'quality'

export interface ClassificationRule {
  id: number
  rule_name: string
  rule_type: RuleType
  topic_name: string
  keywords: string[]
  min_confidence: number
  enabled: boolean
  priority: number
  created_at: string
  updated_at: string
}

export interface RuleCreateRequest {
  rule_name: string
  rule_type: RuleType
  topic_name: string
  keywords: string[]
  min_confidence: number
  enabled: boolean
  priority: number
}

export interface RuleUpdateRequest {
  rule_name?: string
  rule_type?: RuleType
  topic_name?: string
  keywords?: string[]
  min_confidence?: number
  enabled?: boolean
  priority?: number
}

export interface RuleTestRequest {
  content: string
}

export interface RuleTestResult {
  matched: boolean
  score: number
  coverage: number
  matched_keywords: string[]
  match_count: number
  unique_matches: number
}

export interface RulesListResponse {
  rules: ClassificationRule[]
  count: number
}

// Source Reputation Types
export interface SourceReputation {
  source_name: string
  category: string
  reputation_score: number
  total_articles: number
  average_quality_score: number
  spam_count: number
  last_classified_at: string | null
}

export interface SourceReputationUpdateRequest {
  category?: string
  reputation_score?: number
}

// Statistics Types
export interface ClassifierStats {
  total_classified: number
  total_pending: number
  total_failed: number
  classified_today: number
  average_quality_score: number
  classification_rate: number // docs per minute
  topics_distribution: Record<string, number>
  content_types_distribution: Record<string, number>
}

export interface TopicStats {
  topic: string
  count: number
  percentage: number
  avg_quality: number
}

export interface SourceStats {
  source_name: string
  total_classified: number
  avg_quality_score: number
  top_topics: string[]
}

// DLQ Types (for future dashboard display)
export interface DLQStats {
  pending: number
  exhausted: number
  ready: number
  avg_retries: number
  oldest_entry: string | null
}

export interface DLQEntry {
  id: string
  content_id: string
  source_name: string
  index_name: string
  error_message: string
  error_code: string
  retry_count: number
  max_retries: number
  next_retry_at: string
  created_at: string
  last_attempt_at: string
}

// Outbox Types (for future dashboard display)
export interface OutboxStats {
  pending: number
  publishing: number
  published: number
  failed_retryable: number
  failed_exhausted: number
  avg_publish_lag_seconds: number
}

// Classification History Types
export interface ClassificationHistory {
  id: string
  content_id: string
  content_url: string
  source_name: string
  content_type: string
  quality_score: number
  topics: string[]
  is_crime_related: boolean
  processing_time_ms: number
  classified_at: string
}

// Topic Categories (matches backend)
export const TOPIC_CATEGORIES = [
  // Crime sub-categories (highest priority)
  { value: 'violent_crime', label: 'Violent Crime', priority: 10 },
  { value: 'property_crime', label: 'Property Crime', priority: 10 },
  { value: 'drug_crime', label: 'Drug Crime', priority: 10 },
  { value: 'organized_crime', label: 'Organized Crime', priority: 10 },
  { value: 'criminal_justice', label: 'Criminal Justice', priority: 10 },
  // Other topics
  { value: 'politics', label: 'Politics', priority: 5 },
  { value: 'technology', label: 'Technology', priority: 5 },
  { value: 'business', label: 'Business', priority: 5 },
  { value: 'sports', label: 'Sports', priority: 5 },
  { value: 'entertainment', label: 'Entertainment', priority: 5 },
  { value: 'health', label: 'Health', priority: 5 },
  { value: 'science', label: 'Science', priority: 5 },
  { value: 'education', label: 'Education', priority: 5 },
  { value: 'environment', label: 'Environment', priority: 5 },
  { value: 'local', label: 'Local News', priority: 5 },
] as const

export const RULE_TYPE_OPTIONS = [
  { value: 'topic', label: 'Topic', description: 'Classify content into topics' },
  { value: 'content_type', label: 'Content Type', description: 'Detect article, page, video, etc.' },
  { value: 'quality', label: 'Quality', description: 'Influence quality scoring' },
] as const

// Helper to group rules by topic
export function groupRulesByTopic(rules: ClassificationRule[]): Record<string, ClassificationRule[]> {
  return rules.reduce((acc, rule) => {
    const topic = rule.topic_name || 'uncategorized'
    if (!acc[topic]) {
      acc[topic] = []
    }
    acc[topic].push(rule)
    return acc
  }, {} as Record<string, ClassificationRule[]>)
}

// Helper to sort rules by priority
export function sortRulesByPriority(rules: ClassificationRule[]): ClassificationRule[] {
  return [...rules].sort((a, b) => b.priority - a.priority)
}
