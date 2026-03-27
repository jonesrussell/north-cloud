export type CrawlJobStatus =
  | 'pending'
  | 'scheduled'
  | 'running'
  | 'paused'
  | 'completed'
  | 'failed'
  | 'cancelled'

export interface CrawlJob {
  id: string
  source_id: string
  source_name?: string
  url: string
  type: string
  status: CrawlJobStatus
  is_paused: boolean
  schedule_enabled: boolean
  interval_minutes?: number
  interval_type: string
  next_run_at?: string
  max_retries: number
  retry_backoff_seconds: number
  current_retry_count: number
  adaptive_scheduling: boolean
  auto_managed: boolean
  priority: number
  failure_count: number
  error_message?: string
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
  started_at?: string
  completed_at?: string
  paused_at?: string
  cancelled_at?: string
  last_failure_at?: string
  backoff_until?: string
}

export interface CrawlJobsResponse {
  jobs: CrawlJob[]
  total: number
  limit: number
  offset: number
  sort_by: string
  sort_order: string
}

export interface StartCrawlRequest {
  source_id: string
  source_name?: string
  url: string
  type?: string
  interval_minutes?: number
  interval_type?: string
  schedule_enabled: boolean
}

export interface ControlJobAction {
  action: 'pause' | 'resume' | 'cancel' | 'retry'
}
