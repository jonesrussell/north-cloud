import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { crawlerApi, publisherApi, classifierApi, indexManagerApi } from '@/api/client'
import type { ServiceHealth, OverallStatus, ServiceStatus } from '@/types/health'

const DEFAULT_POLL_INTERVAL = 30000 // 30 seconds

export const useHealthStore = defineStore('health', () => {
  // State
  const services = ref<ServiceHealth[]>([
    { name: 'Crawler', status: 'checking', lastCheck: null },
    { name: 'Classifier', status: 'checking', lastCheck: null },
    { name: 'Publisher', status: 'checking', lastCheck: null },
    { name: 'Index Manager', status: 'checking', lastCheck: null },
    { name: 'Elasticsearch', status: 'checking', lastCheck: null },
    { name: 'Redis', status: 'checking', lastCheck: null },
  ])
  const isPolling = ref(false)
  const lastUpdate = ref<Date | null>(null)
  const error = ref<string | null>(null)

  // Private state
  let pollInterval: ReturnType<typeof setInterval> | null = null

  // Getters
  const overallStatus = computed<OverallStatus>(() => {
    const healthyCount = services.value.filter((s) => s.status === 'healthy').length
    const total = services.value.length
    if (healthyCount === total) return 'operational'
    if (healthyCount >= total / 2) return 'degraded'
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
    details?: string
  ) {
    const index = services.value.findIndex((s) => s.name === serviceName)
    if (index !== -1) {
      services.value[index] = {
        ...services.value[index],
        status,
        latency,
        details,
        lastCheck: new Date().toISOString(),
      }
    }
  }

  async function checkService(
    serviceName: string,
    healthFn: () => Promise<unknown>
  ): Promise<ServiceStatus> {
    const start = Date.now()
    try {
      await healthFn()
      const latency = Date.now() - start
      updateServiceHealth(serviceName, 'healthy', latency)
      return 'healthy'
    } catch (err) {
      const latency = Date.now() - start
      const errorMessage = err instanceof Error ? err.message : 'Connection failed'
      updateServiceHealth(serviceName, 'unhealthy', latency, errorMessage)
      return 'unhealthy'
    }
  }

  async function checkAllServices() {
    error.value = null

    // Check all services in parallel
    const results = await Promise.allSettled([
      checkService('Crawler', () => crawlerApi.getHealth()),
      checkService('Classifier', () => classifierApi.getHealth()),
      checkService('Publisher', () => publisherApi.getHealth()),
      checkService('Index Manager', () => indexManagerApi.getHealth()),
    ])

    // Infer Elasticsearch health from Index Manager
    const indexManagerStatus = services.value.find((s) => s.name === 'Index Manager')?.status
    updateServiceHealth(
      'Elasticsearch',
      indexManagerStatus === 'healthy' ? 'healthy' : 'unknown',
      undefined,
      indexManagerStatus === 'healthy' ? undefined : 'Status inferred from Index Manager'
    )

    // Infer Redis health from Publisher
    const publisherStatus = services.value.find((s) => s.name === 'Publisher')?.status
    updateServiceHealth(
      'Redis',
      publisherStatus === 'healthy' ? 'healthy' : 'unknown',
      undefined,
      publisherStatus === 'healthy' ? undefined : 'Status inferred from Publisher'
    )

    lastUpdate.value = new Date()

    // Check if any service failed
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
    services.value = services.value.map((s) => ({
      ...s,
      status: 'checking' as ServiceStatus,
      lastCheck: null,
      latency: undefined,
      details: undefined,
    }))
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
