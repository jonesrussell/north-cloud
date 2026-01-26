/**
 * Real-time event types for SSE and WebSocket communication
 */

export type ConnectionStatus = 'connected' | 'connecting' | 'disconnected' | 'error'

// Job-related events
export interface JobStatusEvent {
  type: 'job:status'
  job_id: string
  status: string
  timestamp: string
  details?: {
    error_message?: string
    next_run_at?: string
  }
}

export interface JobProgressEvent {
  type: 'job:progress'
  job_id: string
  execution_id: string
  articles_found: number
  articles_indexed: number
  timestamp: string
}

export interface JobCompletedEvent {
  type: 'job:completed'
  job_id: string
  execution_id: string
  status: 'completed' | 'failed'
  duration_ms: number
  articles_indexed: number
  error_message?: string
  timestamp: string
}

// Health-related events
export interface ServiceHealthEvent {
  type: 'health:status'
  service: string
  status: 'healthy' | 'degraded' | 'unhealthy'
  latency?: number
  details?: string
  timestamp: string
}

// Metrics-related events
export interface MetricsUpdateEvent {
  type: 'metrics:update'
  metric: string
  value: number
  timestamp: string
}

// Pipeline events
export interface PipelineStageEvent {
  type: 'pipeline:stage'
  stage: 'crawled' | 'classified' | 'published'
  count: number
  timestamp: string
}

// Union type for all realtime events
export type RealtimeEvent =
  | JobStatusEvent
  | JobProgressEvent
  | JobCompletedEvent
  | ServiceHealthEvent
  | MetricsUpdateEvent
  | PipelineStageEvent

// Event handler types
export type EventHandler<T extends RealtimeEvent = RealtimeEvent> = (event: T) => void

export interface RealtimeSubscription {
  id: string
  eventType: string
  handler: EventHandler
}

// SSE endpoint configuration
export interface SSEEndpoint {
  url: string
  name: string
  description?: string
}

export const SSE_ENDPOINTS: Record<string, SSEEndpoint> = {
  jobs: {
    url: '/api/crawler/events',
    name: 'Crawler Events',
    description: 'Real-time job status updates',
  },
  health: {
    url: '/api/health/events',
    name: 'Health Events',
    description: 'Service health status changes',
  },
  metrics: {
    url: '/api/metrics/events',
    name: 'Metrics Events',
    description: 'Pipeline metrics updates',
  },
}
