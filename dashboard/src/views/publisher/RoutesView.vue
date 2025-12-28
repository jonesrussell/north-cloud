<template>
  <div>
    <PageHeader
      title="Routes"
      subtitle="Configure routing rules from sources to channels"
    />

    <div class="bg-white shadow rounded-lg p-6">
      <div class="flex justify-between items-center mb-4">
        <div>
          <label class="flex items-center text-sm text-gray-700">
            <input
              type="checkbox"
              v-model="enabledOnly"
              @change="loadRoutes"
              class="mr-2 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
            >
            Show enabled only
          </label>
        </div>
        <button
          class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500"
          @click="openCreateModal"
        >
          + Add Route
        </button>
      </div>

      <LoadingSpinner
        v-if="loading"
        text="Loading routes..."
      />

      <ErrorAlert
        v-else-if="error"
        :message="error"
        class="mb-4"
      />

      <div v-else>
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200">
            <thead class="bg-gray-50">
              <tr>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Source
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Channel
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Min Quality
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Topics
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Status
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody class="bg-white divide-y divide-gray-200">
              <tr
                v-for="route in routes"
                :key="route.id"
                class="hover:bg-gray-50"
              >
                <td class="px-6 py-4">
                  <div class="text-sm font-medium text-gray-900">
                    {{ route.source_name }}
                  </div>
                  <div class="text-xs text-gray-500">
                    {{ route.source_index_pattern }}
                  </div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                  <code class="text-sm text-gray-900">{{ route.channel_name }}</code>
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  {{ route.min_quality_score }}
                </td>
                <td class="px-6 py-4">
                  <div class="flex flex-wrap gap-1">
                    <span
                      v-for="topic in (route.topics || [])"
                      :key="topic"
                      class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800"
                    >
                      {{ topic }}
                    </span>
                    <span
                      v-if="!route.topics || route.topics.length === 0"
                      class="text-xs text-gray-400"
                    >
                      All
                    </span>
                  </div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                  <StatusBadge
                    :status="route.enabled ? 'enabled' : 'disabled'"
                  />
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm font-medium">
                  <button
                    class="text-blue-600 hover:text-blue-900 mr-4"
                    @click="openEditModal(route)"
                  >
                    Edit
                  </button>
                  <button
                    class="text-red-600 hover:text-red-900"
                    @click="deleteRoute(route)"
                  >
                    Delete
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div
          v-if="!loading && routes.length === 0"
          class="text-center py-12 text-gray-500"
        >
          No routes found. Click "Add Route" to create one.
        </div>
      </div>
    </div>

    <!-- Create/Edit Modal -->
    <div
      v-if="showModal"
      class="fixed inset-0 z-50 overflow-y-auto"
      @click.self="closeModal"
    >
      <div class="flex items-center justify-center min-h-screen px-4">
        <div
          class="fixed inset-0 bg-gray-500 bg-opacity-75 transition-opacity"
          @click="closeModal"
        />
        <div class="relative bg-white rounded-lg shadow-xl max-w-md w-full p-6">
          <div class="flex justify-between items-center mb-4">
            <h2 class="text-xl font-semibold text-gray-900">
              {{ isEditing ? 'Edit Route' : 'Create Route' }}
            </h2>
            <button
              class="text-gray-400 hover:text-gray-500"
              @click="closeModal"
            >
              <span class="sr-only">Close</span>
              <svg
                class="h-6 w-6"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>

          <ErrorAlert
            v-if="modalError"
            :message="modalError"
            class="mb-4"
          />

          <form @submit.prevent="saveRoute">
            <div class="mb-4">
              <label class="block text-sm font-medium text-gray-700 mb-1">
                Source *
              </label>
              <select
                v-model="formData.source_id"
                required
                class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
              >
                <option value="">Select a source...</option>
                <option
                  v-for="source in sources"
                  :key="source.id"
                  :value="source.id"
                >
                  {{ source.name }} ({{ source.index_pattern }})
                </option>
              </select>
            </div>

            <div class="mb-4">
              <label class="block text-sm font-medium text-gray-700 mb-1">
                Channel *
              </label>
              <select
                v-model="formData.channel_id"
                required
                class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
              >
                <option value="">Select a channel...</option>
                <option
                  v-for="channel in channels"
                  :key="channel.id"
                  :value="channel.id"
                >
                  {{ channel.name }}
                </option>
              </select>
            </div>

            <div class="mb-4">
              <label class="block text-sm font-medium text-gray-700 mb-1">
                Minimum Quality Score
              </label>
              <input
                type="number"
                v-model.number="formData.min_quality_score"
                min="0"
                max="100"
                class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
              >
              <p class="mt-1 text-xs text-gray-500">
                Only publish articles with quality score >= this value (0-100)
              </p>
            </div>

            <div class="mb-4">
              <label class="block text-sm font-medium text-gray-700 mb-1">
                Topics
              </label>
              <input
                type="text"
                v-model="topicsInput"
                placeholder="e.g., crime, news, local"
                class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
              >
              <p class="mt-1 text-xs text-gray-500">
                Comma-separated list of topics to filter (leave empty for all topics)
              </p>
            </div>

            <div class="mb-4">
              <label class="flex items-center">
                <input
                  type="checkbox"
                  v-model="formData.enabled"
                  class="mr-2 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                >
                <span class="text-sm text-gray-700">Enabled</span>
              </label>
            </div>

            <div class="flex justify-end space-x-3 mt-6">
              <button
                type="button"
                class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
                @click="closeModal"
              >
                Cancel
              </button>
              <button
                type="submit"
                class="px-4 py-2 bg-green-600 text-white rounded-md hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-green-500 disabled:opacity-50"
                :disabled="saving"
              >
                {{ saving ? 'Saving...' : 'Save' }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { publisherApi } from '../../api/client'
import type { Route, Source, Channel, CreateRouteRequest, UpdateRouteRequest } from '../../types/publisher'
import { PageHeader, LoadingSpinner, ErrorAlert, StatusBadge } from '../../components/common'

const routes = ref<Route[]>([])
const sources = ref<Source[]>([])
const channels = ref<Channel[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const enabledOnly = ref(false)

const showModal = ref(false)
const isEditing = ref(false)
const modalError = ref<string | null>(null)
const saving = ref(false)
const formData = ref<CreateRouteRequest>({
  source_id: 0,
  channel_id: 0,
  min_quality_score: 50,
  topics: null,
  enabled: true,
})
const currentRoute = ref<Route | null>(null)
const topicsInput = ref('')

const loadRoutes = async (): Promise<void> => {
  loading.value = true
  error.value = null
  try {
    const response = await publisherApi.routes.list(enabledOnly.value)
    routes.value = response.data.routes || []
  } catch (err) {
    const axiosError = err as { response?: { data?: { error?: string } } }
    error.value = axiosError.response?.data?.error || 'Failed to load routes'
  } finally {
    loading.value = false
  }
}

const loadSources = async (): Promise<void> => {
  try {
    const response = await publisherApi.sources.list(true) // Only enabled sources
    sources.value = response.data.sources || []
  } catch (err) {
    console.error('Failed to load sources:', err)
  }
}

const loadChannels = async (): Promise<void> => {
  try {
    const response = await publisherApi.channels.list(true) // Only enabled channels
    channels.value = response.data.channels || []
  } catch (err) {
    console.error('Failed to load channels:', err)
  }
}

const openCreateModal = (): void => {
  isEditing.value = false
  formData.value = {
    source_id: 0,
    channel_id: 0,
    min_quality_score: 50,
    topics: null,
    enabled: true,
  }
  topicsInput.value = ''
  currentRoute.value = null
  modalError.value = null
  showModal.value = true
}

const openEditModal = (route: Route): void => {
  isEditing.value = true
  formData.value = {
    source_id: route.source_id,
    channel_id: route.channel_id,
    min_quality_score: route.min_quality_score,
    topics: route.topics,
    enabled: route.enabled,
  }
  topicsInput.value = (route.topics || []).join(', ')
  currentRoute.value = route
  modalError.value = null
  showModal.value = true
}

const closeModal = (): void => {
  showModal.value = false
  formData.value = {
    source_id: 0,
    channel_id: 0,
    min_quality_score: 50,
    topics: null,
    enabled: true,
  }
  topicsInput.value = ''
  currentRoute.value = null
  modalError.value = null
}

const saveRoute = async (): Promise<void> => {
  saving.value = true
  modalError.value = null

  // Parse topics from comma-separated input
  const topics = topicsInput.value
    .split(',')
    .map(t => t.trim())
    .filter(t => t.length > 0)

  const payload: CreateRouteRequest | UpdateRouteRequest = {
    ...formData.value,
    topics: topics.length > 0 ? topics : null,
  }

  try {
    if (isEditing.value && currentRoute.value) {
      await publisherApi.routes.update(currentRoute.value.id, payload as UpdateRouteRequest)
    } else {
      await publisherApi.routes.create(payload as CreateRouteRequest)
    }
    closeModal()
    await loadRoutes()
  } catch (err) {
    const axiosError = err as { response?: { data?: { error?: string } } }
    modalError.value = axiosError.response?.data?.error || 'Failed to save route'
  } finally {
    saving.value = false
  }
}

const deleteRoute = async (route: Route): Promise<void> => {
  if (!confirm(`Are you sure you want to delete the route from "${route.source_name}" to "${route.channel_name}"?`)) {
    return
  }

  try {
    await publisherApi.routes.delete(route.id)
    await loadRoutes()
  } catch (err) {
    const axiosError = err as { response?: { data?: { error?: string } } }
    error.value = axiosError.response?.data?.error || 'Failed to delete route'
  }
}

onMounted(() => {
  loadRoutes()
  loadSources()
  loadChannels()
})
</script>

