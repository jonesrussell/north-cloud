<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import StatusBadge from '@/shared/components/StatusBadge.vue'
import ErrorBanner from '@/shared/components/ErrorBanner.vue'
import LoadingSkeleton from '@/shared/components/LoadingSkeleton.vue'
import GrafanaEmbed from '@/shared/components/GrafanaEmbed.vue'
import { useCrawlJob, useControlJob } from '../composables/useCrawlApi'
import { formatDate, formatInterval } from '../utils'
import { useToast } from '@/shared/composables/useToast'

const route = useRoute()
const router = useRouter()
const toast = useToast()

const jobId = computed(() => route.params.id as string)
const confirmAction = ref<'cancel' | null>(null)

const { data: job, isLoading, isError, error, refetch } = useCrawlJob(jobId)
const { mutate: controlJob, isPending: isControlPending } = useControlJob()

const canPause = computed(() => job.value?.status === 'scheduled')
const canResume = computed(() => job.value?.status === 'paused')
const canCancel = computed(() =>
  ['scheduled', 'running', 'paused', 'pending'].includes(job.value?.status ?? ''),
)
const canRetry = computed(() =>
  ['failed', 'completed', 'cancelled'].includes(job.value?.status ?? ''),
)

function handleAction(action: 'pause' | 'resume' | 'cancel' | 'retry') {
  if (action === 'cancel') {
    confirmAction.value = 'cancel'
    return
  }
  executeAction(action)
}

function executeAction(action: 'pause' | 'resume' | 'cancel' | 'retry') {
  confirmAction.value = null
  controlJob(
    { id: jobId.value, action },
    {
      onSuccess: () => {
        toast.success(`Job ${action} successful`)
      },
      onError: (err) => {
        toast.error(`Failed to ${action} job: ${err.message}`)
      },
    },
  )
}
</script>

<template>
  <div>
    <button
      @click="router.push({ name: 'crawl-jobs' })"
      class="text-sm text-slate-400 hover:text-slate-200 mb-4 inline-block"
    >
      &larr; Back to Jobs
    </button>

    <LoadingSkeleton v-if="isLoading" :lines="10" />

    <ErrorBanner
      v-else-if="isError"
      :message="error?.message ?? 'Failed to load job'"
      @retry="refetch"
    />

    <template v-else-if="job">
      <div class="flex items-center justify-between mb-6">
        <div>
          <h1 class="text-2xl font-bold text-slate-100">
            {{ job.source_name ?? job.source_id }}
          </h1>
          <p class="text-sm text-slate-400 mt-1">{{ job.id }}</p>
        </div>
        <StatusBadge :status="job.status" />
      </div>

      <!-- Action buttons -->
      <div class="flex gap-2 mb-6">
        <button
          v-if="canPause"
          :disabled="isControlPending"
          @click="handleAction('pause')"
          class="px-3 py-1.5 text-sm font-medium text-amber-400 border border-amber-700 rounded hover:bg-amber-900/30 disabled:opacity-50"
        >
          Pause
        </button>
        <button
          v-if="canResume"
          :disabled="isControlPending"
          @click="handleAction('resume')"
          class="px-3 py-1.5 text-sm font-medium text-green-400 border border-green-700 rounded hover:bg-green-900/30 disabled:opacity-50"
        >
          Resume
        </button>
        <button
          v-if="canRetry"
          :disabled="isControlPending"
          @click="handleAction('retry')"
          class="px-3 py-1.5 text-sm font-medium text-blue-400 border border-blue-700 rounded hover:bg-blue-900/30 disabled:opacity-50"
        >
          Retry
        </button>
        <button
          v-if="canCancel"
          :disabled="isControlPending"
          @click="handleAction('cancel')"
          class="px-3 py-1.5 text-sm font-medium text-red-400 border border-red-700 rounded hover:bg-red-900/30 disabled:opacity-50"
        >
          Cancel
        </button>
      </div>

      <!-- Cancel confirmation -->
      <div
        v-if="confirmAction === 'cancel'"
        class="bg-red-900/20 border border-red-800 rounded-lg p-4 mb-6 flex items-center justify-between"
      >
        <p class="text-red-300 text-sm">Are you sure you want to cancel this job?</p>
        <div class="flex gap-2">
          <button
            @click="confirmAction = null"
            class="px-3 py-1 text-sm text-slate-300 border border-slate-600 rounded hover:border-slate-500"
          >
            No
          </button>
          <button
            @click="executeAction('cancel')"
            class="px-3 py-1 text-sm text-red-400 border border-red-700 rounded hover:bg-red-900/30"
          >
            Yes, Cancel
          </button>
        </div>
      </div>

      <!-- Job details grid -->
      <div class="grid grid-cols-1 md:grid-cols-2 gap-6 mb-6">
        <div class="bg-slate-900 border border-slate-800 rounded-lg p-4">
          <h2 class="text-sm font-medium text-slate-400 uppercase mb-3">Job Info</h2>
          <dl class="space-y-2 text-sm">
            <div class="flex justify-between">
              <dt class="text-slate-400">Type</dt>
              <dd class="text-slate-200">{{ job.type }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-slate-400">URL</dt>
              <dd class="text-slate-200 truncate ml-4">{{ job.url }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-slate-400">Interval</dt>
              <dd class="text-slate-200">{{ formatInterval(job.interval_minutes) }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-slate-400">Adaptive</dt>
              <dd class="text-slate-200">{{ job.adaptive_scheduling ? 'Yes' : 'No' }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-slate-400">Auto-managed</dt>
              <dd class="text-slate-200">{{ job.auto_managed ? 'Yes' : 'No' }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-slate-400">Priority</dt>
              <dd class="text-slate-200">{{ job.priority }}</dd>
            </div>
          </dl>
        </div>

        <div class="bg-slate-900 border border-slate-800 rounded-lg p-4">
          <h2 class="text-sm font-medium text-slate-400 uppercase mb-3">Timestamps</h2>
          <dl class="space-y-2 text-sm">
            <div class="flex justify-between">
              <dt class="text-slate-400">Created</dt>
              <dd class="text-slate-200">{{ formatDate(job.created_at) }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-slate-400">Started</dt>
              <dd class="text-slate-200">{{ formatDate(job.started_at) }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-slate-400">Completed</dt>
              <dd class="text-slate-200">{{ formatDate(job.completed_at) }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-slate-400">Next Run</dt>
              <dd class="text-slate-200">{{ formatDate(job.next_run_at) }}</dd>
            </div>
          </dl>
        </div>

        <div class="bg-slate-900 border border-slate-800 rounded-lg p-4">
          <h2 class="text-sm font-medium text-slate-400 uppercase mb-3">Retry Config</h2>
          <dl class="space-y-2 text-sm">
            <div class="flex justify-between">
              <dt class="text-slate-400">Max Retries</dt>
              <dd class="text-slate-200">{{ job.max_retries }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-slate-400">Backoff (seconds)</dt>
              <dd class="text-slate-200">{{ job.retry_backoff_seconds }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-slate-400">Current Retry</dt>
              <dd class="text-slate-200">{{ job.current_retry_count }}</dd>
            </div>
            <div class="flex justify-between">
              <dt class="text-slate-400">Failures</dt>
              <dd class="text-slate-200">{{ job.failure_count }}</dd>
            </div>
          </dl>
        </div>

        <div v-if="job.error_message" class="bg-slate-900 border border-red-800/50 rounded-lg p-4">
          <h2 class="text-sm font-medium text-red-400 uppercase mb-3">Error</h2>
          <p class="text-sm text-red-300 font-mono whitespace-pre-wrap">{{ job.error_message }}</p>
        </div>
      </div>

      <!-- Grafana embed for crawler throughput -->
      <div class="mb-6">
        <h2 class="text-sm font-medium text-slate-400 uppercase mb-3">Crawler Throughput</h2>
        <GrafanaEmbed
          panel-id="crawler-throughput"
          :vars="{ source_id: job.source_id }"
          height="300px"
        />
      </div>
    </template>
  </div>
</template>
