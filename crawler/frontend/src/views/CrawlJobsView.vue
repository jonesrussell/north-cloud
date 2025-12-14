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

    <!-- Create Job Modal (placeholder) -->
    <div v-if="showCreateModal" class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div class="bg-white rounded-lg p-6 max-w-md w-full">
        <h2 class="text-xl font-bold mb-4">Create Crawl Job</h2>
        <p class="text-gray-600 mb-4">Job creation form will be implemented based on crawler API requirements.</p>
        <div class="flex justify-end space-x-2">
          <button
            @click="showCreateModal = false"
            class="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
          >
            Cancel
          </button>
        </div>
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
