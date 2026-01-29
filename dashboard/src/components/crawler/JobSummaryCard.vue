<template>
  <div class="p-6 bg-gray-50">
    <h3 class="text-sm font-medium text-gray-900 mb-4">Job Summary</h3>

    <!-- Core Metrics Grid -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
      <div class="bg-white rounded-lg p-3 shadow-sm">
        <div class="text-2xl font-semibold text-gray-900">
          {{ summary.pages_discovered }}
        </div>
        <div class="text-xs text-gray-500">Pages Discovered</div>
      </div>

      <div class="bg-white rounded-lg p-3 shadow-sm">
        <div class="text-2xl font-semibold text-gray-900">
          {{ summary.pages_crawled }}
        </div>
        <div class="text-xs text-gray-500">Pages Crawled</div>
      </div>

      <div class="bg-white rounded-lg p-3 shadow-sm">
        <div class="text-2xl font-semibold text-gray-900">
          {{ summary.items_extracted }}
        </div>
        <div class="text-xs text-gray-500">Items Extracted</div>
      </div>

      <div class="bg-white rounded-lg p-3 shadow-sm">
        <div
          class="text-2xl font-semibold"
          :class="summary.errors_count > 0 ? 'text-red-600' : 'text-gray-900'"
        >
          {{ summary.errors_count }}
        </div>
        <div class="text-xs text-gray-500">Errors</div>
      </div>
    </div>

    <!-- Duration and Network Stats -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mt-4">
      <div class="bg-white rounded-lg p-3 shadow-sm">
        <div class="text-lg font-semibold text-gray-900">
          {{ formatDuration(summary.duration_ms) }}
        </div>
        <div class="text-xs text-gray-500">Total Duration</div>
      </div>

      <div
        v-if="summary.bytes_fetched"
        class="bg-white rounded-lg p-3 shadow-sm"
      >
        <div class="text-lg font-semibold text-gray-900">
          {{ formatBytes(summary.bytes_fetched) }}
        </div>
        <div class="text-xs text-gray-500">Data Fetched</div>
      </div>

      <div
        v-if="summary.requests_total"
        class="bg-white rounded-lg p-3 shadow-sm"
      >
        <div class="text-lg font-semibold text-gray-900">
          {{ summary.requests_total }}
        </div>
        <div class="text-xs text-gray-500">Total Requests</div>
      </div>

      <div
        v-if="summary.requests_failed"
        class="bg-white rounded-lg p-3 shadow-sm"
      >
        <div
          class="text-lg font-semibold"
          :class="summary.requests_failed > 0 ? 'text-yellow-600' : 'text-gray-900'"
        >
          {{ summary.requests_failed }}
        </div>
        <div class="text-xs text-gray-500">Failed Requests</div>
      </div>
    </div>

    <!-- Status Codes -->
    <div
      v-if="hasStatusCodes"
      class="mt-4"
    >
      <h4 class="text-xs font-medium text-gray-500 mb-2">Status Codes</h4>
      <div class="flex flex-wrap gap-2">
        <span
          v-for="(count, code) in summary.status_codes"
          :key="code"
          :class="getStatusCodeClass(Number(code))"
          class="px-2 py-1 rounded text-xs font-medium"
        >
          {{ code }}: {{ count }}
        </span>
      </div>
    </div>

    <!-- Top Errors -->
    <div
      v-if="summary.top_errors && summary.top_errors.length > 0"
      class="mt-4"
    >
      <h4 class="text-xs font-medium text-gray-500 mb-2">Top Errors</h4>
      <ul class="space-y-1">
        <li
          v-for="(error, index) in summary.top_errors"
          :key="index"
          class="text-sm text-red-600 bg-red-50 px-2 py-1 rounded"
        >
          {{ error.message }}
          <span class="text-red-400">(x{{ error.count }})</span>
        </li>
      </ul>
    </div>

    <!-- Throttle Warning -->
    <div
      v-if="summary.logs_throttled && summary.logs_throttled > 0"
      class="mt-4 p-3 bg-yellow-50 rounded-md flex items-start"
    >
      <ExclamationTriangleIcon class="w-5 h-5 text-yellow-600 mr-2 flex-shrink-0" />
      <div>
        <p class="text-sm text-yellow-800 font-medium">
          Logs Throttled
        </p>
        <p class="text-xs text-yellow-700">
          {{ summary.logs_throttled.toLocaleString() }} logs were throttled
          <span v-if="summary.throttle_percent">
            ({{ summary.throttle_percent.toFixed(1) }}%)
          </span>
        </p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ExclamationTriangleIcon } from '@heroicons/vue/24/outline'
import type { JobSummary } from '@/types/logs'

const props = defineProps<{
  summary: JobSummary
}>()

const hasStatusCodes = computed(() => {
  return props.summary.status_codes && Object.keys(props.summary.status_codes).length > 0
})

const formatDuration = (ms: number): string => {
  if (!ms || ms < 0) return '0ms'
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60000).toFixed(1)}m`
}

const formatBytes = (bytes: number): string => {
  if (!bytes || bytes < 0) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

const getStatusCodeClass = (code: number): string => {
  if (code >= 200 && code < 300) return 'bg-green-100 text-green-800'
  if (code >= 300 && code < 400) return 'bg-blue-100 text-blue-800'
  if (code >= 400 && code < 500) return 'bg-yellow-100 text-yellow-800'
  return 'bg-red-100 text-red-800'
}
</script>
