import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from 'axios'
import type { ServiceHealth, OverallStatus, ServiceStatus, HealthResponse } from '@/types/health'
import { SERVICES } from '@/types/health'

const DEFAULT_POLL_INTERVAL = 30000 // 30 seconds

function buildInitialServices(): ServiceHealth[] {
  return SERVICES.map((s) => ({
    name: s.name,
    status: 'checking' as ServiceStatus,
    lastCheck: null,
    endpoint: s.endpoint,
  }))
}

export const useHealthStore = defineStore('health', () => {
  // State
  const services = ref<ServiceHealth[]>(buildInitialServices())
  const isPolling = ref(false)
  const lastUpdate = ref<Date | null>(null)
  const error = ref<string | null>(null)

  // Private state
  let pollInterval: ReturnType<typeof setInterval> | null = null

  // Getters
  const overallStatus = computed<OverallStatus>(() => {
    const healthy = services.value.filter((s) => s.status === 'healthy').length
    const total = services.value.length
    if (healthy === total) return 'operational'
    if (healthy >= total / 2) return 'degraded'
    return 'outage'
  })

  const healthyCount = computed(() => services.value.filter((s) => s.status === 'healthy').length)

  const unhealthyServices = computed(() => services.value.filter((s) => s.status === 'unhealthy'))

  const isFullyOperational = computed(() => overallStatus.value === 'operational')

  // Actions
  function updateServiceHealth(
    serviceName: string,
    status: ServiceStatus,
    latency?: number,
    details?: string,
    response?: HealthResponse
  ) {
    const index = services.value.findIndex((s) => s.name === serviceName)
    if (index !== -1) {
      services.value[index] = {
        ...services.value[index],
        status,
        latency,
        details,
        lastCheck: new Date().toISOString(),
        uptime: response?.uptime,
        version: response?.version,
        checks: response?.checks,
      }
    }
  }

  async function checkService(service: ServiceHealth): Promise<ServiceStatus> {
    const start = Date.now()
    try {
      const { data } = await axios.get<HealthResponse>(service.endpoint)
      const latency = Date.now() - start

      // Map the Go health status to our ServiceStatus
      const status: ServiceStatus = data.status === 'healthy'
        ? 'healthy'
        : data.status === 'degraded'
          ? 'degraded'
          : 'unhealthy'

      updateServiceHealth(service.name, status, latency, undefined, data)
      return status
    } catch (err) {
      const latency = Date.now() - start
      const errorMessage = err instanceof Error ? err.message : 'Connection failed'
      updateServiceHealth(service.name, 'unhealthy', latency, errorMessage)
      return 'unhealthy'
    }
  }

  async function checkAllServices() {
    error.value = null

    const results = await Promise.allSettled(
      services.value.map((service) => checkService(service))
    )

    lastUpdate.value = new Date()

    const failedCount = results.filter((r) => r.status === 'rejected').length
    if (failedCount > 0) {
      error.value = `${failedCount} service(s) unreachable`
    }
  }

  function startPolling(interval: number = DEFAULT_POLL_INTERVAL) {
    if (isPolling.value) return

    isPolling.value = true
    // Immediate first check
    checkAllServices()
    // Then poll at interval
    pollInterval = setInterval(checkAllServices, interval)
  }

  function stopPolling() {
    if (pollInterval) {
      clearInterval(pollInterval)
      pollInterval = null
    }
    isPolling.value = false
  }

  function setServiceStatus(serviceName: string, status: ServiceStatus) {
    updateServiceHealth(serviceName, status)
  }

  // Cleanup on store disposal
  function $reset() {
    stopPolling()
    services.value = buildInitialServices()
    lastUpdate.value = null
    error.value = null
  }

  return {
    // State
    services,
    isPolling,
    lastUpdate,
    error,

    // Getters
    overallStatus,
    healthyCount,
    unhealthyServices,
    isFullyOperational,

    // Actions
    checkAllServices,
    startPolling,
    stopPolling,
    setServiceStatus,
    $reset,
  }
})
