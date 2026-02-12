<script setup lang="ts">
/**
 * Job Detail View (Refactored)
 *
 * Uses the new feature module architecture:
 * - TanStack Query for server state (job, executions, stats)
 * - useJobDetail composable for all data and actions
 */
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Pause, Play, PlayCircle, XCircle, RotateCcw, Loader2, ChevronDown, ChevronRight } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import JobLogsViewer from '@/components/crawler/JobLogsViewer.vue'
import CrawlMetricsPanel from '@/components/crawler/CrawlMetricsPanel.vue'

import { formatDate } from '@/lib/utils'
import { useJobDetail } from '@/features/intake'
import type { Job, JobStatus, JobExecution } from '@/types/crawler'

const route = useRoute()
const router = useRouter()

const jobId = computed((): string | null => {
  const id = route.params.id
  if (!id || id === 'undefined') {
    return null
  }
  return String(id)
})

// Redirect if no valid ID
watch(jobId, (id) => {
  if (!id) {
    router.replace('/intake/jobs')
  }
}, { immediate: true })

// Use the new composable
const detail = useJobDetail(jobId.value || '')

// Typed accessors
const job = computed(() => detail.job.value as Job | undefined)
const stats = computed(() => detail.stats.value as Record<string, unknown> | undefined)
const executions = computed(() => detail.executions.value as JobExecution[])

// Expandable execution rows
const expandedExecId = ref<string | null>(null)

function toggleExecRow(execId: string) {
  expandedExecId.value = expandedExecId.value === execId ? null : execId
}

type BadgeVariant = 'default' | 'secondary' | 'destructive' | 'outline' | 'success' | 'warning' | 'pending'

const statusVariants: Record<JobStatus, BadgeVariant> = {
  running: 'default',
  scheduled: 'secondary',
  pending: 'pending',
  completed: 'success',
  failed: 'destructive',
  paused: 'warning',
  cancelled: 'outline',
}

const getStatusVariant = (status: string): BadgeVariant => {
  return statusVariants[status as JobStatus] || 'secondary'
}

const formatDuration = (ms: number | undefined | null): string => {
  if (!ms && ms !== 0) return 'N/A'
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  const minutes = Math.floor(ms / 60000)
  const seconds = Math.floor((ms % 60000) / 1000)
  return `${minutes}m ${seconds}s`
}

const canPause = computed(() => {
  if (!job.value) return false
  return ['pending', 'scheduled'].includes(job.value.status) && !job.value.is_paused
})

const canResume = computed(() => {
  if (!job.value) return false
  return job.value.is_paused || job.value.status === 'paused'
})

const canCancel = computed(() => {
  if (!job.value) return false
  return ['pending', 'scheduled', 'running'].includes(job.value.status)
})

const canRetry = computed(() => {
  if (!job.value) return false
  return ['completed', 'failed', 'cancelled'].includes(job.value.status)
})

const canRunNow = computed(() => {
  if (!job.value) return false
  return ['scheduled', 'paused', 'pending'].includes(job.value.status)
})

async function handlePause() {
  await detail.pauseJob()
}

async function handleResume() {
  await detail.resumeJob()
}

async function handleCancel() {
  await detail.cancelJob()
}

async function handleRetry() {
  await detail.retryJob()
}

async function handleRunNow() {
  await detail.forceRunJob()
}

function goBack() {
  router.push('/intake/jobs')
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-4">
        <Button
          variant="ghost"
          size="icon"
          @click="goBack"
        >
          <ArrowLeft class="h-5 w-5" />
        </Button>
        <div>
          <h1 class="text-3xl font-bold tracking-tight">
            {{ job?.source_name || 'Job Details' }}
          </h1>
          <p class="text-muted-foreground">
            Job ID: {{ jobId || 'N/A' }}
          </p>
        </div>
      </div>
      <div class="flex gap-2">
        <Button
          v-if="canPause"
          variant="outline"
          :disabled="detail.isPausing.value"
          @click="handlePause"
        >
          <Loader2
            v-if="detail.isPausing.value"
            class="mr-2 h-4 w-4 animate-spin"
          />
          <Pause
            v-else
            class="mr-2 h-4 w-4"
          />
          Pause
        </Button>
        <Button
          v-if="canResume"
          variant="outline"
          :disabled="detail.isResuming.value"
          @click="handleResume"
        >
          <Loader2
            v-if="detail.isResuming.value"
            class="mr-2 h-4 w-4 animate-spin"
          />
          <Play
            v-else
            class="mr-2 h-4 w-4"
          />
          Resume
        </Button>
        <Button
          v-if="canRunNow"
          variant="outline"
          :disabled="detail.isForceRunning.value"
          @click="handleRunNow"
        >
          <Loader2
            v-if="detail.isForceRunning.value"
            class="mr-2 h-4 w-4 animate-spin"
          />
          <PlayCircle
            v-else
            class="mr-2 h-4 w-4"
          />
          Run now
        </Button>
        <Button
          v-if="canRetry"
          variant="outline"
          :disabled="detail.isRetrying.value"
          @click="handleRetry"
        >
          <Loader2
            v-if="detail.isRetrying.value"
            class="mr-2 h-4 w-4 animate-spin"
          />
          <RotateCcw
            v-else
            class="mr-2 h-4 w-4"
          />
          Retry
        </Button>
        <Button
          v-if="canCancel"
          variant="destructive"
          :disabled="detail.isCancelling.value"
          @click="handleCancel"
        >
          <Loader2
            v-if="detail.isCancelling.value"
            class="mr-2 h-4 w-4 animate-spin"
          />
          <XCircle
            v-else
            class="mr-2 h-4 w-4"
          />
          Cancel
        </Button>
      </div>
    </div>

    <!-- Loading -->
    <div
      v-if="detail.isLoadingJob.value"
      class="flex items-center justify-center py-12"
    >
      <Loader2 class="h-8 w-8 animate-spin text-muted-foreground" />
    </div>

    <!-- Error -->
    <Card
      v-else-if="detail.jobError.value"
      class="border-destructive"
    >
      <CardContent class="pt-6">
        <p class="text-destructive">
          {{ detail.jobError.value?.message || 'Unable to load job details.' }}
        </p>
      </CardContent>
    </Card>

    <!-- Job Details -->
    <template v-else-if="job">
      <!-- Info Card -->
      <Card>
        <CardHeader>
          <CardTitle>Job Information</CardTitle>
        </CardHeader>
        <CardContent>
          <dl class="grid grid-cols-2 gap-4">
            <div>
              <dt class="text-sm text-muted-foreground">
                Status
              </dt>
              <dd class="mt-1">
                <Badge :variant="getStatusVariant(job.status)">
                  {{ job.status }}
                </Badge>
              </dd>
            </div>
            <div>
              <dt class="text-sm text-muted-foreground">
                Source
              </dt>
              <dd class="mt-1">
                {{ job.source_name || 'N/A' }}
              </dd>
            </div>
            <div class="col-span-2">
              <dt class="text-sm text-muted-foreground">
                URL
              </dt>
              <dd class="mt-1">
                <a
                  :href="job.url"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="text-primary hover:underline break-all"
                >
                  {{ job.url }}
                </a>
              </dd>
            </div>
            <div>
              <dt class="text-sm text-muted-foreground">
                Created
              </dt>
              <dd class="mt-1">
                {{ formatDate(job.created_at) }}
              </dd>
            </div>
            <div v-if="job.schedule_enabled">
              <dt class="text-sm text-muted-foreground">
                Schedule
              </dt>
              <dd class="mt-1">
                Every {{ job.interval_minutes }} {{ job.interval_type }}
              </dd>
            </div>
            <div v-if="job.next_run_at">
              <dt class="text-sm text-muted-foreground">
                Next Run
              </dt>
              <dd class="mt-1">
                {{ formatDate(job.next_run_at) }}
              </dd>
            </div>
            <div v-if="job.last_run_at">
              <dt class="text-sm text-muted-foreground">
                Last Run
              </dt>
              <dd class="mt-1">
                {{ formatDate(job.last_run_at) }}
              </dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      <!-- Statistics -->
      <Card v-if="stats && !detail.isLoadingStats.value">
        <CardHeader>
          <CardTitle>Statistics</CardTitle>
        </CardHeader>
        <CardContent>
          <div class="grid grid-cols-3 gap-4">
            <div>
              <p class="text-sm text-muted-foreground">
                Total Executions
              </p>
              <p class="text-2xl font-bold">
                {{ stats.total_executions || 0 }}
              </p>
            </div>
            <div>
              <p class="text-sm text-muted-foreground">
                Successful
              </p>
              <p class="text-2xl font-bold text-green-600">
                {{ stats.successful_runs || 0 }}
              </p>
            </div>
            <div>
              <p class="text-sm text-muted-foreground">
                Failed
              </p>
              <p class="text-2xl font-bold text-red-600">
                {{ stats.failed_runs || 0 }}
              </p>
            </div>
          </div>
        </CardContent>
      </Card>

      <!-- Job Logs -->
      <JobLogsViewer
        v-if="jobId"
        :job-id="jobId"
        :job-status="job.status"
      />

      <!-- Executions -->
      <Card>
        <CardHeader>
          <CardTitle>Execution History</CardTitle>
        </CardHeader>
        <CardContent class="p-0">
          <div
            v-if="detail.isLoadingExecutions.value"
            class="flex justify-center py-8"
          >
            <Loader2 class="h-6 w-6 animate-spin" />
          </div>
          <div
            v-else-if="executions.length === 0"
            class="py-8 text-center text-muted-foreground"
          >
            No executions yet
          </div>
          <table
            v-else
            class="w-full"
          >
            <thead class="border-b bg-muted/50">
              <tr>
                <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                  #
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                  Status
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                  Started
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                  Duration
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase">
                  Items
                </th>
              </tr>
            </thead>
            <tbody class="divide-y">
              <template
                v-for="exec in executions"
                :key="exec.id"
              >
                <tr
                  class="hover:bg-muted/50 cursor-pointer"
                  @click="toggleExecRow(exec.id)"
                >
                  <td class="px-6 py-4 text-sm font-medium">
                    <span class="flex items-center gap-1.5">
                      <ChevronDown
                        v-if="expandedExecId === exec.id"
                        class="h-4 w-4 text-muted-foreground"
                      />
                      <ChevronRight
                        v-else
                        class="h-4 w-4 text-muted-foreground"
                      />
                      #{{ exec.execution_number }}
                    </span>
                  </td>
                  <td class="px-6 py-4">
                    <Badge :variant="getStatusVariant(exec.status)">
                      {{ exec.status }}
                    </Badge>
                  </td>
                  <td class="px-6 py-4 text-sm text-muted-foreground">
                    {{ formatDate(exec.started_at) }}
                  </td>
                  <td class="px-6 py-4 text-sm text-muted-foreground">
                    {{ formatDuration(exec.duration_ms) }}
                  </td>
                  <td class="px-6 py-4 text-sm text-muted-foreground">
                    {{ exec.items_indexed || 0 }} indexed
                  </td>
                </tr>
                <tr v-if="expandedExecId === exec.id">
                  <td
                    colspan="5"
                    class="px-6 py-4 bg-muted/30"
                  >
                    <CrawlMetricsPanel
                      v-if="exec.metadata?.crawl_metrics"
                      :metrics="exec.metadata.crawl_metrics"
                    />
                    <p
                      v-else
                      class="text-sm text-muted-foreground"
                    >
                      No crawl metrics available for this execution.
                    </p>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </CardContent>
      </Card>
    </template>
  </div>
</template>
