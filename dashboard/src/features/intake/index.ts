/**
 * Intake Feature Module
 *
 * Content Intake (Crawler Jobs) feature module.
 * Provides all components, stores, composables, and types for managing crawler jobs.
 */

// API
export * from './api'

// Stores
export * from './stores'

// Composables
export * from './composables'

// Re-export types from central types (for convenience)
export type {
  Job,
  JobExecution,
  JobStats,
  JobStatus,
  JobFilters,
  CreateJobRequest,
  UpdateJobRequest,
  CrawlerStats,
} from '@/types/crawler'
