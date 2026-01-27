import { watch, onMounted, onUnmounted, ref } from 'vue'
import { useJobsStore } from '@/stores/jobs'
import { useRealtimeStore } from '@/stores/realtime'
import { useUIStore } from '@/stores/ui'
import type { JobStatusEvent, JobProgressEvent, JobCompletedEvent } from '@/types/realtime'

/**
 * Composable for integrating real-time job updates with the jobs store
 *
 * When enabled, job status changes are received via SSE and automatically
 * update the local store state without polling.
 */
export function useJobsRealtime() {
  const jobsStore = useJobsStore()
  const realtimeStore = useRealtimeStore()
  const uiStore = useUIStore()

  const lastUpdate = ref<Date | null>(null)
  const updateCount = ref(0)

  let unsubscribes: Array<() => void> = []

  function handleJobStatus(event: JobStatusEvent) {
    const job = jobsStore.items.find((j) => j.id === event.job_id)
    if (job) {
      // Update job status in place
      job.status = event.status as typeof job.status
      if (event.details?.next_run_at) {
        job.next_run_at = event.details.next_run_at
      }
      if (event.details?.error_message) {
        job.error_message = event.details.error_message
      }

      lastUpdate.value = new Date()
      updateCount.value++

      // Show toast for important status changes
      if (event.status === 'failed') {
        uiStore.error(`Job ${event.job_id.slice(0, 8)} failed`, 'Job Error')
      }
    }

    // Also update selected job if it matches
    if (jobsStore.selectedJob?.id === event.job_id) {
      jobsStore.selectedJob.status = event.status as typeof jobsStore.selectedJob.status
    }
  }

  function handleJobProgress(event: JobProgressEvent) {
    // Find execution in the store and update progress
    const execution = jobsStore.executions.find((e) => e.id === event.execution_id)
    if (execution) {
      execution.articles_found = event.articles_found
      execution.articles_indexed = event.articles_indexed
    }

    lastUpdate.value = new Date()
    updateCount.value++
  }

  function handleJobCompleted(event: JobCompletedEvent) {
    // Update job status
    const job = jobsStore.items.find((j) => j.id === event.job_id)
    if (job) {
      job.status = event.status
      job.last_run_at = event.timestamp

      // Show toast notification
      if (event.status === 'completed') {
        uiStore.success(
          `Indexed ${event.articles_indexed} articles in ${Math.round(event.duration_ms / 1000)}s`,
          'Job Completed'
        )
      } else if (event.status === 'failed') {
        uiStore.error(event.error_message || 'Unknown error', 'Job Failed')
      }
    }

    // Update execution
    const execution = jobsStore.executions.find((e) => e.id === event.execution_id)
    if (execution) {
      execution.status = event.status
      execution.completed_at = event.timestamp
      execution.duration_ms = event.duration_ms
      execution.articles_indexed = event.articles_indexed
      execution.error_message = event.error_message
    }

    lastUpdate.value = new Date()
    updateCount.value++
  }

  function setupSubscriptions() {
    // Clean up existing subscriptions
    cleanupSubscriptions()

    // Subscribe to job events
    unsubscribes.push(
      realtimeStore.subscribe<JobStatusEvent>('job:status', handleJobStatus)
    )
    unsubscribes.push(
      realtimeStore.subscribe<JobProgressEvent>('job:progress', handleJobProgress)
    )
    unsubscribes.push(
      realtimeStore.subscribe<JobCompletedEvent>('job:completed', handleJobCompleted)
    )
  }

  function cleanupSubscriptions() {
    for (const unsub of unsubscribes) {
      unsub()
    }
    unsubscribes = []
  }

  // Setup when realtime becomes enabled
  watch(
    () => realtimeStore.enabled && realtimeStore.isConnected,
    (connected) => {
      if (connected) {
        setupSubscriptions()
        // Stop polling when realtime is active
        jobsStore.stopPolling()
      } else {
        cleanupSubscriptions()
        // Resume polling when realtime is not available
        jobsStore.startPolling()
      }
    },
    { immediate: true }
  )

  onMounted(() => {
    if (realtimeStore.enabled && realtimeStore.isConnected) {
      setupSubscriptions()
    }
  })

  onUnmounted(() => {
    cleanupSubscriptions()
  })

  return {
    lastUpdate,
    updateCount,
    isRealtime: () => realtimeStore.enabled && realtimeStore.isConnected,
  }
}
