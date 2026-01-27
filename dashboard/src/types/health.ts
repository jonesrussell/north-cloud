// Health monitoring types

export type ServiceStatus = 'healthy' | 'degraded' | 'unhealthy' | 'checking' | 'unknown'

export interface ServiceHealth {
  name: string
  status: ServiceStatus
  latency?: number
  lastCheck: string | null
  details?: string
  endpoint?: string
}

export interface HealthCheckResult {
  service: string
  status: ServiceStatus
  latency: number
  error?: string
}

export type OverallStatus = 'operational' | 'degraded' | 'outage'

// Alert thresholds from user requirements
export const HEALTH_THRESHOLDS = {
  // Failure rate threshold (>15% over last 20 executions)
  FAILURE_RATE_PERCENT: 15,
  FAILURE_RATE_WINDOW: 20,

  // Stale lock detection (>5 minutes)
  STALE_LOCK_MINUTES: 5,

  // Delayed job threshold (>5 minutes or >2x interval)
  DELAYED_JOB_MINUTES: 5,
  DELAYED_JOB_MULTIPLIER: 2,

  // Queue backlog threshold (>100 items or >5x normal rate)
  QUEUE_BACKLOG_ITEMS: 100,
  QUEUE_BACKLOG_MULTIPLIER: 5,

  // Elasticsearch issues
  ES_DISK_PERCENT: 80,
} as const

// Service definitions
export const SERVICES = [
  { name: 'Crawler', endpoint: '/api/health/crawler' },
  { name: 'Classifier', endpoint: '/api/health/classifier' },
  { name: 'Publisher', endpoint: '/api/health/publisher' },
  { name: 'Index Manager', endpoint: '/api/health/index-manager' },
  { name: 'Elasticsearch', endpoint: null }, // Inferred from other services
  { name: 'Redis', endpoint: null }, // Inferred from publisher
] as const

export type ServiceName = (typeof SERVICES)[number]['name']
