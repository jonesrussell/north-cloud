// Crawler service types

export type JobStatus =
  | 'pending'
  | 'scheduled'
  | 'running'
  | 'paused'
  | 'completed'
  | 'failed'
  | 'cancelled'

export interface Job {
  id: string
  source_id: string
  source_name: string
  url: string
  status: JobStatus
  created_at: string
  updated_at?: string
  next_run_at?: string
  last_run_at?: string
  schedule_enabled: boolean
  interval_minutes?: number
  interval_type?: 'minutes' | 'hours' | 'days'
  retry_count?: number
  max_retries?: number
  error_message?: string
}

export interface JobExecution {
  id: string
  job_id: string
  status: 'running' | 'completed' | 'failed'
  started_at: string
  completed_at?: string
  duration_ms?: number
  articles_found?: number
  articles_indexed?: number
  error_message?: string
}

export interface JobStats {
  total_executions: number
  successful_executions: number
  failed_executions: number
  average_duration_ms: number
  last_execution?: JobExecution
}

export interface CreateJobRequest {
  source_id: string
  source_name?: string
  url: string
  schedule_enabled?: boolean
  interval_minutes?: number
  interval_type?: 'minutes' | 'hours' | 'days'
}

export interface UpdateJobRequest {
  schedule_enabled?: boolean
  interval_minutes?: number
  interval_type?: 'minutes' | 'hours' | 'days'
}

export interface JobFilters {
  status?: JobStatus | JobStatus[]
  source_id?: string
  schedule_enabled?: boolean
  search?: string
}

export type JobStatusCounts = Record<JobStatus, number>

export interface CrawlerStats {
  crawled_today: number
  indexed_today: number
  total_jobs: number
  active_jobs: number
  failed_jobs_24h: number
}

/** Response from POST /admin/sync-enabled-sources (create/resume jobs for enabled sources). */
export interface SyncReport {
  sources_seen: number
  sources_enabled: number
  created: string[]
  resumed: string[]
  already_has_job: string[]
  skipped_disabled: string[]
  errors: string[]
}

// Job status badge variants
export const JOB_STATUS_VARIANTS: Record<JobStatus, string> = {
  pending: 'secondary',
  scheduled: 'default',
  running: 'default',
  paused: 'warning',
  completed: 'success',
  failed: 'destructive',
  cancelled: 'secondary',
} as const
