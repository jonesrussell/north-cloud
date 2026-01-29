<template>
  <div class="bg-white shadow rounded-lg overflow-hidden">
    <div class="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
      <h2 class="text-lg font-medium text-gray-900">
        Job Logs
      </h2>
      <div class="flex items-center space-x-2">
        <!-- Category Filter -->
        <select
          v-model="categoryFilter"
          class="text-sm border-gray-300 rounded-md shadow-sm focus:border-blue-500 focus:ring-blue-500"
        >
          <option value="">All Categories</option>
          <option value="crawler.lifecycle">Lifecycle</option>
          <option value="crawler.fetch">Fetch</option>
          <option value="crawler.extract">Extract</option>
          <option value="crawler.error">Errors</option>
          <option value="crawler.queue">Queue</option>
          <option value="crawler.rate_limit">Rate Limit</option>
          <option value="crawler.metrics">Metrics</option>
        </select>
        <!-- Level Filter -->
        <select
          v-model="levelFilter"
          class="text-sm border-gray-300 rounded-md shadow-sm focus:border-blue-500 focus:ring-blue-500"
        >
          <option value="">All Levels</option>
          <option value="error">Errors Only</option>
          <option value="warn">Warnings+</option>
          <option value="info">Info+</option>
          <option value="debug">Debug+</option>
        </select>
        <!-- Execution Selector -->
        <select
          v-if="!isLiveStreaming && executions.length > 0"
          v-model="selectedExecution"
          class="text-sm border-gray-300 rounded-md shadow-sm focus:border-blue-500 focus:ring-blue-500"
        >
          <option
            v-for="exec in executions"
            :key="exec.execution_id"
            :value="exec.execution_number"
          >
            Execution #{{ exec.execution_number }} - {{ formatStatus(exec.status) }}
            {{ exec.log_available ? '' : '(no logs)' }}
          </option>
        </select>
        <!-- Download Button -->
        <button
          v-if="canDownload"
          class="inline-flex items-center px-3 py-1.5 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 transition-colors"
          :disabled="downloading"
          @click="downloadLogs"
        >
          <ArrowDownTrayIcon class="h-4 w-4 mr-1" />
          {{ downloading ? 'Downloading...' : 'Download' }}
        </button>
        <!-- Live Status Indicator -->
        <span
          v-if="isLiveStreaming"
          class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800"
        >
          <span class="w-2 h-2 mr-1.5 bg-green-500 rounded-full animate-pulse" />
          Live
        </span>
      </div>
    </div>

    <!-- Loading State -->
    <div
      v-if="loading"
      class="p-8 text-center"
    >
      <LoadingSpinner
        size="md"
        text="Loading logs..."
      />
    </div>

    <!-- Error State -->
    <div
      v-else-if="error"
      class="p-6"
    >
      <ErrorAlert :message="error" />
    </div>

    <!-- No Logs Available -->
    <div
      v-else-if="!hasLiveLogs && executions.length === 0"
      class="p-8 text-center"
    >
      <p class="text-sm text-gray-500">
        No logs available for this job yet.
      </p>
    </div>

    <!-- Logs Display -->
    <div
      v-else
      class="relative"
    >
      <!-- Log Container -->
      <div
        ref="logContainer"
        class="bg-gray-900 text-gray-100 font-mono text-sm overflow-y-auto"
        :style="{ height: containerHeight }"
      >
        <div class="p-4 space-y-0.5">
          <div
            v-for="(line, index) in filteredLogs"
            :key="index"
            class="flex items-start hover:bg-gray-800 px-2 py-0.5 rounded"
          >
            <span
              class="w-20 flex-shrink-0 text-gray-500 select-none text-xs"
            >{{ formatTimestamp(line.timestamp) }}</span>
            <span
              :class="getLevelClass(line.level)"
              class="w-12 flex-shrink-0 uppercase text-xs font-semibold"
            >{{ line.level }}</span>
            <span
              v-if="line.category"
              class="w-20 flex-shrink-0 text-xs text-gray-400 truncate"
              :title="line.category"
            >{{ formatCategory(line.category) }}</span>
            <span class="flex-1 break-all whitespace-pre-wrap">{{ line.message }}</span>
          </div>
          <!-- Replay indicator -->
          <div
            v-if="replayedCount > 0 && displayedLogs.length > replayedCount"
            class="text-center text-gray-500 text-xs py-2 border-t border-gray-700 mt-2"
          >
            &#8593; {{ replayedCount }} buffered logs replayed &#8593;
          </div>
          <!-- Empty state for streaming -->
          <div
            v-if="isLiveStreaming && filteredLogs.length === 0"
            class="text-gray-500 italic"
          >
            {{ displayedLogs.length > 0 ? 'No logs match current filters' : 'Waiting for log output...' }}
          </div>
        </div>
      </div>
      <!-- Auto-scroll toggle -->
      <button
        v-if="filteredLogs.length > 0"
        class="absolute bottom-4 right-4 px-3 py-1.5 rounded-md text-xs font-medium transition-colors"
        :class="autoScroll ? 'bg-blue-600 text-white' : 'bg-gray-700 text-gray-300 hover:bg-gray-600'"
        @click="autoScroll = !autoScroll"
      >
        Auto-scroll {{ autoScroll ? 'ON' : 'OFF' }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { ArrowDownTrayIcon } from '@heroicons/vue/24/outline'
import { crawlerApi } from '../../api/client'
import { LoadingSpinner, ErrorAlert } from '../common'
import type { LogLine, LogSSEEvent, LogCategory, LogLevel } from '@/types/logs'
import { getCategoryShortName, shouldShowLevel } from '@/types/logs'

interface ExecutionLogInfo {
  execution_id: string
  execution_number: number
  status: string
  started_at: string
  completed_at?: string
  log_available: boolean
  log_object_key?: string
  log_size_bytes?: number
  log_line_count?: number
}

interface LogsMetadataResponse {
  job_id: string
  executions: ExecutionLogInfo[]
  has_live_logs: boolean
  limit: number
  offset: number
}

// Props
const props = defineProps<{
  jobId: string
  jobStatus: string
}>()

// State
const loading = ref(true)
const error = ref<string | null>(null)
const executions = ref<ExecutionLogInfo[]>([])
const hasLiveLogs = ref(false)
const selectedExecution = ref<number | null>(null)
const displayedLogs = ref<LogLine[]>([])
const downloading = ref(false)
const autoScroll = ref(true)
const logContainer = ref<HTMLElement | null>(null)
const replayedCount = ref(0)
const categoryFilter = ref<LogCategory | ''>('')
const levelFilter = ref<LogLevel | ''>('')

// SSE connection
let eventSource: EventSource | null = null

// Computed
const containerHeight = computed(() => '400px')

const isLiveStreaming = computed(() => {
  return hasLiveLogs.value && ['running', 'pending'].includes(props.jobStatus)
})

const canDownload = computed(() => {
  if (!selectedExecution.value) return false
  const exec = executions.value.find(e => e.execution_number === selectedExecution.value)
  return exec?.log_available === true
})

const filteredLogs = computed(() => {
  return displayedLogs.value.filter(line => {
    // Category filter
    if (categoryFilter.value && line.category !== categoryFilter.value) {
      return false
    }
    // Level filter (hierarchical - show selected level and above)
    if (levelFilter.value && line.level) {
      if (!shouldShowLevel(line.level as LogLevel, levelFilter.value)) {
        return false
      }
    }
    return true
  })
})

// Methods
const loadLogsMetadata = async () => {
  console.log('[JobLogsViewer] loadLogsMetadata called for jobId:', props.jobId)
  try {
    loading.value = true
    error.value = null
    console.log('[JobLogsViewer] Calling crawlerApi.jobs.logs...')
    const response = await crawlerApi.jobs.logs(props.jobId)
    console.log('[JobLogsViewer] logs response:', response.data)
    const data = response.data as LogsMetadataResponse
    executions.value = data.executions || []
    hasLiveLogs.value = data.has_live_logs || false

    // Select the latest execution by default
    if (executions.value.length > 0) {
      selectedExecution.value = executions.value[0].execution_number
    }

    // Start live streaming if available, otherwise load archived logs
    if (isLiveStreaming.value) {
      startLiveStream()
    } else if (selectedExecution.value !== null) {
      // Load archived logs for the selected execution
      await loadArchivedLogs(selectedExecution.value)
      return // Skip finally block, loadArchivedLogs handles loading state
    }
  } catch (err) {
    error.value = 'Unable to load logs metadata.'
    console.error('[JobLogsViewer] Error loading logs:', err)
  } finally {
    loading.value = false
  }
}

const startLiveStream = () => {
  if (eventSource) {
    eventSource.close()
  }

  const token = localStorage.getItem('dashboard_token')
  if (!token) {
    error.value = 'Authentication required for live logs.'
    return
  }

  // Reset state for new stream
  replayedCount.value = 0
  displayedLogs.value = []

  // Connect to SSE endpoint
  const url = `/api/crawler/jobs/${props.jobId}/logs/stream`
  eventSource = new EventSource(`${url}?token=${encodeURIComponent(token)}`)

  eventSource.onopen = () => {
    console.log('[JobLogsViewer] SSE connection opened')
  }

  eventSource.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data) as LogSSEEvent
      switch (data.type) {
        case 'log:replay':
          // Replay batch: prepend to displayed logs
          displayedLogs.value = [...data.data.lines, ...displayedLogs.value]
          replayedCount.value = data.data.count
          console.log(`[JobLogsViewer] Replayed ${data.data.count} buffered lines`)
          if (autoScroll.value) {
            scrollToBottom()
          }
          break
        case 'log:line':
          addLogLine(data.data)
          break
        case 'log:archived':
          // Logs archived, reload metadata
          loadLogsMetadata()
          break
        case 'connected':
          console.log('[JobLogsViewer] SSE connected:', data.data.message)
          break
      }
    } catch (err) {
      console.error('[JobLogsViewer] Error parsing SSE event:', err)
    }
  }

  eventSource.onerror = (err) => {
    console.error('[JobLogsViewer] SSE error:', err)
    // Reconnect after a delay if job is still running
    if (isLiveStreaming.value) {
      setTimeout(() => {
        if (isLiveStreaming.value) {
          startLiveStream()
        }
      }, 5000)
    }
  }
}

const stopLiveStream = () => {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
}

const loadArchivedLogs = async (executionNumber: number) => {
  // Check if this execution has logs available
  const exec = executions.value.find(e => e.execution_number === executionNumber)
  if (!exec?.log_available) {
    displayedLogs.value = []
    return
  }

  try {
    loading.value = true
    error.value = null
    const response = await crawlerApi.jobs.viewLogs(props.jobId, executionNumber)
    const data = response.data as { lines: LogLine[]; line_count: number }
    displayedLogs.value = data.lines || []
    if (autoScroll.value) {
      scrollToBottom()
    }
  } catch (err) {
    console.error('[JobLogsViewer] Error loading archived logs:', err)
    error.value = 'Failed to load archived logs.'
    displayedLogs.value = []
  } finally {
    loading.value = false
  }
}

const addLogLine = (line: LogLine) => {
  displayedLogs.value.push(line)
  // Limit buffer size
  const maxLines = 1000
  if (displayedLogs.value.length > maxLines) {
    displayedLogs.value = displayedLogs.value.slice(-maxLines)
  }
  // Auto-scroll
  if (autoScroll.value) {
    scrollToBottom()
  }
}

const scrollToBottom = () => {
  nextTick(() => {
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  })
}

const downloadLogs = async () => {
  if (!selectedExecution.value) return

  try {
    downloading.value = true
    const response = await crawlerApi.jobs.downloadLogs(props.jobId, selectedExecution.value)

    // Create download link
    const blob = new Blob([response.data], { type: 'application/gzip' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `job-${props.jobId}-exec-${selectedExecution.value}.log.gz`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)
  } catch (err) {
    console.error('[JobLogsViewer] Error downloading logs:', err)
    error.value = 'Failed to download logs.'
  } finally {
    downloading.value = false
  }
}

const formatTimestamp = (timestamp: string): string => {
  if (!timestamp) return ''
  try {
    const date = new Date(timestamp)
    return date.toLocaleTimeString('en-US', {
      hour12: false,
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
  } catch {
    return timestamp
  }
}

const formatStatus = (status: string): string => {
  return status.charAt(0).toUpperCase() + status.slice(1)
}

const formatCategory = (category: string): string => {
  return getCategoryShortName(category as LogCategory)
}

const getLevelClass = (level: string): string => {
  switch (level?.toLowerCase()) {
    case 'error':
      return 'text-red-400'
    case 'warn':
    case 'warning':
      return 'text-yellow-400'
    case 'info':
      return 'text-blue-400'
    case 'debug':
      return 'text-gray-400'
    default:
      return 'text-gray-300'
  }
}

// Watch for job status changes
watch(() => props.jobStatus, (newStatus) => {
  if (['running', 'pending'].includes(newStatus) && hasLiveLogs.value) {
    startLiveStream()
  } else {
    stopLiveStream()
    // Reload metadata when job completes
    if (['completed', 'failed', 'cancelled'].includes(newStatus)) {
      loadLogsMetadata()
    }
  }
})

// Watch for execution selection changes
watch(selectedExecution, async (newExec) => {
  if (!isLiveStreaming.value && newExec !== null) {
    // Fetch archived logs for the selected execution
    await loadArchivedLogs(newExec)
  }
})

// Lifecycle
onMounted(() => {
  console.log('[JobLogsViewer] Component mounted, jobId:', props.jobId, 'jobStatus:', props.jobStatus)
  loadLogsMetadata()
})

onUnmounted(() => {
  stopLiveStream()
})
</script>
