import { watch, onMounted, onUnmounted, ref } from 'vue'
import { useHealthStore } from '@/stores/health'
import { useRealtimeStore } from '@/stores/realtime'
import { useUIStore } from '@/stores/ui'
import type { ServiceHealthEvent } from '@/types/realtime'
import type { ServiceStatus } from '@/types/health'

/**
 * Composable for integrating real-time health updates with the health store
 *
 * When enabled, service health changes are received via SSE and automatically
 * update the local store state without polling.
 */
export function useHealthRealtime() {
  const healthStore = useHealthStore()
  const realtimeStore = useRealtimeStore()
  const uiStore = useUIStore()

  const lastUpdate = ref<Date | null>(null)
  const updateCount = ref(0)

  let unsubscribe: (() => void) | null = null

  function handleHealthEvent(event: ServiceHealthEvent) {
    const service = healthStore.services.find((s) => s.name === event.service)

    if (service) {
      const previousStatus = service.status
      const newStatus = event.status as ServiceStatus

      // Update service in place
      service.status = newStatus
      service.latency = event.latency
      service.details = event.details
      service.lastCheck = event.timestamp

      lastUpdate.value = new Date()
      updateCount.value++

      // Show toast for status degradation
      if (previousStatus === 'healthy' && newStatus !== 'healthy') {
        if (newStatus === 'unhealthy') {
          uiStore.error(`${event.service} is now unhealthy`, 'Service Alert')
        } else if (newStatus === 'degraded') {
          uiStore.warning(`${event.service} is experiencing issues`, 'Service Warning')
        }
      }

      // Show toast for recovery
      if (previousStatus !== 'healthy' && newStatus === 'healthy') {
        uiStore.success(`${event.service} is now healthy`, 'Service Recovered')
      }
    }

    // Update lastUpdate timestamp on store
    healthStore.lastUpdate = new Date()
  }

  function setupSubscription() {
    cleanupSubscription()

    unsubscribe = realtimeStore.subscribe<ServiceHealthEvent>(
      'health:status',
      handleHealthEvent
    )
  }

  function cleanupSubscription() {
    if (unsubscribe) {
      unsubscribe()
      unsubscribe = null
    }
  }

  // Setup when realtime becomes enabled
  watch(
    () => realtimeStore.enabled && realtimeStore.isConnected,
    (connected) => {
      if (connected) {
        setupSubscription()
        // Stop polling when realtime is active
        healthStore.stopPolling()
      } else {
        cleanupSubscription()
        // Resume polling when realtime is not available
        healthStore.startPolling()
      }
    },
    { immediate: true }
  )

  onMounted(() => {
    if (realtimeStore.enabled && realtimeStore.isConnected) {
      setupSubscription()
    }
  })

  onUnmounted(() => {
    cleanupSubscription()
  })

  return {
    lastUpdate,
    updateCount,
    isRealtime: () => realtimeStore.enabled && realtimeStore.isConnected,
  }
}
