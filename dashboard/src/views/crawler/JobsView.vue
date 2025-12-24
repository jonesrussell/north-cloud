<template>
  <div>
    <PageHeader title="Crawl Jobs" subtitle="Manage and monitor crawl jobs">
      <template #actions>
        <button
          @click="showCreateModal = true"
          class="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors flex items-center"
        >
          <PlusIcon class="h-5 w-5 mr-2" />
          Create Job
        </button>
      </template>
    </PageHeader>

    <!-- Loading State -->
    <LoadingSpinner v-if="loading" size="lg" text="Loading jobs..." :full-page="true" />

    <!-- Error State -->
    <ErrorAlert v-else-if="error" :message="error" class="mb-6" />

    <!-- Empty State -->
    <div v-else-if="jobs.length === 0" class="bg-white shadow rounded-lg p-8 text-center">
      <BriefcaseIcon class="mx-auto h-12 w-12 text-gray-400" />
      <h3 class="mt-2 text-sm font-medium text-gray-900">No crawl jobs</h3>
      <p class="mt-1 text-sm text-gray-500">Get started by creating your first crawl job.</p>
      <div class="mt-6">
        <button
          @click="showCreateModal = true"
          class="inline-flex items-center px-4 py-2 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700"
        >
          <PlusIcon class="-ml-1 mr-2 h-5 w-5" />
          Create Job
        </button>
      </div>
    </div>

    <!-- Jobs Table -->
    <div v-else class="bg-white shadow rounded-lg overflow-hidden">
      <table class="min-w-full divide-y divide-gray-200">
        <thead class="bg-gray-50">
          <tr>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Job ID
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Source
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Status
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Created
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Next Run
            </th>
            <th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
              Actions
            </th>
          </tr>
        </thead>
        <tbody class="bg-white divide-y divide-gray-200">
          <tr v-for="job in jobs" :key="job.id" class="hover:bg-gray-50">
            <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
              {{ truncateId(job.id) }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
              {{ job.source_name || 'N/A' }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <StatusBadge :status="job.status" />
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
              {{ formatDate(job.created_at) }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
              {{ formatNextRun(job) }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
              <button
                @click="confirmDelete(job)"
                class="text-red-600 hover:text-red-900"
              >
                Delete
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Create Job Modal -->
    <div v-if="showCreateModal" class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div class="bg-white rounded-lg shadow-xl max-w-md w-full mx-4">
        <div class="px-6 py-4 border-b border-gray-200">
          <h2 class="text-lg font-medium text-gray-900">Create Crawl Job</h2>
        </div>

        <form @submit.prevent="createJob" class="p-6">
          <!-- Source Selection -->
          <div class="mb-4">
            <label for="source" class="block text-sm font-medium text-gray-700 mb-2">
              Source <span class="text-red-500">*</span>
            </label>
            <select
              id="source"
              v-model="newJob.source_id"
              required
              class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
              :class="{ 'border-red-500': sourceError }"
              @change="onSourceChange"
            >
              <option value="">Select a source...</option>
              <option v-for="source in sources" :key="source.id" :value="source.id">
                {{ source.name }}
              </option>
            </select>
            <p v-if="sourceError" class="mt-1 text-sm text-red-600">{{ sourceError }}</p>
            <p v-if="loadingSources" class="mt-1 text-xs text-gray-500">Loading sources...</p>
            <p v-else-if="sourcesError" class="mt-1 text-xs text-red-500">{{ sourcesError }}</p>
          </div>

          <!-- URL Display (Read-only) -->
          <div class="mb-4">
            <label for="url" class="block text-sm font-medium text-gray-700 mb-2">
              URL to Crawl <span class="text-red-500">*</span>
            </label>
            <input
              id="url"
              :value="newJob.url || (selectedSource?.url || '')"
              type="url"
              disabled
              placeholder="Select a source to see URL"
              class="w-full px-3 py-2 border border-gray-300 rounded-md bg-gray-50 text-gray-700 cursor-not-allowed"
            />
            <p v-if="!selectedSource" class="mt-1 text-xs text-gray-500">
              URL will be populated automatically when you select a source
            </p>
          </div>

          <!-- Schedule Time -->
          <div class="mb-4">
            <label for="schedule_time" class="block text-sm font-medium text-gray-700 mb-2">
              Schedule (Cron Expression)
            </label>
            <input
              id="schedule_time"
              v-model="newJob.schedule_time"
              type="text"
              placeholder="0 */6 * * *"
              class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <p class="mt-1 text-xs text-gray-500">
              Examples: "0 */6 * * *" (every 6 hours), "0 0 * * *" (daily at midnight)
            </p>
          </div>

          <!-- Schedule Enabled -->
          <div class="mb-4 flex items-center">
            <input
              id="schedule_enabled"
              v-model="newJob.schedule_enabled"
              type="checkbox"
              class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
            />
            <label for="schedule_enabled" class="ml-2 block text-sm text-gray-700">
              Enable scheduled crawling
            </label>
          </div>

          <!-- Error Message -->
          <ErrorAlert v-if="createError" :message="createError" class="mb-4" />

          <!-- Success Message -->
          <div v-if="createSuccess" class="mb-4 bg-green-50 border border-green-200 rounded-lg p-3 text-green-700 text-sm">
            Job created successfully!
          </div>

          <!-- Form Actions -->
          <div class="flex justify-end space-x-3 pt-4 border-t border-gray-200">
            <button
              type="button"
              @click="closeCreateModal"
              class="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
              :disabled="creating"
            >
              Cancel
            </button>
            <button
              type="submit"
              class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
              :disabled="creating"
            >
              {{ creating ? 'Creating...' : 'Create Job' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Delete Confirmation Modal -->
    <ConfirmModal
      :show="showDeleteModal"
      title="Delete Job"
      message="Are you sure you want to delete this job? This action cannot be undone."
      type="danger"
      confirm-text="Delete"
      :loading="deleting"
      @confirm="deleteJob"
      @cancel="showDeleteModal = false"
    />
  </div>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'
import { PlusIcon, BriefcaseIcon } from '@heroicons/vue/24/outline'
import { crawlerApi, sourcesApi } from '../../api/client'
import {
  PageHeader,
  LoadingSpinner,
  ErrorAlert,
  StatusBadge,
  ConfirmModal,
} from '../../components/common'

const loading = ref(true)
const error = ref(null)
const jobs = ref([])
const showCreateModal = ref(false)
const showDeleteModal = ref(false)
const creating = ref(false)
const deleting = ref(false)
const createError = ref(null)
const createSuccess = ref(false)
const urlError = ref(null)
const sourceError = ref(null)
const jobToDelete = ref(null)

// Sources data
const sources = ref([])
const loadingSources = ref(false)
const sourcesError = ref(null)
const selectedSource = ref(null)

// New job form data
const newJob = ref({
  source_id: '',
  url: '',
  schedule_time: '',
  schedule_enabled: false,
})

const loadSources = async () => {
  try {
    loadingSources.value = true
    sourcesError.value = null
    const response = await sourcesApi.list()
    sources.value = response.data?.sources || response.data || []
  } catch (err) {
    sourcesError.value = 'Unable to load sources from source manager'
    console.error('[JobsView] Error loading sources:', err)
  } finally {
    loadingSources.value = false
  }
}

const loadJobs = async () => {
  try {
    loading.value = true
    error.value = null
    const response = await crawlerApi.jobs.list()
    jobs.value = response.data?.jobs || response.data || []
  } catch (err) {
    error.value = 'Unable to load jobs. Backend API may not be available yet.'
    console.error('[JobsView] Error loading jobs:', err)
  } finally {
    loading.value = false
  }
}

const onSourceChange = () => {
  selectedSource.value = sources.value.find((s) => s.id === newJob.value.source_id)
  if (selectedSource.value && selectedSource.value.url) {
    newJob.value.url = selectedSource.value.url
  } else {
    newJob.value.url = ''
  }
}

const validateUrl = (url) => {
  try {
    const parsedUrl = new URL(url)
    if (!['http:', 'https:'].includes(parsedUrl.protocol)) {
      return 'URL must use HTTP or HTTPS protocol'
    }
    return null
  } catch {
    return 'Please enter a valid URL'
  }
}

const createJob = async () => {
  createError.value = null
  createSuccess.value = false
  urlError.value = null
  sourceError.value = null

  if (!newJob.value.source_id) {
    sourceError.value = 'Please select a source'
    return
  }

  // URL is auto-populated from source, but validate it exists
  if (!newJob.value.url || !selectedSource.value?.url) {
    urlError.value = 'Source URL is missing. Please select a valid source.'
    return
  }

  const validationError = validateUrl(newJob.value.url)
  if (validationError) {
    urlError.value = validationError
    return
  }

  try {
    creating.value = true

    const jobData = {
      source_id: newJob.value.source_id,
      source_name: selectedSource.value?.name || '',
      url: newJob.value.url.trim(),
      schedule_time: newJob.value.schedule_time.trim(),
      schedule_enabled: newJob.value.schedule_enabled,
    }

    await crawlerApi.jobs.create(jobData)
    createSuccess.value = true

    newJob.value = {
      source_id: '',
      url: '',
      schedule_time: '',
      schedule_enabled: false,
    }
    selectedSource.value = null

    await loadJobs()

    setTimeout(() => {
      closeCreateModal()
    }, 1500)
  } catch (err) {
    createError.value = err.response?.data?.error || 'Failed to create job. Please try again.'
    console.error('[JobsView] Error creating job:', err)
  } finally {
    creating.value = false
  }
}

const closeCreateModal = () => {
  showCreateModal.value = false
  createError.value = null
  createSuccess.value = false
  urlError.value = null
  sourceError.value = null
  newJob.value = {
    source_id: '',
    url: '',
    schedule_time: '',
    schedule_enabled: false,
  }
  selectedSource.value = null
}

watch(showCreateModal, (newValue) => {
  if (newValue && sources.value.length === 0) {
    loadSources()
  }
})

const confirmDelete = (job) => {
  jobToDelete.value = job
  showDeleteModal.value = true
}

const deleteJob = async () => {
  if (!jobToDelete.value) return

  try {
    deleting.value = true
    await crawlerApi.jobs.delete(jobToDelete.value.id)
    jobs.value = jobs.value.filter((j) => j.id !== jobToDelete.value.id)
    showDeleteModal.value = false
    jobToDelete.value = null
  } catch (err) {
    console.error('[JobsView] Error deleting job:', err)
  } finally {
    deleting.value = false
  }
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
  if (!job.schedule_enabled || !job.schedule_time) {
    return job.status === 'pending' ? 'Pending' : 'N/A'
  }

  try {
    const nextRun = calculateNextRunTime(job.schedule_time)
    if (nextRun) {
      return nextRun.toLocaleString()
    }
    return 'Invalid schedule'
  } catch (err) {
    console.error('[JobsView] Error calculating next run time:', err)
    return 'Invalid schedule'
  }
}

const calculateNextRunTime = (cronExpression) => {
  if (!cronExpression) return null

  const parts = cronExpression.trim().split(/\s+/)
  if (parts.length !== 5) {
    return null
  }

  const [minute, hour] = parts
  const now = new Date()
  let nextRun = new Date(now)

  nextRun.setSeconds(0)
  nextRun.setMilliseconds(0)

  let targetMinute = 0
  if (minute !== '*') {
    if (minute.startsWith('*/')) {
      const step = parseInt(minute.substring(2), 10)
      if (!isNaN(step) && step > 0) {
        const currentMinute = now.getMinutes()
        targetMinute = Math.ceil((currentMinute + 1) / step) * step
        if (targetMinute >= 60) {
          targetMinute = 0
          nextRun.setHours(nextRun.getHours() + 1)
        }
      }
    } else {
      const minuteValue = parseInt(minute, 10)
      if (!isNaN(minuteValue) && minuteValue >= 0 && minuteValue < 60) {
        targetMinute = minuteValue
      }
    }
  }
  nextRun.setMinutes(targetMinute)

  let targetHour = now.getHours()
  if (hour !== '*') {
    if (hour.startsWith('*/')) {
      const step = parseInt(hour.substring(2), 10)
      if (!isNaN(step) && step > 0) {
        const currentHour = now.getHours()
        let nextHour = Math.ceil((currentHour + 1) / step) * step
        if (nextHour >= 24) {
          nextHour = 0
          nextRun.setDate(nextRun.getDate() + 1)
        }
        targetHour = nextHour
      }
    } else {
      const hourValue = parseInt(hour, 10)
      if (!isNaN(hourValue) && hourValue >= 0 && hourValue < 24) {
        targetHour = hourValue
      }
    }
  }
  nextRun.setHours(targetHour)

  if (nextRun <= now) {
    if (hour.startsWith('*/')) {
      const step = parseInt(hour.substring(2), 10)
      if (!isNaN(step) && step > 0) {
        nextRun.setHours(nextRun.getHours() + step)
      }
    } else {
      nextRun.setDate(nextRun.getDate() + 1)
    }
  }

  return nextRun
}

onMounted(() => {
  loadJobs()
  loadSources()
})
</script>
