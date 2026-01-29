// Job logging types

export type LogCategory =
  | 'crawler.lifecycle'
  | 'crawler.fetch'
  | 'crawler.extract'
  | 'crawler.error'
  | 'crawler.rate_limit'
  | 'crawler.queue'
  | 'crawler.metrics'

export type LogLevel = 'debug' | 'info' | 'warn' | 'error'

export interface LogLine {
  schema_version?: number
  timestamp: string
  level: LogLevel
  category?: LogCategory
  message: string
  job_id?: string
  execution_id?: string
  fields?: Record<string, unknown>
}

export interface JobSummary {
  pages_discovered: number
  pages_crawled: number
  items_extracted: number
  errors_count: number
  duration_ms: number
  crawl_duration_ms?: number
  extract_duration_ms?: number
  backoff_duration_ms?: number
  bytes_fetched?: number
  requests_total?: number
  requests_failed?: number
  queue_max_depth?: number
  queue_enqueued?: number
  queue_dequeued?: number
  status_codes?: Record<number, number>
  top_errors?: ErrorSummary[]
  logs_emitted?: number
  logs_throttled?: number
  throttle_percent?: number
}

export interface ErrorSummary {
  message: string
  count: number
  last_url?: string
}

// SSE Event types
export interface LogReplayEvent {
  type: 'log:replay'
  data: {
    lines: LogLine[]
    count: number
  }
}

export interface LogLineEvent {
  type: 'log:line'
  data: LogLine
}

export interface LogArchivedEvent {
  type: 'log:archived'
  data: {
    job_id: string
    execution_id: string
    object_key: string
  }
}

export interface LogConnectedEvent {
  type: 'connected'
  data: {
    message: string
    job_id: string
  }
}

export type LogSSEEvent =
  | LogReplayEvent
  | LogLineEvent
  | LogArchivedEvent
  | LogConnectedEvent

// Helper to get short category name for display
export function getCategoryShortName(category: LogCategory | string): string {
  return category.replace('crawler.', '')
}

// Level order for filtering (lower index = less verbose)
export const LOG_LEVEL_ORDER: LogLevel[] = ['error', 'warn', 'info', 'debug']

export function shouldShowLevel(lineLevel: LogLevel, filterLevel: LogLevel): boolean {
  const lineIndex = LOG_LEVEL_ORDER.indexOf(lineLevel)
  const filterIndex = LOG_LEVEL_ORDER.indexOf(filterLevel)
  return lineIndex <= filterIndex
}
