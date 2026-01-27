// Pinia store exports
export { useHealthStore } from './health'
export { useMetricsStore } from './metrics'
export { useUIStore } from './ui'
export { useRealtimeStore } from './realtime'

/**
 * @deprecated Use useSources() from '@/features/scheduling' instead
 * Kept for backwards compatibility only
 */
export { useSourcesStore } from './sources'
export type { Source } from './sources'

/**
 * @deprecated Use useJobs() from '@/features/intake' instead
 * Kept for backwards compatibility only
 */
export { useJobsStore } from './jobs'

// Re-export types for convenience
export type { ServiceHealth, OverallStatus } from '@/types/health'
export type { PipelineMetrics, AggregatedMetrics } from '@/types/metrics'
export type { Job, JobExecution, JobStats, JobFilters, JobStatus } from '@/types/crawler'
export type { ConnectionStatus, RealtimeEvent } from '@/types/realtime'
