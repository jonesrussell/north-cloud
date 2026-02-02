<template>
  <div>
    <PageHeader
      title="Channels"
      subtitle="Manage Layer 2 custom channels with filtering rules"
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
                  Redis Channel
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Rules
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
                v-for="channel in channels"
                :key="channel.id"
                class="hover:bg-gray-50"
              >
                <td class="px-6 py-4">
                  <div class="text-sm font-medium text-gray-900">
                    {{ channel.name }}
                  </div>
                  <div class="text-xs text-gray-500">
                    {{ channel.slug }}
                  </div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                  <code class="text-sm text-blue-600 bg-blue-50 px-2 py-1 rounded">
                    {{ channel.redis_channel }}
                  </code>
                </td>
                <td class="px-6 py-4">
                  <div class="text-xs space-y-1">
                    <div
                      v-if="channel.rules.include_topics?.length"
                      class="text-green-600"
                    >
                      Include: {{ channel.rules.include_topics.join(', ') }}
                    </div>
                    <div
                      v-if="channel.rules.exclude_topics?.length"
                      class="text-red-600"
                    >
                      Exclude: {{ channel.rules.exclude_topics.join(', ') }}
                    </div>
                    <div
                      v-if="channel.rules.min_quality_score"
                      class="text-gray-600"
                    >
                      Min Quality: {{ channel.rules.min_quality_score }}
                    </div>
                    <div
                      v-if="!hasRules(channel)"
                      class="text-gray-400 italic"
                    >
                      No rules (matches all)
                    </div>
                  </div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                  <StatusBadge
                    :status="channel.enabled ? 'enabled' : 'disabled'"
                  />
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm font-medium">
                  <button
                    class="text-green-600 hover:text-green-900 mr-3"
                    @click="previewChannel(channel)"
                  >
                    Preview
                  </button>
                  <button
                    class="text-blue-600 hover:text-blue-900 mr-3"
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
        <div class="relative bg-white rounded-lg shadow-xl max-w-lg w-full p-6">
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
            <div class="space-y-4">
              <!-- Basic Info -->
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">
                  Name *
                </label>
                <input
                  v-model="formData.name"
                  type="text"
                  placeholder="e.g., StreetCode Crime Feed"
                  required
                  class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                >
              </div>

              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">
                  Slug *
                </label>
                <input
                  v-model="formData.slug"
                  type="text"
                  placeholder="e.g., streetcode-crime-feed"
                  required
                  class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                >
                <p class="mt-1 text-xs text-gray-500">
                  URL-safe identifier (lowercase, hyphens)
                </p>
              </div>

              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">
                  Redis Channel *
                </label>
                <input
                  v-model="formData.redis_channel"
                  type="text"
                  placeholder="e.g., streetcode:crime_feed"
                  required
                  class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                >
                <p class="mt-1 text-xs text-gray-500">
                  Redis pub/sub channel name
                </p>
              </div>

              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">
                  Description
                </label>
                <textarea
                  v-model="formData.description"
                  placeholder="Description of what content this channel contains"
                  rows="2"
                  class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                />
              </div>

              <!-- Rules Section -->
              <div class="border-t pt-4">
                <h3 class="text-sm font-medium text-gray-900 mb-3">
                  Filtering Rules
                </h3>

                <div>
                  <label class="block text-sm font-medium text-gray-700 mb-1">
                    Include Topics
                  </label>
                  <input
                    v-model="rulesInput.include_topics"
                    type="text"
                    placeholder="violent_crime, property_crime"
                    class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                  >
                  <p class="mt-1 text-xs text-gray-500">
                    Comma-separated list of topics to include
                  </p>
                </div>

                <div class="mt-3">
                  <label class="block text-sm font-medium text-gray-700 mb-1">
                    Exclude Topics
                  </label>
                  <input
                    v-model="rulesInput.exclude_topics"
                    type="text"
                    placeholder="criminal_justice"
                    class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                  >
                  <p class="mt-1 text-xs text-gray-500">
                    Comma-separated list of topics to exclude
                  </p>
                </div>

                <div class="mt-3">
                  <label class="block text-sm font-medium text-gray-700 mb-1">
                    Minimum Quality Score
                  </label>
                  <input
                    v-model.number="rulesInput.min_quality_score"
                    type="number"
                    min="0"
                    max="100"
                    placeholder="50"
                    class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                  >
                  <p class="mt-1 text-xs text-gray-500">
                    0-100, articles below this score are excluded
                  </p>
                </div>
              </div>

              <div>
                <label class="flex items-center">
                  <input
                    v-model="formData.enabled"
                    type="checkbox"
                    class="mr-2 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                  >
                  <span class="text-sm text-gray-700">Enabled</span>
                </label>
              </div>
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

    <!-- Preview Modal -->
    <div
      v-if="showPreviewModal"
      class="fixed inset-0 z-50 overflow-y-auto"
      @click.self="closePreviewModal"
    >
      <div class="flex items-center justify-center min-h-screen px-4">
        <div
          class="fixed inset-0 bg-gray-500 bg-opacity-75 transition-opacity"
          @click="closePreviewModal"
        />
        <div class="relative bg-white rounded-lg shadow-xl max-w-2xl w-full p-6">
          <div class="flex justify-between items-center mb-4">
            <h2 class="text-xl font-semibold text-gray-900">
              Channel Preview
            </h2>
            <button
              class="text-gray-400 hover:text-gray-500"
              @click="closePreviewModal"
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

          <LoadingSpinner
            v-if="previewLoading"
            text="Loading preview..."
          />

          <div
            v-else-if="previewData"
            class="space-y-4"
          >
            <div class="bg-gray-50 rounded-lg p-4">
              <h3 class="font-medium text-gray-900 mb-2">
                {{ previewData.channel.name }}
              </h3>
              <p class="text-sm text-gray-500">
                {{ previewData.channel.description || 'No description' }}
              </p>
              <code class="text-xs text-blue-600 bg-blue-50 px-2 py-1 rounded mt-2 inline-block">
                {{ previewData.channel.redis_channel }}
              </code>
            </div>

            <div class="grid grid-cols-2 gap-4">
              <div class="bg-blue-50 rounded-lg p-4">
                <div class="text-2xl font-bold text-blue-700">
                  {{ previewData.matching_count }}
                </div>
                <div class="text-sm text-blue-600">
                  Matching Articles
                </div>
              </div>
              <div class="bg-gray-50 rounded-lg p-4">
                <div class="text-2xl font-bold text-gray-700">
                  v{{ previewData.rules_summary.rules_version }}
                </div>
                <div class="text-sm text-gray-600">
                  Rules Version
                </div>
              </div>
            </div>

            <div class="bg-yellow-50 rounded-lg p-4">
              <p class="text-sm text-yellow-700">
                {{ previewData.note }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { publisherApi } from '../../api/client'
import type {
  Channel,
  ChannelRules,
  CreateChannelRequest,
  UpdateChannelRequest,
  ChannelPreviewResponse,
} from '../../types/publisher'
import { PageHeader, LoadingSpinner, ErrorAlert, StatusBadge } from '../../components/common'

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
  slug: '',
  redis_channel: '',
  description: '',
  enabled: true,
})
const rulesInput = ref({
  include_topics: '',
  exclude_topics: '',
  min_quality_score: 0,
})
const currentChannel = ref<Channel | null>(null)

const showPreviewModal = ref(false)
const previewLoading = ref(false)
const previewData = ref<ChannelPreviewResponse | null>(null)

const hasRules = (channel: Channel): boolean => {
  const rules = channel.rules
  return !!(
    rules.include_topics?.length ||
    rules.exclude_topics?.length ||
    rules.min_quality_score ||
    rules.content_types?.length
  )
}

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

const parseTopicsInput = (input: string): string[] => {
  if (!input.trim()) return []
  return input
    .split(',')
    .map((t) => t.trim())
    .filter((t) => t.length > 0)
}

const buildRulesFromInput = (): ChannelRules => {
  const rules: ChannelRules = {}

  const includeTopics = parseTopicsInput(rulesInput.value.include_topics)
  if (includeTopics.length > 0) {
    rules.include_topics = includeTopics
  }

  const excludeTopics = parseTopicsInput(rulesInput.value.exclude_topics)
  if (excludeTopics.length > 0) {
    rules.exclude_topics = excludeTopics
  }

  if (rulesInput.value.min_quality_score > 0) {
    rules.min_quality_score = rulesInput.value.min_quality_score
  }

  return rules
}

const openCreateModal = (): void => {
  isEditing.value = false
  formData.value = {
    name: '',
    slug: '',
    redis_channel: '',
    description: '',
    enabled: true,
  }
  rulesInput.value = {
    include_topics: '',
    exclude_topics: '',
    min_quality_score: 0,
  }
  currentChannel.value = null
  modalError.value = null
  showModal.value = true
}

const openEditModal = (channel: Channel): void => {
  isEditing.value = true
  formData.value = {
    name: channel.name,
    slug: channel.slug,
    redis_channel: channel.redis_channel,
    description: channel.description,
    enabled: channel.enabled,
  }
  rulesInput.value = {
    include_topics: channel.rules.include_topics?.join(', ') || '',
    exclude_topics: channel.rules.exclude_topics?.join(', ') || '',
    min_quality_score: channel.rules.min_quality_score || 0,
  }
  currentChannel.value = channel
  modalError.value = null
  showModal.value = true
}

const closeModal = (): void => {
  showModal.value = false
  formData.value = { name: '', slug: '', redis_channel: '', description: '', enabled: true }
  rulesInput.value = { include_topics: '', exclude_topics: '', min_quality_score: 0 }
  currentChannel.value = null
  modalError.value = null
}

const saveChannel = async (): Promise<void> => {
  saving.value = true
  modalError.value = null
  try {
    const rules = buildRulesFromInput()
    const requestData = {
      ...formData.value,
      rules,
    }

    if (isEditing.value && currentChannel.value) {
      await publisherApi.channels.update(currentChannel.value.id, requestData as UpdateChannelRequest)
    } else {
      await publisherApi.channels.create(requestData)
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

const previewChannel = async (channel: Channel): Promise<void> => {
  showPreviewModal.value = true
  previewLoading.value = true
  previewData.value = null

  try {
    const response = await publisherApi.channels.preview(channel.id)
    previewData.value = response.data
  } catch (err) {
    const axiosError = err as { response?: { data?: { error?: string } } }
    error.value = axiosError.response?.data?.error || 'Failed to load preview'
    showPreviewModal.value = false
  } finally {
    previewLoading.value = false
  }
}

const closePreviewModal = (): void => {
  showPreviewModal.value = false
  previewData.value = null
}

onMounted(() => {
  loadChannels()
})
</script>
