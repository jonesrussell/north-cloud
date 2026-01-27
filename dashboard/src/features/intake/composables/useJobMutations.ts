/**
 * Jobs Mutation Composables
 *
 * TanStack Query mutation hooks for job CRUD and control actions.
 * Handles optimistic updates, cache invalidation, and toast notifications.
 */

import { useMutation, useQueryClient } from '@tanstack/vue-query'
import {
  jobsKeys,
  createJob as createJobApi,
  updateJob as updateJobApi,
  deleteJob as deleteJobApi,
  pauseJob as pauseJobApi,
  resumeJob as resumeJobApi,
  cancelJob as cancelJobApi,
  retryJob as retryJobApi,
} from '../api/jobs'
import { useJobsUIStore } from '../stores/useJobsUIStore'
import { useToast } from '@/composables/useToast'
import type { Job, CreateJobRequest, UpdateJobRequest } from '@/types/crawler'

// ============================================================================
// Create Job Mutation
// ============================================================================

/**
 * Create a new job
 */
export function useCreateJobMutation() {
  const queryClient = useQueryClient()
  const uiStore = useJobsUIStore()
  const { toast } = useToast()

  return useMutation({
    mutationFn: (data: CreateJobRequest) => createJobApi(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: jobsKeys.lists() })
      toast.success('Job created successfully')
      uiStore.closeModal('create')
    },
    onError: (error: Error & { response?: { data?: { error?: string } } }) => {
      const message = error.response?.data?.error || error.message || 'Failed to create job'
      toast.error(message)
    },
  })
}

// ============================================================================
// Update Job Mutation
// ============================================================================

/**
 * Update an existing job
 */
export function useUpdateJobMutation() {
  const queryClient = useQueryClient()
  const uiStore = useJobsUIStore()
  const { toast } = useToast()

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateJobRequest }) =>
      updateJobApi(id, data),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: jobsKeys.detail(variables.id) })
      queryClient.invalidateQueries({ queryKey: jobsKeys.lists() })
      toast.success('Job updated successfully')
      uiStore.closeModal('edit')
    },
    onError: (error: Error & { response?: { data?: { error?: string } } }) => {
      const message = error.response?.data?.error || error.message || 'Failed to update job'
      toast.error(message)
    },
  })
}

// ============================================================================
// Delete Job Mutation
// ============================================================================

/**
 * Delete a job
 */
export function useDeleteJobMutation() {
  const queryClient = useQueryClient()
  const uiStore = useJobsUIStore()
  const { toast } = useToast()

  return useMutation({
    mutationFn: (id: string) => deleteJobApi(id),
    onSuccess: (_data, id) => {
      queryClient.removeQueries({ queryKey: jobsKeys.detail(id) })
      queryClient.invalidateQueries({ queryKey: jobsKeys.lists() })
      toast.success('Job deleted successfully')
      uiStore.closeModal('delete')
      uiStore.clearSelection()
    },
    onError: (error: Error & { response?: { data?: { error?: string } } }) => {
      const message = error.response?.data?.error || error.message || 'Failed to delete job'
      toast.error(message)
    },
  })
}

// ============================================================================
// Job Control Mutations (Pause, Resume, Cancel, Retry)
// ============================================================================

/**
 * Pause a job with optimistic update
 */
export function usePauseJobMutation() {
  const queryClient = useQueryClient()
  const uiStore = useJobsUIStore()
  const { toast } = useToast()

  return useMutation({
    mutationFn: (id: string) => pauseJobApi(id),
    onMutate: async (id) => {
      // Cancel in-flight queries
      await queryClient.cancelQueries({ queryKey: jobsKeys.detail(id) })

      // Snapshot previous value
      const previousJob = queryClient.getQueryData<Job>(jobsKeys.detail(id))

      // Optimistic update
      if (previousJob) {
        queryClient.setQueryData<Job>(jobsKeys.detail(id), {
          ...previousJob,
          status: 'paused',
        })
      }

      uiStore.setActionInProgress(id)
      return { previousJob }
    },
    onSuccess: (_data, id) => {
      queryClient.invalidateQueries({ queryKey: jobsKeys.detail(id) })
      queryClient.invalidateQueries({ queryKey: jobsKeys.lists() })
      toast.success('Job paused')
    },
    onError: (error: Error & { response?: { data?: { error?: string } } }, id, context) => {
      // Rollback optimistic update
      if (context?.previousJob) {
        queryClient.setQueryData(jobsKeys.detail(id), context.previousJob)
      }
      const message = error.response?.data?.error || error.message || 'Failed to pause job'
      toast.error(message)
    },
    onSettled: () => {
      uiStore.setActionInProgress(null)
    },
  })
}

/**
 * Resume a paused job with optimistic update
 */
export function useResumeJobMutation() {
  const queryClient = useQueryClient()
  const uiStore = useJobsUIStore()
  const { toast } = useToast()

  return useMutation({
    mutationFn: (id: string) => resumeJobApi(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: jobsKeys.detail(id) })

      const previousJob = queryClient.getQueryData<Job>(jobsKeys.detail(id))

      if (previousJob) {
        queryClient.setQueryData<Job>(jobsKeys.detail(id), {
          ...previousJob,
          status: 'scheduled',
        })
      }

      uiStore.setActionInProgress(id)
      return { previousJob }
    },
    onSuccess: (_data, id) => {
      queryClient.invalidateQueries({ queryKey: jobsKeys.detail(id) })
      queryClient.invalidateQueries({ queryKey: jobsKeys.lists() })
      toast.success('Job resumed')
    },
    onError: (error: Error & { response?: { data?: { error?: string } } }, id, context) => {
      if (context?.previousJob) {
        queryClient.setQueryData(jobsKeys.detail(id), context.previousJob)
      }
      const message = error.response?.data?.error || error.message || 'Failed to resume job'
      toast.error(message)
    },
    onSettled: () => {
      uiStore.setActionInProgress(null)
    },
  })
}

/**
 * Cancel a running job
 */
export function useCancelJobMutation() {
  const queryClient = useQueryClient()
  const uiStore = useJobsUIStore()
  const { toast } = useToast()

  return useMutation({
    mutationFn: (id: string) => cancelJobApi(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: jobsKeys.detail(id) })

      const previousJob = queryClient.getQueryData<Job>(jobsKeys.detail(id))

      if (previousJob) {
        queryClient.setQueryData<Job>(jobsKeys.detail(id), {
          ...previousJob,
          status: 'cancelled',
        })
      }

      uiStore.setActionInProgress(id)
      return { previousJob }
    },
    onSuccess: (_data, id) => {
      queryClient.invalidateQueries({ queryKey: jobsKeys.detail(id) })
      queryClient.invalidateQueries({ queryKey: jobsKeys.lists() })
      toast.success('Job cancelled')
      uiStore.cancelActionConfirmation()
    },
    onError: (error: Error & { response?: { data?: { error?: string } } }, id, context) => {
      if (context?.previousJob) {
        queryClient.setQueryData(jobsKeys.detail(id), context.previousJob)
      }
      const message = error.response?.data?.error || error.message || 'Failed to cancel job'
      toast.error(message)
    },
    onSettled: () => {
      uiStore.setActionInProgress(null)
    },
  })
}

/**
 * Retry a failed job
 */
export function useRetryJobMutation() {
  const queryClient = useQueryClient()
  const uiStore = useJobsUIStore()
  const { toast } = useToast()

  return useMutation({
    mutationFn: (id: string) => retryJobApi(id),
    onMutate: (id) => {
      uiStore.setActionInProgress(id)
    },
    onSuccess: (_data, id) => {
      queryClient.invalidateQueries({ queryKey: jobsKeys.detail(id) })
      queryClient.invalidateQueries({ queryKey: jobsKeys.executions(id) })
      queryClient.invalidateQueries({ queryKey: jobsKeys.lists() })
      toast.success('Job retry initiated')
    },
    onError: (error: Error & { response?: { data?: { error?: string } } }) => {
      const message = error.response?.data?.error || error.message || 'Failed to retry job'
      toast.error(message)
    },
    onSettled: () => {
      uiStore.setActionInProgress(null)
    },
  })
}

// ============================================================================
// Bulk Operations
// ============================================================================

/**
 * Bulk pause multiple jobs
 */
export function useBulkPauseJobsMutation() {
  const queryClient = useQueryClient()
  const uiStore = useJobsUIStore()
  const { toast } = useToast()

  return useMutation({
    mutationFn: async (ids: string[]) => {
      await Promise.all(ids.map((id) => pauseJobApi(id)))
      return ids
    },
    onSuccess: (ids) => {
      ids.forEach((id) => {
        queryClient.invalidateQueries({ queryKey: jobsKeys.detail(id) })
      })
      queryClient.invalidateQueries({ queryKey: jobsKeys.lists() })
      toast.success(`${ids.length} jobs paused`)
      uiStore.cancelBulkAction()
    },
    onError: (error: Error & { response?: { data?: { error?: string } } }) => {
      const message = error.response?.data?.error || error.message || 'Failed to pause jobs'
      toast.error(message)
    },
  })
}

/**
 * Bulk delete multiple jobs
 */
export function useBulkDeleteJobsMutation() {
  const queryClient = useQueryClient()
  const uiStore = useJobsUIStore()
  const { toast } = useToast()

  return useMutation({
    mutationFn: async (ids: string[]) => {
      await Promise.all(ids.map((id) => deleteJobApi(id)))
      return ids
    },
    onSuccess: (ids) => {
      ids.forEach((id) => {
        queryClient.removeQueries({ queryKey: jobsKeys.detail(id) })
      })
      queryClient.invalidateQueries({ queryKey: jobsKeys.lists() })
      toast.success(`${ids.length} jobs deleted`)
      uiStore.cancelBulkAction()
    },
    onError: (error: Error & { response?: { data?: { error?: string } } }) => {
      const message = error.response?.data?.error || error.message || 'Failed to delete jobs'
      toast.error(message)
    },
  })
}

// ============================================================================
// Convenience Hook: All Mutations
// ============================================================================

/**
 * Returns all job mutations for use in components
 */
export function useJobMutations() {
  return {
    createJob: useCreateJobMutation(),
    updateJob: useUpdateJobMutation(),
    deleteJob: useDeleteJobMutation(),
    pauseJob: usePauseJobMutation(),
    resumeJob: useResumeJobMutation(),
    cancelJob: useCancelJobMutation(),
    retryJob: useRetryJobMutation(),
    bulkPauseJobs: useBulkPauseJobsMutation(),
    bulkDeleteJobs: useBulkDeleteJobsMutation(),
  }
}
