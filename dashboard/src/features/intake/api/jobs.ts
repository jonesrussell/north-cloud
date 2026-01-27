/**
 * Jobs API Module
 *
 * Centralized API functions and query key factory for the Jobs domain.
 * This module is the single source of truth for all job-related API calls.
 */

import { crawlerApi } from '@/api/client'
import type {
  Job,
  JobExecution,
  JobStats,
  JobFilters,
  CreateJobRequest,
  UpdateJobRequest,
} from '@/types/crawler'

// ============================================================================
// Query Key Factory
// ============================================================================

/**
 * Hierarchical query keys for jobs domain.
 * Used by TanStack Query for cache management and invalidation.
 *
 * Key hierarchy:
 * - ['jobs'] - All job-related queries
 * - ['jobs', 'list', filters] - Job list with specific filters
 * - ['jobs', 'detail', id] - Single job detail
 * - ['jobs', 'detail', id, 'executions'] - Job executions
 * - ['jobs', 'detail', id, 'stats'] - Job statistics
 */
export const jobsKeys = {
  all: ['jobs'] as const,
  lists: () => [...jobsKeys.all, 'list'] as const,
  list: (filters?: JobFilters) => [...jobsKeys.lists(), filters] as const,
  details: () => [...jobsKeys.all, 'detail'] as const,
  detail: (id: string) => [...jobsKeys.details(), id] as const,
  executions: (id: string, params?: { limit?: number; offset?: number }) =>
    [...jobsKeys.detail(id), 'executions', params] as const,
  stats: (id: string) => [...jobsKeys.detail(id), 'stats'] as const,
  logs: (id: string, params?: { limit?: number; offset?: number; execution?: string }) =>
    [...jobsKeys.detail(id), 'logs', params] as const,
}

// ============================================================================
// API Response Types
// ============================================================================

export interface JobsListResponse {
  jobs: Job[]
  total: number
}

export interface JobExecutionsResponse {
  executions: JobExecution[]
  total: number
}

// ============================================================================
// Query Functions
// ============================================================================

/**
 * Fetch jobs list with optional filters
 */
export async function fetchJobs(filters?: JobFilters): Promise<JobsListResponse> {
  const params: Record<string, unknown> = {}

  if (filters?.status) {
    params.status = Array.isArray(filters.status)
      ? filters.status.join(',')
      : filters.status
  }

  if (filters?.source_id) {
    params.source_id = filters.source_id
  }

  if (filters?.schedule_enabled !== undefined) {
    params.schedule_enabled = filters.schedule_enabled
  }

  if (filters?.search) {
    params.search = filters.search
  }

  const response = await crawlerApi.jobs.list(params)
  const jobs = response.data?.jobs || response.data || []
  const total = response.data?.total || jobs.length

  return { jobs, total }
}

/**
 * Fetch single job by ID
 */
export async function fetchJob(id: string): Promise<Job> {
  const response = await crawlerApi.jobs.get(id)
  return response.data
}

/**
 * Fetch job executions with pagination
 */
export async function fetchJobExecutions(
  id: string,
  params?: { limit?: number; offset?: number }
): Promise<JobExecutionsResponse> {
  const response = await crawlerApi.jobs.executions(id, params)
  const executions = response.data?.executions || response.data || []
  const total = response.data?.total || executions.length

  return { executions, total }
}

/**
 * Fetch job statistics
 */
export async function fetchJobStats(id: string): Promise<JobStats> {
  const response = await crawlerApi.jobs.stats(id)
  return response.data
}

/**
 * Fetch job logs
 */
export async function fetchJobLogs(
  id: string,
  params?: { limit?: number; offset?: number; execution?: string }
) {
  const response = await crawlerApi.jobs.logs(id, params)
  return response.data
}

// ============================================================================
// Mutation Functions
// ============================================================================

/**
 * Create a new job
 */
export async function createJob(data: CreateJobRequest): Promise<Job> {
  const response = await crawlerApi.jobs.create(data)
  return response.data
}

/**
 * Update an existing job
 */
export async function updateJob(id: string, data: UpdateJobRequest): Promise<Job> {
  const response = await crawlerApi.jobs.update(id, data)
  return response.data
}

/**
 * Delete a job
 */
export async function deleteJob(id: string): Promise<void> {
  await crawlerApi.jobs.delete(id)
}

/**
 * Pause a job
 */
export async function pauseJob(id: string): Promise<void> {
  await crawlerApi.jobs.pause(id)
}

/**
 * Resume a paused job
 */
export async function resumeJob(id: string): Promise<void> {
  await crawlerApi.jobs.resume(id)
}

/**
 * Cancel a running job
 */
export async function cancelJob(id: string): Promise<void> {
  await crawlerApi.jobs.cancel(id)
}

/**
 * Retry a failed job
 */
export async function retryJob(id: string): Promise<void> {
  await crawlerApi.jobs.retry(id)
}
