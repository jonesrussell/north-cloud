<script setup lang="ts">
import { computed } from 'vue'
import { AlertTriangle } from 'lucide-vue-next'
import type { CrawlMetrics } from '@/types/crawler'

const props = defineProps<{
  metrics: CrawlMetrics
}>()

const hasStatusCodes = computed(() => {
  return props.metrics.status_codes && Object.keys(props.metrics.status_codes).length > 0
})

const hasErrorCategories = computed(() => {
  return props.metrics.error_categories && Object.keys(props.metrics.error_categories).length > 0
})

const hasSkipped = computed(() => {
  return props.metrics.skipped && Object.keys(props.metrics.skipped).length > 0
})

const hasAlerts = computed(() => {
  return (props.metrics.cloudflare_blocks && props.metrics.cloudflare_blocks > 0) ||
    (props.metrics.rate_limits && props.metrics.rate_limits > 0)
})

function formatBytes(bytes: number): string {
  if (!bytes || bytes < 0) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function formatMs(ms: number): string {
  return `${ms.toFixed(1)}ms`
}

function getStatusCodeClass(code: number): string {
  if (code >= 200 && code < 300) return 'bg-green-100 text-green-800'
  if (code >= 300 && code < 400) return 'bg-blue-100 text-blue-800'
  if (code >= 400 && code < 500) return 'bg-yellow-100 text-yellow-800'
  return 'bg-red-100 text-red-800'
}

function formatSkipKey(key: string): string {
  return key.replace(/_/g, ' ')
}
</script>

<template>
  <div class="space-y-4">
    <!-- Network Stats -->
    <div>
      <h4 class="text-xs font-medium text-muted-foreground mb-2">
        Network
      </h4>
      <div class="grid grid-cols-2 md:grid-cols-4 gap-3">
        <div class="bg-background rounded-lg p-3 border">
          <div class="text-lg font-semibold">
            {{ metrics.requests_total }}
          </div>
          <div class="text-xs text-muted-foreground">
            Requests
          </div>
        </div>
        <div class="bg-background rounded-lg p-3 border">
          <div
            class="text-lg font-semibold"
            :class="metrics.requests_failed > 0 ? 'text-yellow-600' : ''"
          >
            {{ metrics.requests_failed }}
          </div>
          <div class="text-xs text-muted-foreground">
            Failed
          </div>
        </div>
        <div class="bg-background rounded-lg p-3 border">
          <div class="text-lg font-semibold">
            {{ formatBytes(metrics.bytes_downloaded) }}
          </div>
          <div class="text-xs text-muted-foreground">
            Downloaded
          </div>
        </div>
        <div
          v-if="metrics.response_time"
          class="bg-background rounded-lg p-3 border"
        >
          <div class="text-lg font-semibold">
            {{ formatMs(metrics.response_time.avg_ms) }}
          </div>
          <div class="text-xs text-muted-foreground">
            Avg Response
          </div>
        </div>
      </div>
    </div>

    <!-- Response Time Detail -->
    <div v-if="metrics.response_time">
      <h4 class="text-xs font-medium text-muted-foreground mb-2">
        Response Time
      </h4>
      <div class="grid grid-cols-3 gap-3">
        <div class="bg-background rounded-lg p-3 border">
          <div class="text-lg font-semibold">
            {{ formatMs(metrics.response_time.min_ms) }}
          </div>
          <div class="text-xs text-muted-foreground">
            Min
          </div>
        </div>
        <div class="bg-background rounded-lg p-3 border">
          <div class="text-lg font-semibold">
            {{ formatMs(metrics.response_time.avg_ms) }}
          </div>
          <div class="text-xs text-muted-foreground">
            Avg
          </div>
        </div>
        <div class="bg-background rounded-lg p-3 border">
          <div class="text-lg font-semibold">
            {{ formatMs(metrics.response_time.max_ms) }}
          </div>
          <div class="text-xs text-muted-foreground">
            Max
          </div>
        </div>
      </div>
    </div>

    <!-- Alerts -->
    <div
      v-if="hasAlerts"
      class="flex flex-wrap gap-2"
    >
      <div
        v-if="metrics.cloudflare_blocks && metrics.cloudflare_blocks > 0"
        class="flex items-center gap-1.5 bg-orange-50 text-orange-800 px-3 py-1.5 rounded-md text-sm"
      >
        <AlertTriangle class="h-4 w-4" />
        {{ metrics.cloudflare_blocks }} Cloudflare blocks
      </div>
      <div
        v-if="metrics.rate_limits && metrics.rate_limits > 0"
        class="flex items-center gap-1.5 bg-yellow-50 text-yellow-800 px-3 py-1.5 rounded-md text-sm"
      >
        <AlertTriangle class="h-4 w-4" />
        {{ metrics.rate_limits }} rate limits
      </div>
    </div>

    <!-- Status Codes -->
    <div v-if="hasStatusCodes">
      <h4 class="text-xs font-medium text-muted-foreground mb-2">
        Status Codes
      </h4>
      <div class="flex flex-wrap gap-2">
        <span
          v-for="(count, code) in metrics.status_codes"
          :key="code"
          :class="getStatusCodeClass(Number(code))"
          class="px-2 py-1 rounded text-xs font-medium"
        >
          {{ code }}: {{ count }}
        </span>
      </div>
    </div>

    <!-- Error Categories -->
    <div v-if="hasErrorCategories">
      <h4 class="text-xs font-medium text-muted-foreground mb-2">
        Error Categories
      </h4>
      <div class="flex flex-wrap gap-2">
        <span
          v-for="(count, category) in metrics.error_categories"
          :key="category"
          class="bg-red-50 text-red-700 px-2 py-1 rounded text-xs font-medium"
        >
          {{ category }}: {{ count }}
        </span>
      </div>
    </div>

    <!-- Skip Reasons -->
    <div v-if="hasSkipped">
      <h4 class="text-xs font-medium text-muted-foreground mb-2">
        Skipped
      </h4>
      <div class="flex flex-wrap gap-2">
        <span
          v-for="(count, reason) in metrics.skipped"
          :key="reason"
          class="bg-muted text-muted-foreground px-2 py-1 rounded text-xs font-medium"
        >
          {{ formatSkipKey(String(reason)) }}: {{ count }}
        </span>
      </div>
    </div>

    <!-- Top Errors -->
    <div v-if="metrics.top_errors && metrics.top_errors.length > 0">
      <h4 class="text-xs font-medium text-muted-foreground mb-2">
        Top Errors
      </h4>
      <ul class="space-y-1">
        <li
          v-for="(error, index) in metrics.top_errors"
          :key="index"
          class="text-sm text-red-600 bg-red-50 px-2 py-1 rounded"
        >
          {{ error.message }}
          <span class="text-red-400">(x{{ error.count }})</span>
        </li>
      </ul>
    </div>
  </div>
</template>
