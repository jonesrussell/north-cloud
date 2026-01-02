<template>
  <div>
    <PageHeader
      title="Channels"
      subtitle="Manage Redis pub/sub channels for article distribution"
    />

    <div class="bg-white shadow rounded-lg p-6">
      <div class="flex justify-between items-center mb-4">
        <div>
          <label class="flex items-center text-sm text-gray-700">
            <input
              v-model="enabledOnly"
              type="checkbox"
              class="mr-2 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
              @change="loadChannels"
            >
            Show enabled only
          </label>
        </div>
        <button
          class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500"
          @click="openCreateModal"
        >
          + Add Channel
        </button>
      </div>

      <LoadingSpinner
        v-if="loading"
        text="Loading channels..."
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
                  Description
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
                v-for="channel in channels"
                :key="channel.id"
                class="hover:bg-gray-50"
              >
                <td class="px-6 py-4 whitespace-nowrap">
                  <code class="text-sm text-gray-900">{{ channel.name }}</code>
                </td>
                <td class="px-6 py-4">
                  <div class="text-sm text-gray-500">
                    {{ channel.description || '-' }}
                  </div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                  <StatusBadge
                    :status="channel.enabled ? 'enabled' : 'disabled'"
                  />
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  {{ formatDate(channel.created_at) }}
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm font-medium">
                  <button
                    class="text-green-600 hover:text-green-900 mr-4"
                    @click="testPublish(channel)"
                  >
                    Test Publish
                  </button>
                  <button
                    class="text-blue-600 hover:text-blue-900 mr-4"
                    @click="openEditModal(channel)"
                  >
                    Edit
                  </button>
                  <button
                    class="text-red-600 hover:text-red-900"
                    @click="deleteChannel(channel)"
                  >
                    Delete
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div
          v-if="!loading && channels.length === 0"
          class="text-center py-12 text-gray-500"
        >
          No channels found. Click "Add Channel" to create one.
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
              {{ isEditing ? 'Edit Channel' : 'Create Channel' }}
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

          <form @submit.prevent="saveChannel">
            <div class="mb-4">
              <label class="block text-sm font-medium text-gray-700 mb-1">
                Name *
              </label>
              <input
                v-model="formData.name"
                type="text"
                placeholder="e.g., articles:crime"
                required
                class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
              >
              <p class="mt-1 text-xs text-gray-500">
                Redis pub/sub channel name (e.g., articles:crime, articles:news)
              </p>
            </div>

            <div class="mb-4">
              <label class="block text-sm font-medium text-gray-700 mb-1">
                Description
              </label>
              <textarea
                v-model="formData.description"
                placeholder="Description of what content this channel contains"
                rows="3"
                class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
              />
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

    <!-- Test Results Modal -->
    <TestResultsModal
      ref="testResultsModal"
      title="Test Publish Results"
      subtitle="Preview articles that would be published to this channel"
      loading-message="Testing publish configuration..."
      @close="testResultsModal?.close()"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { publisherApi } from '../../api/client'
import type { Channel, CreateChannelRequest, UpdateChannelRequest, PreviewArticle } from '../../types/publisher'
import { PageHeader, LoadingSpinner, ErrorAlert, StatusBadge, TestResultsModal } from '../../components/common'

const channels = ref<Channel[]>([])
const loading = ref(false)
const error = ref<string | null>(null)
const enabledOnly = ref(false)

const showModal = ref(false)
const isEditing = ref(false)
const modalError = ref<string | null>(null)
const saving = ref(false)
const formData = ref<CreateChannelRequest>({
  name: '',
  description: '',
  enabled: true,
})
const currentChannel = ref<Channel | null>(null)
const testResultsModal = ref<InstanceType<typeof TestResultsModal> | null>(null)
const testingPublish = ref(false)

const loadChannels = async (): Promise<void> => {
  loading.value = true
  error.value = null
  try {
    const response = await publisherApi.channels.list(enabledOnly.value)
    channels.value = response.data.channels || []
  } catch (err) {
    const axiosError = err as { response?: { data?: { error?: string } } }
    error.value = axiosError.response?.data?.error || 'Failed to load channels'
  } finally {
    loading.value = false
  }
}

const openCreateModal = (): void => {
  isEditing.value = false
  formData.value = {
    name: '',
    description: '',
    enabled: true,
  }
  currentChannel.value = null
  modalError.value = null
  showModal.value = true
}

const openEditModal = (channel: Channel): void => {
  isEditing.value = true
  formData.value = {
    name: channel.name,
    description: channel.description,
    enabled: channel.enabled,
  }
  currentChannel.value = channel
  modalError.value = null
  showModal.value = true
}

const closeModal = (): void => {
  showModal.value = false
  formData.value = { name: '', description: '', enabled: true }
  currentChannel.value = null
  modalError.value = null
}

const saveChannel = async (): Promise<void> => {
  saving.value = true
  modalError.value = null
  try {
    if (isEditing.value && currentChannel.value) {
      await publisherApi.channels.update(currentChannel.value.id, formData.value as UpdateChannelRequest)
    } else {
      await publisherApi.channels.create(formData.value)
    }
    closeModal()
    await loadChannels()
  } catch (err) {
    const axiosError = err as { response?: { data?: { error?: string } } }
    modalError.value = axiosError.response?.data?.error || 'Failed to save channel'
  } finally {
    saving.value = false
  }
}

const deleteChannel = async (channel: Channel): Promise<void> => {
  if (!confirm(`Are you sure you want to delete channel "${channel.name}"?`)) {
    return
  }

  try {
    await publisherApi.channels.delete(channel.id)
    await loadChannels()
  } catch (err) {
    const axiosError = err as { response?: { data?: { error?: string } } }
    error.value = axiosError.response?.data?.error || 'Failed to delete channel'
  }
}

const testPublish = async (channel: Channel): Promise<void> => {
  testingPublish.value = true
  error.value = null

  try {
    // Open modal and show loading
    testResultsModal.value?.open()
    testResultsModal.value?.setLoading(true, 'Testing publish configuration...')

    // Call test publish API
    const response = await publisherApi.channels.testPublish(channel.id)

    // Transform response to match TestResultsModal format
    const testResults = {
      articles_found: response.data.estimated_count || 0,
      success_rate: response.data.routes_count > 0 ? 100 : 0,
      warnings: response.data.routes_count === 0
        ? ['No enabled routes found for this channel']
        : [],
      sample_articles: (response.data.sample_articles || []).map((article: PreviewArticle) => ({
        title: article.title,
        url: article.url,
        published_date: article.published_date,
        quality_score: article.quality_score,
        topics: article.topics,
        source: article.source,
      })),
    }

    // Show results
    testResultsModal.value?.setLoading(false)
    testResultsModal.value?.open(testResults)
  } catch (err: unknown) {
    testingPublish.value = false
    const axiosError = err as { response?: { data?: { error?: string } } }
    error.value = axiosError.response?.data?.error || 'Failed to test publish'
    testResultsModal.value?.setLoading(false)
  } finally {
    testingPublish.value = false
  }
}

const formatDate = (dateString: string): string => {
  return new Date(dateString).toLocaleString()
}

onMounted(() => {
  loadChannels()
})
</script>

