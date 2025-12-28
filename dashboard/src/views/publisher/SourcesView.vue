<template>
  <div>
    <PageHeader
      title="Sources"
      subtitle="Manage Elasticsearch indexes as content sources"
    />

    <div class="bg-white shadow rounded-lg p-6">
      <div class="flex justify-between items-center mb-4">
        <div>
          <label class="flex items-center text-sm text-gray-700">
            <input
              v-model="enabledOnly"
              type="checkbox"
              class="mr-2 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
              @change="loadSources"
            >
            Show enabled only
          </label>
        </div>
        <button
          class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500"
          @click="openCreateModal"
        >
          + Add Source
        </button>
      </div>

      <LoadingSpinner
        v-if="loading"
        text="Loading sources..."
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
                  Name
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Index Pattern
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
              <tr
                v-for="source in sources"
                :key="source.id"
                class="hover:bg-gray-50"
              >
                <td class="px-6 py-4 whitespace-nowrap">
                  <div class="text-sm font-medium text-gray-900">
                    {{ source.name }}
                  </div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                  <code class="text-sm text-gray-600">{{ source.index_pattern }}</code>
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                  <StatusBadge
                    :status="source.enabled ? 'enabled' : 'disabled'"
                  />
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  {{ formatDate(source.created_at) }}
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm font-medium">
                  <button
                    class="text-blue-600 hover:text-blue-900 mr-4"
                    @click="openEditModal(source)"
                  >
                    Edit
                  </button>
                  <button
                    class="text-red-600 hover:text-red-900"
                    @click="deleteSource(source)"
                  >
                    Delete
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div
          v-if="!loading && sources.length === 0"
          class="text-center py-12 text-gray-500"
        >
          No sources found. Click "Add Source" to create one.
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
              {{ isEditing ? 'Edit Source' : 'Create Source' }}
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

          <form @submit.prevent="saveSource">
            <div class="mb-4">
              <label class="block text-sm font-medium text-gray-700 mb-1">
                Name *
              </label>
              <input
                v-model="formData.name"
                type="text"
                placeholder="e.g., sudbury_com"
                required
                class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
              >
            </div>

            <div class="mb-4">
              <label class="block text-sm font-medium text-gray-700 mb-1">
                Index Pattern *
              </label>
              <input
                v-model="formData.index_pattern"
                type="text"
                placeholder="e.g., sudbury_com_classified_content"
                required
                class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
              >
              <p class="mt-1 text-xs text-gray-500">
                Elasticsearch index pattern to query
              </p>
            </div>

            <div class="mb-4">
              <label class="flex items-center">
                <input
                  v-model="formData.enabled"
                  type="checkbox"
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
import type { Source, CreateSourceRequest, UpdateSourceRequest } from '../../types/publisher'
import { PageHeader, LoadingSpinner, ErrorAlert, StatusBadge } from '../../components/common'

const sources = ref<Source[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const enabledOnly = ref(false)

const showModal = ref(false)
const isEditing = ref(false)
const modalError = ref<string | null>(null)
const saving = ref(false)
const formData = ref<CreateSourceRequest>({
  name: '',
  index_pattern: '',
  enabled: true,
})
const currentSource = ref<Source | null>(null)

const loadSources = async (): Promise<void> => {
  loading.value = true
  error.value = null
  try {
    const response = await publisherApi.sources.list(enabledOnly.value)
    sources.value = response.data.sources || []
  } catch (err) {
    const axiosError = err as { response?: { data?: { error?: string } } }
    error.value = axiosError.response?.data?.error || 'Failed to load sources'
  } finally {
    loading.value = false
  }
}

const openCreateModal = (): void => {
  isEditing.value = false
  formData.value = {
    name: '',
    index_pattern: '',
    enabled: true,
  }
  currentSource.value = null
  modalError.value = null
  showModal.value = true
}

const openEditModal = (source: Source): void => {
  isEditing.value = true
  formData.value = {
    name: source.name,
    index_pattern: source.index_pattern,
    enabled: source.enabled,
  }
  currentSource.value = source
  modalError.value = null
  showModal.value = true
}

const closeModal = (): void => {
  showModal.value = false
  formData.value = { name: '', index_pattern: '', enabled: true }
  currentSource.value = null
  modalError.value = null
}

const saveSource = async (): Promise<void> => {
  saving.value = true
  modalError.value = null
  try {
    if (isEditing.value && currentSource.value) {
      await publisherApi.sources.update(currentSource.value.id, formData.value as UpdateSourceRequest)
    } else {
      await publisherApi.sources.create(formData.value)
    }
    closeModal()
    await loadSources()
  } catch (err) {
    const axiosError = err as { response?: { data?: { error?: string } } }
    modalError.value = axiosError.response?.data?.error || 'Failed to save source'
  } finally {
    saving.value = false
  }
}

const deleteSource = async (source: Source): Promise<void> => {
  if (!confirm(`Are you sure you want to delete source "${source.name}"?`)) {
    return
  }

  try {
    await publisherApi.sources.delete(source.id)
    await loadSources()
  } catch (err) {
    const axiosError = err as { response?: { data?: { error?: string } } }
    error.value = axiosError.response?.data?.error || 'Failed to delete source'
  }
}

const formatDate = (dateString: string): string => {
  return new Date(dateString).toLocaleString()
}

onMounted(() => {
  loadSources()
})
</script>

