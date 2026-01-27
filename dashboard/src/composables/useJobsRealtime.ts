import { watch, onMounted, onUnmounted, ref } from 'vue'
import { useQueryClient } from '@tanstack/vue-query'
import { useRealtimeStore } from '@/stores/realtime'
import { useToast } from '@/composables/useToast'
import { jobsKeys } from '@/features/intake/api/jobs'
import type { JobStatusEvent, JobProgressEvent, JobCompletedEvent } from '@/types/realtime'

/**
 * Composable for integrating real-time job updates with TanStack Query
 *
 * When enabled, job status changes are received via SSE and automatically
 * invalidate the query cache, triggering refetches for fresh data.
 */
export function useJobsRealtime() {
  const queryClient = useQueryClient()
  const realtimeStore = useRealtimeStore()
  const toast = useToast()

  const lastUpdate = ref<Date | null>(null)
  const updateCount = ref(0)

  let unsubscribes: Array<() => void> = []

  function handleJobStatus(event: JobStatusEvent) {
    // Invalidate job list and specific job detail queries
    queryClient.invalidateQueries({ queryKey: jobsKeys.lists() })

    if (event.job_id) {
      queryClient.invalidateQueries({ queryKey: jobsKeys.detail(event.job_id) })
    }

    lastUpdate.value = new Date()
    updateCount.value++

    // Show toast for important status changes
    if (event.status === 'failed') {
      toast.error(`Job ${event.job_id.slice(0, 8)} failed`, {
        description: event.details?.error_message || 'Unknown error',
      })
    }
  }

  function handleJobProgress(event: JobProgressEvent) {
    // Invalidate executions query for this job
    if (event.job_id) {
      queryClient.invalidateQueries({
        queryKey: jobsKeys.executions(event.job_id),
      })
    }

    lastUpdate.value = new Date()
    updateCount.value++
  }

  function handleJobCompleted(event: JobCompletedEvent) {
    // Invalidate all relevant queries
    queryClient.invalidateQueries({ queryKey: jobsKeys.lists() })

    if (event.job_id) {
      queryClient.invalidateQueries({ queryKey: jobsKeys.detail(event.job_id) })
      queryClient.invalidateQueries({ queryKey: jobsKeys.executions(event.job_id) })
      queryClient.invalidateQueries({ queryKey: jobsKeys.stats(event.job_id) })
    }

    lastUpdate.value = new Date()
    updateCount.value++

    // Show toast notification
    if (event.status === 'completed') {
      const duration = event.duration_ms ? Math.round(event.duration_ms / 1000) : 0
      toast.success('Job completed', {
        description: `Indexed ${event.articles_indexed || 0} articles in ${duration}s`,
      })
    } else if (event.status === 'failed') {
      toast.error('Job failed', {
        description: event.error_message || 'Unknown error',
      })
    }
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
      } else {
        cleanupSubscriptions()
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
