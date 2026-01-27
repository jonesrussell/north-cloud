/**
 * @deprecated Use the feature module instead:
 * import { useJobs, useJobDetail } from '@/features/intake'
 *
 * This store is kept for backwards compatibility only.
 * All new code should use the TanStack Query-based feature module.
 */
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { crawlerApi } from '@/api/client'
import type { Job, JobExecution, JobStats, JobFilters, JobStatus } from '@/types/crawler'

const DEFAULT_PAGE_SIZE = 25
const DEFAULT_EXECUTIONS_LIMIT = 50

/**
 * @deprecated Use useJobs() or useJobDetail() from '@/features/intake' instead
 */
export const useJobsStore = defineStore('jobs', () => {
  // State
  const items = ref<Job[]>([])
  const selectedJob = ref<Job | null>(null)
  const executions = ref<JobExecution[]>([])
  const stats = ref<JobStats | null>(null)
  const loading = ref(false)
  const executionsLoading = ref(false)
  const error = ref<string | null>(null)

  const filters = ref<JobFilters>({
    status: undefined,
    source_id: undefined,
    schedule_enabled: undefined,
    search: undefined,
  })

  const pagination = ref({
    page: 1,
    pageSize: DEFAULT_PAGE_SIZE,
    total: 0,
  })

  const executionsPagination = ref({
    offset: 0,
    limit: DEFAULT_EXECUTIONS_LIMIT,
    total: 0,
  })

  // Polling state
  let pollingInterval: ReturnType<typeof setInterval> | null = null
  const isPolling = ref(false)

  // Getters
  const filteredItems = computed(() => {
    let result = items.value

    if (filters.value.status) {
      const statusFilter = Array.isArray(filters.value.status)
        ? filters.value.status
        : [filters.value.status]
      result = result.filter((job) => statusFilter.includes(job.status))
    }

    if (filters.value.source_id) {
      result = result.filter((job) => job.source_id === filters.value.source_id)
    }

    if (filters.value.schedule_enabled !== undefined) {
      result = result.filter((job) => job.schedule_enabled === filters.value.schedule_enabled)
    }

    if (filters.value.search) {
      const searchLower = filters.value.search.toLowerCase()
      result = result.filter(
        (job) =>
          job.id.toLowerCase().includes(searchLower) ||
          job.source_name.toLowerCase().includes(searchLower) ||
          job.url.toLowerCase().includes(searchLower)
      )
    }

    return result
  })

  const paginatedItems = computed(() => {
    const start = (pagination.value.page - 1) * pagination.value.pageSize
    const end = start + pagination.value.pageSize
    return filteredItems.value.slice(start, end)
  })

  const totalPages = computed(() =>
    Math.ceil(filteredItems.value.length / pagination.value.pageSize)
  )

  const hasMoreExecutions = computed(
    () => executions.value.length < executionsPagination.value.total
  )

  // Status counts for filter badges
  const statusCounts = computed(() => {
    const counts: Record<JobStatus, number> = {
      pending: 0,
      scheduled: 0,
      running: 0,
      paused: 0,
      completed: 0,
      failed: 0,
      cancelled: 0,
    }
    for (const job of items.value) {
      counts[job.status]++
    }
    return counts
  })

  const activeJobsCount = computed(
    () => statusCounts.value.running + statusCounts.value.scheduled + statusCounts.value.pending
  )

  const failedJobsCount = computed(() => statusCounts.value.failed)

  // Actions
  async function fetchJobs() {
    loading.value = true
    error.value = null

    try {
      const response = await crawlerApi.jobs.list()
      const data = response.data?.jobs || response.data || []
      items.value = data
      pagination.value.total = data.length
    } catch (err) {
      error.value = 'Failed to load jobs. Please check if the crawler service is running.'
      console.error('Failed to fetch jobs:', err)
    } finally {
      loading.value = false
    }
  }

  async function fetchJob(id: string) {
    loading.value = true
    error.value = null

    try {
      const response = await crawlerApi.jobs.get(id)
      selectedJob.value = response.data
      return response.data
    } catch (err) {
      error.value = 'Failed to load job details.'
      console.error('Failed to fetch job:', err)
      return null
    } finally {
      loading.value = false
    }
  }

  async function fetchJobStats(id: string) {
    try {
      const response = await crawlerApi.jobs.stats(id)
      stats.value = response.data
      return response.data
    } catch (err) {
      console.error('Failed to fetch job stats:', err)
      return null
    }
  }

  async function fetchExecutions(id: string, reset = false) {
    if (reset) {
      executionsPagination.value.offset = 0
      executions.value = []
    }

    executionsLoading.value = true

    try {
      const response = await crawlerApi.jobs.executions(id, {
        limit: executionsPagination.value.limit,
        offset: executionsPagination.value.offset,
      })

      const newExecutions = response.data?.executions || response.data || []
      executionsPagination.value.total = response.data?.total || newExecutions.length

      if (reset) {
        executions.value = newExecutions
      } else {
        executions.value = [...executions.value, ...newExecutions]
      }

      executionsPagination.value.offset += newExecutions.length
      return newExecutions
    } catch (err) {
      console.error('Failed to fetch executions:', err)
      return []
    } finally {
      executionsLoading.value = false
    }
  }

  async function createJob(data: Partial<Job>) {
    try {
      const response = await crawlerApi.jobs.create(data)
      await fetchJobs() // Refresh list
      return response.data
    } catch (err) {
      error.value = 'Failed to create job.'
      console.error('Failed to create job:', err)
      throw err
    }
  }

  async function updateJob(id: string, data: Partial<Job>) {
    try {
      const response = await crawlerApi.jobs.update(id, data)
      await fetchJobs() // Refresh list
      if (selectedJob.value?.id === id) {
        await fetchJob(id)
      }
      return response.data
    } catch (err) {
      error.value = 'Failed to update job.'
      console.error('Failed to update job:', err)
      throw err
    }
  }

  async function deleteJob(id: string) {
    try {
      await crawlerApi.jobs.delete(id)
      await fetchJobs() // Refresh list
      if (selectedJob.value?.id === id) {
        selectedJob.value = null
      }
    } catch (err) {
      error.value = 'Failed to delete job.'
      console.error('Failed to delete job:', err)
      throw err
    }
  }

  // Job control actions
  async function pauseJob(id: string) {
    try {
      await crawlerApi.jobs.pause(id)
      await refreshJobState(id)
    } catch (err) {
      error.value = 'Failed to pause job.'
      throw err
    }
  }

  async function resumeJob(id: string) {
    try {
      await crawlerApi.jobs.resume(id)
      await refreshJobState(id)
    } catch (err) {
      error.value = 'Failed to resume job.'
      throw err
    }
  }

  async function cancelJob(id: string) {
    try {
      await crawlerApi.jobs.cancel(id)
      await refreshJobState(id)
    } catch (err) {
      error.value = 'Failed to cancel job.'
      throw err
    }
  }

  async function retryJob(id: string) {
    try {
      await crawlerApi.jobs.retry(id)
      await refreshJobState(id)
    } catch (err) {
      error.value = 'Failed to retry job.'
      throw err
    }
  }

  async function refreshJobState(id: string) {
    // Refresh both list and selected job if applicable
    await fetchJobs()
    if (selectedJob.value?.id === id) {
      await fetchJob(id)
    }
  }

  // Filter actions
  function setFilter<K extends keyof JobFilters>(key: K, value: JobFilters[K]) {
    filters.value[key] = value
    pagination.value.page = 1 // Reset to first page when filtering
  }

  function clearFilters() {
    filters.value = {
      status: undefined,
      source_id: undefined,
      schedule_enabled: undefined,
      search: undefined,
    }
    pagination.value.page = 1
  }

  function setPage(page: number) {
    pagination.value.page = page
  }

  function setPageSize(size: number) {
    pagination.value.pageSize = size
    pagination.value.page = 1
  }

  // Polling
  function startPolling(interval: number = 30000) {
    if (pollingInterval) {
      clearInterval(pollingInterval)
    }

    isPolling.value = true
    pollingInterval = setInterval(() => {
      fetchJobs()
    }, interval)
  }

  function stopPolling() {
    if (pollingInterval) {
      clearInterval(pollingInterval)
      pollingInterval = null
    }
    isPolling.value = false
  }

  function $reset() {
    stopPolling()
    items.value = []
    selectedJob.value = null
    executions.value = []
    stats.value = null
    loading.value = false
    error.value = null
    clearFilters()
  }

  return {
    // State
    items,
    selectedJob,
    executions,
    stats,
    loading,
    executionsLoading,
    error,
    filters,
    pagination,
    executionsPagination,
    isPolling,

    // Getters
    filteredItems,
    paginatedItems,
    totalPages,
    hasMoreExecutions,
    statusCounts,
    activeJobsCount,
    failedJobsCount,

    // Actions
    fetchJobs,
    fetchJob,
    fetchJobStats,
    fetchExecutions,
    createJob,
    updateJob,
    deleteJob,
    pauseJob,
    resumeJob,
    cancelJob,
    retryJob,

    // Filter actions
    setFilter,
    clearFilters,
    setPage,
    setPageSize,

    // Polling
    startPolling,
    stopPolling,

    $reset,
  }
})
