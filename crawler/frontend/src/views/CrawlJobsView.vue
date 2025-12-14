<template>
  <div>
    <div class="mb-6 flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-bold text-gray-900">Crawl Jobs</h1>
        <p class="mt-1 text-sm text-gray-600">Manage and monitor crawl jobs</p>
      </div>
      <button
        @click="showCreateModal = true"
        class="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors"
      >
        Create Job
      </button>
    </div>

    <!-- Jobs List -->
    <div v-if="loading" class="text-center py-8 text-gray-500">
      Loading jobs...
    </div>
    <div v-else-if="error" class="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
      {{ error }}
    </div>
    <div v-else-if="jobs.length === 0" class="bg-white shadow rounded-lg p-8 text-center">
      <div class="text-gray-500">
        No crawl jobs found. Create your first job to get started.
      </div>
    </div>
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
              Actions
            </th>
          </tr>
        </thead>
        <tbody class="bg-white divide-y divide-gray-200">
          <tr v-for="job in jobs" :key="job.id">
            <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
              {{ job.id }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
              {{ job.source || 'N/A' }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <span
                :class="[
                  'px-2 inline-flex text-xs leading-5 font-semibold rounded-full',
                  job.status === 'running' ? 'bg-green-100 text-green-800' :
                  job.status === 'completed' ? 'bg-blue-100 text-blue-800' :
                  job.status === 'failed' ? 'bg-red-100 text-red-800' :
                  'bg-gray-100 text-gray-800'
                ]"
              >
                {{ job.status }}
              </span>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
              {{ formatDate(job.created_at) }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm font-medium">
              <button
                @click="deleteJob(job.id)"
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
      <div class="bg-white rounded-lg p-6 max-w-md w-full">
        <h2 class="text-xl font-bold mb-4">Create Crawl Job</h2>

        <form @submit.prevent="createJob">
          <!-- URL Input -->
          <div class="mb-4">
            <label for="url" class="block text-sm font-medium text-gray-700 mb-2">
              URL to Crawl <span class="text-red-500">*</span>
            </label>
            <input
              id="url"
              v-model="newJob.url"
              type="url"
              required
              placeholder="https://example.com"
              class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
              :class="{ 'border-red-500': urlError }"
            />
            <p v-if="urlError" class="mt-1 text-sm text-red-600">{{ urlError }}</p>
          </div>

          <!-- Source Name Input (Optional) -->
          <div class="mb-4">
            <label for="source" class="block text-sm font-medium text-gray-700 mb-2">
              Source Name (Optional)
            </label>
            <input
              id="source"
              v-model="newJob.source"
              type="text"
              placeholder="Enter source name or leave empty"
              class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <p class="mt-1 text-xs text-gray-500">If left empty, the domain will be used</p>
          </div>

          <!-- Error Message -->
          <div v-if="createError" class="mb-4 bg-red-50 border border-red-200 rounded-lg p-3 text-red-700 text-sm">
            {{ createError }}
          </div>

          <!-- Success Message -->
          <div v-if="createSuccess" class="mb-4 bg-green-50 border border-green-200 rounded-lg p-3 text-green-700 text-sm">
            Job created successfully!
          </div>

          <!-- Form Actions -->
          <div class="flex justify-end space-x-2">
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
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { crawlerApi } from '../api/client'

const loading = ref(true)
const error = ref(null)
const jobs = ref([])
const showCreateModal = ref(false)
const creating = ref(false)
const createError = ref(null)
const createSuccess = ref(false)
const urlError = ref(null)

// New job form data
const newJob = ref({
  url: '',
  source: ''
})

const loadJobs = async () => {
  try {
    loading.value = true
    error.value = null
    jobs.value = await crawlerApi.listJobs()
  } catch (err) {
    error.value = 'Unable to load jobs. Backend API may not be available yet.'
    console.error('Error loading jobs:', err)
  } finally {
    loading.value = false
  }
}

const validateUrl = (url) => {
  try {
    const parsedUrl = new URL(url)
    if (!['http:', 'https:'].includes(parsedUrl.protocol)) {
      return 'URL must use HTTP or HTTPS protocol'
    }
    return null
  } catch (err) {
    return 'Please enter a valid URL'
  }
}

const createJob = async () => {
  // Reset messages
  createError.value = null
  createSuccess.value = false
  urlError.value = null

  // Validate URL
  const validationError = validateUrl(newJob.value.url)
  if (validationError) {
    urlError.value = validationError
    return
  }

  try {
    creating.value = true

    // Prepare job data
    const jobData = {
      url: newJob.value.url.trim()
    }

    // Add source if provided
    if (newJob.value.source && newJob.value.source.trim()) {
      jobData.source = newJob.value.source.trim()
    }

    // Create the job
    const result = await crawlerApi.createJob(jobData)

    // Show success message
    createSuccess.value = true

    // Reset form
    newJob.value = {
      url: '',
      source: ''
    }

    // Reload jobs list
    await loadJobs()

    // Close modal after a short delay
    setTimeout(() => {
      closeCreateModal()
    }, 1500)
  } catch (err) {
    createError.value = err.response?.data?.error || 'Failed to create job. Please try again.'
    console.error('Error creating job:', err)
  } finally {
    creating.value = false
  }
}

const closeCreateModal = () => {
  showCreateModal.value = false
  createError.value = null
  createSuccess.value = false
  urlError.value = null
  newJob.value = {
    url: '',
    source: ''
  }
}

const deleteJob = async (id) => {
  if (!confirm('Are you sure you want to delete this job?')) return
  try {
    await crawlerApi.deleteJob(id)
    jobs.value = jobs.value.filter(j => j.id !== id)
  } catch (err) {
    alert('Failed to delete job')
    console.error('Error deleting job:', err)
  }
}

const formatDate = (dateString) => {
  if (!dateString) return 'N/A'
  return new Date(dateString).toLocaleString()
}

onMounted(() => {
  loadJobs()
})
</script>
