<template>
  <div>
    <PageHeader
      title="Queued Links"
      subtitle="View and manage discovered links"
    />

    <!-- Loading State -->
    <LoadingSpinner
      v-if="loading"
      size="lg"
      text="Loading queued links..."
      :full-page="true"
    />

    <!-- Error State -->
    <ErrorAlert
      v-else-if="error"
      :message="error"
      class="mb-6"
    />

    <!-- Filters and Search -->
    <div
      v-if="!loading && !error"
      class="bg-white shadow rounded-lg p-4 mb-6"
    >
      <div class="grid grid-cols-1 gap-4 md:grid-cols-4">
        <!-- Search -->
        <div class="md:col-span-2">
          <label
            for="search"
            class="block text-sm font-medium text-gray-700 mb-1"
          >
            Search URL
          </label>
          <input
            id="search"
            v-model="filters.search"
            type="text"
            placeholder="Search by URL..."
            class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
            @input="debouncedLoadLinks"
          >
        </div>

        <!-- Status Filter -->
        <div>
          <label
            for="status"
            class="block text-sm font-medium text-gray-700 mb-1"
          >
            Status
          </label>
          <select
            id="status"
            v-model="filters.status"
            class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
            @change="loadLinks"
          >
            <option value="">
              All
            </option>
            <option value="pending">
              Pending
            </option>
            <option value="processing">
              Processing
            </option>
            <option value="completed">
              Completed
            </option>
            <option value="failed">
              Failed
            </option>
          </select>
        </div>

        <!-- Source Filter -->
        <div>
          <label
            for="source"
            class="block text-sm font-medium text-gray-700 mb-1"
          >
            Source
          </label>
          <input
            id="source"
            v-model="filters.source_name"
            type="text"
            placeholder="Filter by source..."
            class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
            @input="debouncedLoadLinks"
          >
        </div>
      </div>

      <!-- Sort Controls -->
      <div class="mt-4 flex items-center gap-4">
        <div class="flex items-center gap-2">
          <label
            for="sort"
            class="text-sm font-medium text-gray-700"
          >
            Sort by:
          </label>
          <select
            id="sort"
            v-model="filters.sort"
            class="px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
            @change="loadLinks"
          >
            <option value="priority">
              Priority
            </option>
            <option value="queued_at">
              Queued At
            </option>
            <option value="discovered_at">
              Discovered At
            </option>
          </select>
        </div>
        <div class="flex items-center gap-2">
          <label
            for="order"
            class="text-sm font-medium text-gray-700"
          >
            Order:
          </label>
          <select
            id="order"
            v-model="filters.order"
            class="px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
            @change="loadLinks"
          >
            <option value="desc">
              Descending
            </option>
            <option value="asc">
              Ascending
            </option>
          </select>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div
      v-if="!loading && !error && links.length === 0"
      class="bg-white shadow rounded-lg p-8 text-center"
    >
      <LinkIcon class="mx-auto h-12 w-12 text-gray-400" />
      <h3 class="mt-2 text-sm font-medium text-gray-900">
        No queued links
      </h3>
      <p class="mt-1 text-sm text-gray-500">
        Links discovered during crawling will appear here when link saving is enabled.
      </p>
    </div>

    <!-- Links Table -->
    <div
      v-else-if="!loading && !error"
      class="bg-white shadow rounded-lg overflow-hidden"
    >
      <table class="min-w-full divide-y divide-gray-200">
        <thead class="bg-gray-50">
          <tr>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              URL
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Source
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Parent URL
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Depth
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Discovered
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Status
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Priority
            </th>
            <th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
              Actions
            </th>
          </tr>
        </thead>
        <tbody class="bg-white divide-y divide-gray-200">
          <tr
            v-for="link in links"
            :key="link.id"
            class="hover:bg-gray-50"
          >
            <td class="px-6 py-4 text-sm">
              <a
                :href="link.url"
                target="_blank"
                rel="noopener noreferrer"
                class="text-blue-600 hover:text-blue-800 truncate block max-w-xs"
                :title="link.url"
              >
                {{ truncateUrl(link.url) }}
              </a>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
              {{ link.source_name }}
            </td>
            <td class="px-6 py-4 text-sm text-gray-500">
              <span
                v-if="link.parent_url"
                :title="link.parent_url"
                class="truncate block max-w-xs"
              >
                {{ truncateUrl(link.parent_url) }}
              </span>
              <span
                v-else
                class="text-gray-400"
              >—</span>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
              {{ link.depth }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
              {{ formatDate(link.discovered_at) }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <StatusBadge :status="link.status" />
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
              {{ link.priority }}
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
              <div class="flex justify-end gap-2">
                <button
                  class="text-blue-600 hover:text-blue-900"
                  @click="createJobFromLink(link)"
                >
                  Create Job
                </button>
                <button
                  class="text-green-600 hover:text-green-900"
                  @click="createSourceFromLink(link)"
                >
                  Create Source
                </button>
                <button
                  class="text-red-600 hover:text-red-900"
                  @click="confirmDelete(link)"
                >
                  Delete
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>

      <!-- Pagination -->
      <div
        v-if="total > filters.limit"
        class="bg-white px-4 py-3 border-t border-gray-200 sm:px-6 flex items-center justify-between"
      >
        <div class="flex-1 flex justify-between sm:hidden">
          <button
            :disabled="filters.offset === 0"
            class="relative inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
            @click="previousPage"
          >
            Previous
          </button>
          <button
            :disabled="filters.offset + filters.limit >= total"
            class="ml-3 relative inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
            @click="nextPage"
          >
            Next
          </button>
        </div>
        <div class="hidden sm:flex-1 sm:flex sm:items-center sm:justify-between">
          <div>
            <p class="text-sm text-gray-700">
              Showing
              <span class="font-medium">{{ filters.offset + 1 }}</span>
              to
              <span class="font-medium">{{ Math.min(filters.offset + filters.limit, total) }}</span>
              of
              <span class="font-medium">{{ total }}</span>
              results
            </p>
          </div>
          <div>
            <nav
              class="relative z-0 inline-flex rounded-md shadow-sm -space-x-px"
              aria-label="Pagination"
            >
              <button
                :disabled="filters.offset === 0"
                class="relative inline-flex items-center px-2 py-2 rounded-l-md border border-gray-300 bg-white text-sm font-medium text-gray-500 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                @click="previousPage"
              >
                Previous
              </button>
              <button
                :disabled="filters.offset + filters.limit >= total"
                class="relative inline-flex items-center px-2 py-2 rounded-r-md border border-gray-300 bg-white text-sm font-medium text-gray-500 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                @click="nextPage"
              >
                Next
              </button>
            </nav>
          </div>
        </div>
      </div>
    </div>

    <!-- Create Job Modal -->
    <div
      v-if="showCreateJobModal"
      class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
    >
      <div class="bg-white rounded-lg shadow-xl max-w-md w-full mx-4">
        <div class="px-6 py-4 border-b border-gray-200">
          <h2 class="text-lg font-medium text-gray-900">
            Create Job from Link
          </h2>
        </div>

        <form
          class="p-6"
          @submit.prevent="submitCreateJob"
        >
          <div class="mb-4">
            <label
              for="job-url"
              class="block text-sm font-medium text-gray-700 mb-2"
            >
              URL <span class="text-red-500">*</span>
            </label>
            <input
              id="job-url"
              :value="selectedLink?.url"
              type="url"
              disabled
              class="w-full px-3 py-2 border border-gray-300 rounded-md bg-gray-50 text-gray-700 cursor-not-allowed"
            >
          </div>

          <div class="mb-4">
            <label
              for="job-source-name"
              class="block text-sm font-medium text-gray-700 mb-2"
            >
              Source Name
            </label>
            <input
              id="job-source-name"
              v-model="newJob.source_name"
              type="text"
              :placeholder="selectedLink?.source_name"
              class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
          </div>

          <div class="mb-4">
            <label
              for="job-schedule-time"
              class="block text-sm font-medium text-gray-700 mb-2"
            >
              Schedule (Cron Expression)
            </label>
            <input
              id="job-schedule-time"
              v-model="newJob.schedule_time"
              type="text"
              placeholder="0 */6 * * *"
              class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
            <p class="mt-1 text-xs text-gray-500">
              Examples: "0 */6 * * *" (every 6 hours), "0 0 * * *" (daily at midnight)
            </p>
          </div>

          <div class="mb-4 flex items-center">
            <input
              id="job-schedule-enabled"
              v-model="newJob.schedule_enabled"
              type="checkbox"
              class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
            >
            <label
              for="job-schedule-enabled"
              class="ml-2 block text-sm text-gray-700"
            >
              Enable scheduled crawling
            </label>
          </div>

          <ErrorAlert
            v-if="createJobError"
            :message="createJobError"
            class="mb-4"
          />

          <div class="flex justify-end space-x-3 pt-4 border-t border-gray-200">
            <button
              type="button"
              class="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
              :disabled="creatingJob"
              @click="closeCreateJobModal"
            >
              Cancel
            </button>
            <button
              type="submit"
              class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
              :disabled="creatingJob"
            >
              {{ creatingJob ? 'Creating...' : 'Create Job' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Delete Confirmation Modal -->
    <ConfirmModal
      :show="showDeleteModal"
      title="Delete Queued Link"
      message="Are you sure you want to delete this queued link? This action cannot be undone."
      type="danger"
      confirm-text="Delete"
      :loading="deleting"
      @confirm="deleteLink"
      @cancel="showDeleteModal = false"
    />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { LinkIcon } from '@heroicons/vue/24/outline'
import { crawlerApi } from '../../api/client'
import {
  PageHeader,
  LoadingSpinner,
  ErrorAlert,
  StatusBadge,
  ConfirmModal,
} from '../../components/common'

const router = useRouter()

const loading = ref(true)
const error = ref(null)
const links = ref([])
const total = ref(0)
const showCreateJobModal = ref(false)
const showDeleteModal = ref(false)
const creatingJob = ref(false)
const deleting = ref(false)
const createJobError = ref(null)
const selectedLink = ref(null)
const linkToDelete = ref(null)

const filters = ref({
  status: '',
  source_name: '',
  search: '',
  sort: 'priority',
  order: 'desc',
  limit: 50,
  offset: 0,
})

const newJob = ref({
  source_name: '',
  schedule_time: '',
  schedule_enabled: false,
})

let debounceTimer = null

const debouncedLoadLinks = () => {
  if (debounceTimer) {
    clearTimeout(debounceTimer)
  }
  debounceTimer = setTimeout(() => {
    filters.value.offset = 0 // Reset to first page on search
    loadLinks()
  }, 500)
}

const loadLinks = async () => {
  try {
    loading.value = true
    error.value = null

    const params = {
      limit: filters.value.limit,
      offset: filters.value.offset,
      sort: filters.value.sort,
      order: filters.value.order,
    }

    if (filters.value.status) {
      params.status = filters.value.status
    }
    if (filters.value.source_name) {
      params.source_name = filters.value.source_name
    }
    if (filters.value.search) {
      params.search = filters.value.search
    }

    const response = await crawlerApi.queuedLinks.list(params)
    links.value = response.data?.links || []
    total.value = response.data?.total || 0
  } catch (err) {
    error.value = 'Unable to load queued links. Backend API may not be available yet.'
    console.error('[QueuedLinksView] Error loading links:', err)
  } finally {
    loading.value = false
  }
}

const createJobFromLink = (link) => {
  selectedLink.value = link
  newJob.value = {
    source_name: link.source_name,
    schedule_time: '',
    schedule_enabled: false,
  }
  showCreateJobModal.value = true
}

const submitCreateJob = async () => {
  if (!selectedLink.value) return

  try {
    creatingJob.value = true
    createJobError.value = null

    const jobData = {
      source_id: selectedLink.value.source_id,
      source_name: newJob.value.source_name || selectedLink.value.source_name,
      schedule_time: newJob.value.schedule_time.trim(),
      schedule_enabled: newJob.value.schedule_enabled,
    }

    await crawlerApi.queuedLinks.createJob(selectedLink.value.id, jobData)
    closeCreateJobModal()
    await loadLinks()
  } catch (err) {
    createJobError.value = err.response?.data?.error || 'Failed to create job. Please try again.'
    console.error('[QueuedLinksView] Error creating job:', err)
  } finally {
    creatingJob.value = false
  }
}

const closeCreateJobModal = () => {
  showCreateJobModal.value = false
  createJobError.value = null
  selectedLink.value = null
  newJob.value = {
    source_name: '',
    schedule_time: '',
    schedule_enabled: false,
  }
}

const createSourceFromLink = (link) => {
  router.push(`/sources/new?url=${encodeURIComponent(link.url)}`)
}

const confirmDelete = (link) => {
  linkToDelete.value = link
  showDeleteModal.value = true
}

const deleteLink = async () => {
  if (!linkToDelete.value) return

  try {
    deleting.value = true
    await crawlerApi.queuedLinks.delete(linkToDelete.value.id)
    await loadLinks()
    showDeleteModal.value = false
    linkToDelete.value = null
  } catch (err) {
    console.error('[QueuedLinksView] Error deleting link:', err)
  } finally {
    deleting.value = false
  }
}

const previousPage = () => {
  if (filters.value.offset > 0) {
    filters.value.offset -= filters.value.limit
    loadLinks()
  }
}

const nextPage = () => {
  if (filters.value.offset + filters.value.limit < total.value) {
    filters.value.offset += filters.value.limit
    loadLinks()
  }
}

const truncateUrl = (url) => {
  if (!url) return 'N/A'
  if (url.length <= 60) return url
  return `${url.substring(0, 57)}...`
}

const formatDate = (dateString) => {
  if (!dateString) return 'N/A'
  return new Date(dateString).toLocaleString()
}

onMounted(() => {
  loadLinks()
})
</script>

