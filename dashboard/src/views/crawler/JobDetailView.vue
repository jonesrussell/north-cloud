<template>
  <div>
    <PageHeader
      :title="job?.source_name || 'Job Details'"
      :subtitle="job ? `Job ID: ${truncateId(job.id)}` : 'Loading...'"
    >
      <template #actions>
        <button
          class="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50 transition-colors flex items-center mr-3"
          @click="goBack"
        >
          <ArrowLeftIcon class="h-5 w-5 mr-2" />
          Back
        </button>
        <button
          v-if="job && canPause"
          class="px-4 py-2 bg-yellow-600 text-white rounded-md hover:bg-yellow-700 transition-colors"
          @click="pauseJob"
        >
          Pause
        </button>
        <button
          v-if="job && canResume"
          class="px-4 py-2 bg-green-600 text-white rounded-md hover:bg-green-700 transition-colors"
          @click="resumeJob"
        >
          Resume
        </button>
        <button
          v-if="job && canRetry"
          class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors ml-2"
          :disabled="retrying"
          @click="retryJob"
        >
          {{ retrying ? 'Retrying...' : 'Retry' }}
        </button>
        <button
          v-if="job && canCancel"
          class="px-4 py-2 bg-red-600 text-white rounded-md hover:bg-red-700 transition-colors ml-2"
          @click="confirmCancel"
        >
          Cancel
        </button>
      </template>
    </PageHeader>

    <!-- Loading State -->
    <LoadingSpinner
      v-if="loading"
      size="lg"
      text="Loading job details..."
      :full-page="true"
    />

    <!-- Error State -->
    <ErrorAlert
      v-else-if="error"
      :message="error"
      class="mb-6"
    />

    <!-- Job Details -->
    <div
      v-else-if="job"
      class="space-y-6"
    >
      <!-- Job Info Card -->
      <div class="bg-white shadow rounded-lg p-6">
        <h2 class="text-lg font-medium text-gray-900 mb-4">
          Job Information
        </h2>
        <dl class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Job ID
            </dt>
            <dd class="mt-1 text-sm text-gray-900 font-mono">
              {{ job.id }}
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Status
            </dt>
            <dd class="mt-1">
              <StatusBadge :status="job.status" />
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Source
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ job.source_name || 'N/A' }}
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              URL
            </dt>
            <dd class="mt-1 text-sm">
              <a
                :href="job.url"
                target="_blank"
                rel="noopener noreferrer"
                class="text-blue-600 hover:text-blue-800 break-all"
              >
                {{ job.url }}
              </a>
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Created
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ formatDate(job.created_at) }}
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Next Run
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ formatNextRun(job) }}
            </dd>
          </div>
          <div
            v-if="job.schedule_enabled"
          >
            <dt class="text-sm font-medium text-gray-500">
              Schedule
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              Every {{ job.interval_minutes }} {{ job.interval_type }}
            </dd>
          </div>
          <div
            v-if="job.is_paused"
          >
            <dt class="text-sm font-medium text-gray-500">
              Paused At
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ formatDate(job.paused_at) }}
            </dd>
          </div>
        </dl>
      </div>

      <!-- Statistics Card -->
      <div
        v-if="stats"
        class="bg-white shadow rounded-lg p-6"
      >
        <h2 class="text-lg font-medium text-gray-900 mb-4">
          Statistics
        </h2>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Total Executions
            </dt>
            <dd class="mt-1 text-2xl font-semibold text-gray-900">
              {{ stats.total_executions || 0 }}
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Successful Runs
            </dt>
            <dd class="mt-1 text-2xl font-semibold text-green-600">
              {{ stats.successful_runs || 0 }}
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Failed Runs
            </dt>
            <dd class="mt-1 text-2xl font-semibold text-red-600">
              {{ stats.failed_runs || 0 }}
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Success Rate
            </dt>
            <dd class="mt-1 text-2xl font-semibold text-gray-900">
              {{ successRate }}%
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Average Duration
            </dt>
            <dd class="mt-1 text-2xl font-semibold text-gray-900">
              {{ formatDuration(stats.average_duration_ms) }}
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Total Items Crawled
            </dt>
            <dd class="mt-1 text-2xl font-semibold text-gray-900">
              {{ stats.total_items_crawled || 0 }}
            </dd>
          </div>
        </div>
      </div>

      <!-- Job Logs -->
      <JobLogsViewer
        :job-id="jobId"
        :job-status="job.status"
      />

      <!-- Execution History -->
      <div class="bg-white shadow rounded-lg overflow-hidden">
        <div class="px-6 py-4 border-b border-gray-200">
          <h2 class="text-lg font-medium text-gray-900">
            Execution History
          </h2>
        </div>
        <div
          v-if="loadingExecutions"
          class="p-8 text-center"
        >
          <LoadingSpinner
            size="md"
            text="Loading executions..."
          />
        </div>
        <div
          v-else-if="executionsError"
          class="p-6"
        >
          <ErrorAlert :message="executionsError" />
        </div>
        <div
          v-else-if="executions.length === 0"
          class="p-8 text-center"
        >
          <p class="text-sm text-gray-500">
            No executions yet
          </p>
        </div>
        <table
          v-else
          class="min-w-full divide-y divide-gray-200"
        >
          <thead class="bg-gray-50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Execution #
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Status
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Started
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Duration
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Items Crawled
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Items Indexed
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Retry
              </th>
            </tr>
          </thead>
          <tbody class="bg-white divide-y divide-gray-200">
            <tr
              v-for="execution in executions"
              :key="execution.id"
              class="hover:bg-gray-50 cursor-pointer"
              @click="viewExecution(execution.id)"
            >
              <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                #{{ execution.execution_number }}
              </td>
              <td class="px-6 py-4 whitespace-nowrap">
                <StatusBadge :status="execution.status" />
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                {{ formatDate(execution.started_at) }}
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                {{ formatDuration(execution.duration_ms) }}
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                {{ execution.items_crawled || 0 }}
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                {{ execution.items_indexed || 0 }}
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                {{ execution.retry_attempt > 0 ? `Attempt ${execution.retry_attempt + 1}` : 'First try' }}
              </td>
            </tr>
          </tbody>
        </table>
        <div
          v-if="executions.length > 0 && totalExecutions > executions.length"
          class="px-6 py-4 border-t border-gray-200 bg-gray-50"
        >
          <p class="text-sm text-gray-500 text-center">
            Showing {{ executions.length }} of {{ totalExecutions }} executions
            <button
              v-if="!loadingExecutions"
              class="text-blue-600 hover:text-blue-800 ml-2"
              @click="loadMoreExecutions"
            >
              Load more
            </button>
          </p>
        </div>
      </div>
    </div>

    <!-- Cancel Confirmation Modal -->
    <ConfirmModal
      :show="showCancelModal"
      title="Cancel Job"
      message="Are you sure you want to cancel this job? This action cannot be undone."
      type="danger"
      confirm-text="Cancel Job"
      :loading="cancelling"
      @confirm="cancelJob"
      @cancel="showCancelModal = false"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeftIcon } from '@heroicons/vue/24/outline'
import { crawlerApi } from '../../api/client'
import {
  PageHeader,
  LoadingSpinner,
  ErrorAlert,
  StatusBadge,
  ConfirmModal,
} from '../../components/common'
import JobLogsViewer from '../../components/crawler/JobLogsViewer.vue'

const route = useRoute()
const router = useRouter()

const jobId = computed(() => String(route.params.id))

const loading = ref(true)
const error = ref(null)
const job = ref(null)

const loadingExecutions = ref(false)
const executionsError = ref(null)
const executions = ref([])
const totalExecutions = ref(0)
const executionsOffset = ref(0)
const executionsLimit = 50

const loadingStats = ref(false)
const stats = ref(null)

const showCancelModal = ref(false)
const cancelling = ref(false)
const retrying = ref(false)

const loadJob = async () => {
  try {
    loading.value = true
    error.value = null
    const response = await crawlerApi.jobs.get(jobId.value)
    // API returns job directly, not wrapped in an object
    job.value = response.data
  } catch (err) {
    error.value = 'Unable to load job details. The job may not exist.'
    console.error('[JobDetailView] Error loading job:', err)
  } finally {
    loading.value = false
  }
}

const loadExecutions = async (reset = false) => {
  try {
    loadingExecutions.value = true
    executionsError.value = null
    
    if (reset) {
      executionsOffset.value = 0
      executions.value = []
    }

    const response = await crawlerApi.jobs.executions(jobId.value, {
      limit: executionsLimit,
      offset: executionsOffset.value,
    })

    const newExecutions = response.data?.executions || response.data || []
    executions.value = reset ? newExecutions : [...executions.value, ...newExecutions]
    totalExecutions.value = response.data?.total || 0
    executionsOffset.value += newExecutions.length
  } catch (err) {
    executionsError.value = 'Unable to load execution history.'
    console.error('[JobDetailView] Error loading executions:', err)
  } finally {
    loadingExecutions.value = false
  }
}

const loadStats = async () => {
  try {
    loadingStats.value = true
    const response = await crawlerApi.jobs.stats(jobId.value)
    stats.value = response.data
  } catch (err) {
    console.error('[JobDetailView] Error loading stats:', err)
  } finally {
    loadingStats.value = false
  }
}

const loadMoreExecutions = () => {
  loadExecutions(false)
}

const pauseJob = async () => {
  try {
    await crawlerApi.jobs.pause(jobId.value)
    await loadJob()
  } catch (err) {
    console.error('[JobDetailView] Error pausing job:', err)
  }
}

const resumeJob = async () => {
  try {
    await crawlerApi.jobs.resume(jobId.value)
    await loadJob()
  } catch (err) {
    console.error('[JobDetailView] Error resuming job:', err)
  }
}

const confirmCancel = () => {
  showCancelModal.value = true
}

const cancelJob = async () => {
  try {
    cancelling.value = true
    await crawlerApi.jobs.cancel(jobId.value)
    showCancelModal.value = false
    await loadJob()
  } catch (err) {
    console.error('[JobDetailView] Error cancelling job:', err)
  } finally {
    cancelling.value = false
  }
}

const retryJob = async () => {
  try {
    retrying.value = true
    await crawlerApi.jobs.retry(jobId.value)
    // Reload job and executions to see the new execution
    await Promise.all([loadJob(), loadExecutions(true)])
  } catch (err) {
    console.error('[JobDetailView] Error retrying job:', err)
  } finally {
    retrying.value = false
  }
}

const viewExecution = (executionId) => {
  // Could navigate to execution detail page in the future
  console.log('View execution:', executionId)
}

const goBack = () => {
  router.push('/crawler/jobs')
}

const truncateId = (id) => {
  if (!id) return 'N/A'
  if (id.length <= 12) return id
  return `${id.substring(0, 8)}...`
}

const formatDate = (dateString) => {
  if (!dateString) return 'N/A'
  return new Date(dateString).toLocaleString()
}

const formatNextRun = (job) => {
  if (!job) return 'N/A'
  
  if (!job.schedule_enabled) {
    return job.status === 'pending' ? 'Pending' : 'N/A'
  }

  if (job.next_run_at) {
    try {
      return new Date(job.next_run_at).toLocaleString()
    } catch (err) {
      return 'Invalid date'
    }
  }

  return job.status === 'pending' ? 'Pending' : 'N/A'
}

const formatDuration = (ms) => {
  if (!ms && ms !== 0) return 'N/A'
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  const minutes = Math.floor(ms / 60000)
  const seconds = Math.floor((ms % 60000) / 1000)
  return `${minutes}m ${seconds}s`
}

const successRate = computed(() => {
  if (!stats.value || !stats.value.total_executions || stats.value.total_executions === 0) {
    return 0
  }
  const rate = (stats.value.successful_runs / stats.value.total_executions) * 100
  return Math.round(rate)
})

const canPause = computed(() => {
  if (!job.value) return false
  return ['pending', 'scheduled'].includes(job.value.status) && !job.value.is_paused
})

const canResume = computed(() => {
  if (!job.value) return false
  return job.value.is_paused && job.value.status !== 'cancelled'
})

const canCancel = computed(() => {
  if (!job.value) return false
  return ['pending', 'scheduled', 'running'].includes(job.value.status)
})

const canRetry = computed(() => {
  if (!job.value) return false
  // Can retry completed, failed, or cancelled jobs that aren't currently running
  return ['completed', 'failed', 'cancelled'].includes(job.value.status)
})

onMounted(() => {
  loadJob()
  loadExecutions(true)
  loadStats()
})
</script>
