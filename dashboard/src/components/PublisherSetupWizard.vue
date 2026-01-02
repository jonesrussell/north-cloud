<template>
  <div
    v-if="isOpen"
    class="fixed inset-0 z-50 overflow-y-auto"
    @click.self="handleBackdropClick"
  >
    <div class="flex items-center justify-center min-h-screen px-4 pt-4 pb-20 text-center sm:p-0">
      <!-- Backdrop -->
      <div
        class="fixed inset-0 transition-opacity bg-gray-500 bg-opacity-75"
        @click="handleBackdropClick"
      />

      <!-- Modal panel -->
      <div class="relative inline-block w-full max-w-3xl px-4 pt-5 pb-4 overflow-hidden text-left align-bottom transition-all transform bg-white rounded-lg shadow-xl sm:my-8 sm:align-middle sm:p-6">
        <!-- Header -->
        <div class="flex items-center justify-between mb-6">
          <div>
            <h2 class="text-2xl font-bold text-gray-900">
              Set Up Publishing
            </h2>
            <p class="mt-1 text-sm text-gray-600">
              Connect a source to a channel in 3 easy steps
            </p>
          </div>
          <button
            class="text-gray-400 hover:text-gray-500 focus:outline-none"
            @click="close"
          >
            <span class="sr-only">Close</span>
            <svg
              class="w-6 h-6"
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

        <!-- Progress indicator -->
        <div class="mb-8">
          <nav aria-label="Progress">
            <ol class="flex items-center">
              <li
                v-for="(step, index) in steps"
                :key="step.id"
                :class="[
                  index !== steps.length - 1 ? 'pr-8 sm:pr-20' : '',
                  'relative'
                ]"
              >
                <div
                  v-if="index !== steps.length - 1"
                  class="absolute inset-0 flex items-center"
                  aria-hidden="true"
                >
                  <div
                    :class="[
                      currentStep > index ? 'bg-blue-600' : 'bg-gray-200',
                      'h-0.5 w-full'
                    ]"
                  />
                </div>
                <div
                  :class="[
                    currentStep === index
                      ? 'border-blue-600 bg-white'
                      : currentStep > index
                        ? 'bg-blue-600 border-blue-600'
                        : 'border-gray-300 bg-white',
                    'relative w-8 h-8 flex items-center justify-center border-2 rounded-full'
                  ]"
                >
                  <span
                    v-if="currentStep > index"
                    class="text-white"
                  >
                    <svg
                      class="w-5 h-5"
                      fill="currentColor"
                      viewBox="0 0 20 20"
                    >
                      <path
                        fill-rule="evenodd"
                        d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z"
                        clip-rule="evenodd"
                      />
                    </svg>
                  </span>
                  <span
                    v-else
                    :class="[
                      currentStep === index ? 'text-blue-600' : 'text-gray-500',
                      'text-sm font-semibold'
                    ]"
                  >
                    {{ index + 1 }}
                  </span>
                </div>
                <span
                  :class="[
                    currentStep === index ? 'text-blue-600 font-semibold' : 'text-gray-500',
                    'absolute -bottom-8 left-1/2 transform -translate-x-1/2 whitespace-nowrap text-sm'
                  ]"
                >
                  {{ step.title }}
                </span>
              </li>
            </ol>
          </nav>
        </div>

        <!-- Error Alert -->
        <ErrorAlert
          v-if="error"
          :message="error"
          class="mb-4"
        />

        <!-- Step Content -->
        <div class="mt-12">
          <!-- Step 1: Select or Create Source -->
          <div v-show="currentStep === 0">
            <h3 class="text-lg font-medium text-gray-900 mb-4">
              Select Content Source
            </h3>
            <p class="text-sm text-gray-600 mb-4">
              Choose an Elasticsearch index that contains the articles you want to publish.
            </p>

            <div class="space-y-4">
              <!-- Existing Source Selection -->
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-2">
                  Select Existing Source
                </label>
                <select
                  v-model="selectedSourceId"
                  class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                  @change="onSourceSelected"
                >
                  <option
                    :value="null"
                  >
                    -- Or create a new source below --
                  </option>
                  <option
                    v-for="source in existingSources"
                    :key="source.id"
                    :value="source.id"
                  >
                    {{ source.name }} ({{ source.index_pattern }})
                  </option>
                </select>
              </div>

              <!-- OR Divider -->
              <div class="relative">
                <div class="absolute inset-0 flex items-center">
                  <div class="w-full border-t border-gray-300" />
                </div>
                <div class="relative flex justify-center text-sm">
                  <span class="px-2 bg-white text-gray-500">OR</span>
                </div>
              </div>

              <!-- New Source Form -->
              <div class="p-4 bg-gray-50 rounded-lg">
                <h4 class="text-sm font-medium text-gray-900 mb-3">
                  Create New Source
                </h4>
                <div class="space-y-3">
                  <div>
                    <label class="block text-sm font-medium text-gray-700 mb-1">
                      Name *
                    </label>
                    <input
                      v-model="newSource.name"
                      type="text"
                      placeholder="e.g., sudbury_com"
                      class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                      :disabled="selectedSourceId !== null"
                    >
                  </div>
                  <div>
                    <label class="block text-sm font-medium text-gray-700 mb-1">
                      Index Pattern *
                    </label>
                    <input
                      v-model="newSource.index_pattern"
                      type="text"
                      placeholder="e.g., sudbury_com_classified_content"
                      class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                      :disabled="selectedSourceId !== null"
                    >
                    <p class="mt-1 text-xs text-gray-500">
                      Elasticsearch index pattern to query
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Step 2: Select or Create Channel -->
          <div v-show="currentStep === 1">
            <h3 class="text-lg font-medium text-gray-900 mb-4">
              Select Destination Channel
            </h3>
            <p class="text-sm text-gray-600 mb-4">
              Choose a Redis pub/sub channel where articles will be published.
            </p>

            <div class="space-y-4">
              <!-- Existing Channel Selection -->
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-2">
                  Select Existing Channel
                </label>
                <select
                  v-model="selectedChannelId"
                  class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                  @change="onChannelSelected"
                >
                  <option :value="null">
                    -- Or create a new channel below --
                  </option>
                  <option
                    v-for="channel in existingChannels"
                    :key="channel.id"
                    :value="channel.id"
                  >
                    {{ channel.name }}{{ channel.description ? ` - ${channel.description}` : '' }}
                  </option>
                </select>
              </div>

              <!-- OR Divider -->
              <div class="relative">
                <div class="absolute inset-0 flex items-center">
                  <div class="w-full border-t border-gray-300" />
                </div>
                <div class="relative flex justify-center text-sm">
                  <span class="px-2 bg-white text-gray-500">OR</span>
                </div>
              </div>

              <!-- New Channel Form -->
              <div class="p-4 bg-gray-50 rounded-lg">
                <h4 class="text-sm font-medium text-gray-900 mb-3">
                  Create New Channel
                </h4>
                <div class="space-y-3">
                  <div>
                    <label class="block text-sm font-medium text-gray-700 mb-1">
                      Name *
                    </label>
                    <input
                      v-model="newChannel.name"
                      type="text"
                      placeholder="e.g., articles:crime"
                      class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                      :disabled="selectedChannelId !== null"
                    >
                    <p class="mt-1 text-xs text-gray-500">
                      Redis pub/sub channel name (e.g., articles:crime, articles:news)
                    </p>
                  </div>
                  <div>
                    <label class="block text-sm font-medium text-gray-700 mb-1">
                      Description
                    </label>
                    <textarea
                      v-model="newChannel.description"
                      rows="2"
                      placeholder="What content does this channel contain?"
                      class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                      :disabled="selectedChannelId !== null"
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Step 3: Configure Route -->
          <div v-show="currentStep === 2">
            <h3 class="text-lg font-medium text-gray-900 mb-4">
              Configure Routing Rules
            </h3>
            <p class="text-sm text-gray-600 mb-4">
              Set quality filters and topic preferences for articles published through this route.
            </p>

            <div class="space-y-4">
              <!-- Route Configuration Summary -->
              <div class="p-4 bg-blue-50 border border-blue-200 rounded-md">
                <h4 class="text-sm font-medium text-blue-900 mb-2">
                  Route Summary
                </h4>
                <div class="text-sm text-blue-800">
                  <p>
                    <strong>From:</strong> {{ getSourceDisplayName() }}
                  </p>
                  <p>
                    <strong>To:</strong> {{ getChannelDisplayName() }}
                  </p>
                </div>
              </div>

              <!-- Quality Score Filter -->
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-2">
                  Minimum Quality Score: {{ route.min_quality_score }}
                </label>
                <input
                  v-model.number="route.min_quality_score"
                  type="range"
                  min="0"
                  max="100"
                  step="5"
                  class="w-full h-2 bg-gray-200 rounded-lg appearance-none cursor-pointer slider"
                >
                <div class="flex justify-between text-xs text-gray-500 mt-1">
                  <span>Low Quality (0)</span>
                  <span>Medium (50)</span>
                  <span>High Quality (100)</span>
                </div>
                <p class="mt-2 text-xs text-gray-500">
                  Only articles with quality score >= {{ route.min_quality_score }} will be published
                </p>
              </div>

              <!-- Topics Filter -->
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">
                  Topics (optional)
                </label>
                <input
                  v-model="topicsInput"
                  type="text"
                  placeholder="e.g., crime, news, local"
                  class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                >
                <p class="mt-1 text-xs text-gray-500">
                  Comma-separated list of topics to filter. Leave empty to publish all topics.
                </p>
              </div>

              <!-- Live Preview -->
              <RoutePreviewPanel
                v-if="route.source_id && route.channel_id"
                ref="routePreviewPanel"
                :source-id="route.source_id.toString()"
                :min-quality-score="route.min_quality_score"
                :topics="topicsInput ? topicsInput.split(',').map(t => t.trim()).filter(t => t.length > 0) : []"
                :auto-refresh="false"
                class="mt-4"
                @refresh="handlePreviewRefresh"
              />
              <div
                v-else
                class="p-4 bg-gray-50 border border-gray-200 rounded-md"
              >
                <p class="text-sm text-gray-600">
                  Preview will appear once source and channel are selected.
                </p>
              </div>
            </div>
          </div>

          <!-- Success State -->
          <div v-show="currentStep === 3">
            <div class="text-center py-8">
              <div class="mx-auto flex items-center justify-center h-16 w-16 rounded-full bg-green-100 mb-4">
                <svg
                  class="h-10 w-10 text-green-600"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M5 13l4 4L19 7"
                  />
                </svg>
              </div>
              <h3 class="text-lg font-medium text-gray-900 mb-2">
                Publishing Route Created!
              </h3>
              <p class="text-sm text-gray-600 mb-6">
                Your route has been successfully configured. The router service will begin publishing articles within the next 5 minutes.
              </p>
              <div class="space-y-3 max-w-md mx-auto">
                <router-link
                  to="/publisher/routes"
                  class="block w-full px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 text-center"
                  @click="close"
                >
                  View All Routes
                </router-link>
                <button
                  class="w-full px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
                  @click="resetAndRestart"
                >
                  Set Up Another Route
                </button>
                <button
                  class="w-full px-4 py-2 text-sm text-gray-600 hover:text-gray-800"
                  @click="close"
                >
                  Close
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Navigation Buttons -->
        <div
          v-if="currentStep < 3"
          class="mt-8 flex justify-between"
        >
          <button
            v-if="currentStep > 0"
            class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
            @click="previousStep"
          >
            Back
          </button>
          <div v-else />

          <button
            class="px-6 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
            :disabled="!canProceed || saving"
            @click="nextStep"
          >
            {{ currentStep === 2 ? (saving ? 'Creating...' : 'Activate Route') : 'Continue' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { publisherApi } from '../api/client'
import type { Source, Channel, CreateSourceRequest, CreateChannelRequest, CreateRouteRequest } from '../types/publisher'
import { ErrorAlert, RoutePreviewPanel } from './common'

const props = defineProps<{
  open?: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'success'): void
}>()

const isOpen = ref(props.open ?? false)
const currentStep = ref(0)
const saving = ref(false)
const error = ref<string | null>(null)

const steps = [
  { id: 'source', title: 'Source' },
  { id: 'channel', title: 'Channel' },
  { id: 'route', title: 'Configure' },
]

// Step 1: Source data
const existingSources = ref<Source[]>([])
const selectedSourceId = ref<number | null>(null)
const newSource = ref<CreateSourceRequest>({
  name: '',
  index_pattern: '',
  enabled: true,
})
const createdSourceId = ref<number | null>(null)

// Step 2: Channel data
const existingChannels = ref<Channel[]>([])
const selectedChannelId = ref<number | null>(null)
const newChannel = ref<CreateChannelRequest>({
  name: '',
  description: '',
  enabled: true,
})
const createdChannelId = ref<number | null>(null)

// Step 3: Route data
const route = ref<CreateRouteRequest>({
  source_id: 0,
  channel_id: 0,
  min_quality_score: 50,
  topics: null,
  enabled: true,
})
const topicsInput = ref('')
const routePreviewPanel = ref<{ refresh: () => void; setLoading: (loading: boolean) => void; setResults: (count: number, articles: any[]) => void; setError: (error: string) => void } | null>(null)

// Load existing sources and channels
const loadExistingSources = async (): Promise<void> => {
  try {
    const response = await publisherApi.sources.list(true) // Only enabled
    existingSources.value = response.data.sources || []
  } catch (err) {
    console.error('Failed to load sources:', err)
  }
}

const loadExistingChannels = async (): Promise<void> => {
  try {
    const response = await publisherApi.channels.list(false) // All channels
    existingChannels.value = response.data.channels || []
  } catch (err) {
    console.error('Failed to load channels:', err)
  }
}

// Validation
const canProceed = computed(() => {
  if (currentStep.value === 0) {
    // Must either select existing source OR fill in new source form
    const hasExistingSource = selectedSourceId.value !== null
    const hasNewSource = newSource.value.name.trim() !== '' && newSource.value.index_pattern.trim() !== ''
    return hasExistingSource || hasNewSource
  }
  if (currentStep.value === 1) {
    // Must either select existing channel OR fill in new channel form
    const hasExistingChannel = selectedChannelId.value !== null
    const hasNewChannel = newChannel.value.name.trim() !== ''
    return hasExistingChannel || hasNewChannel
  }
  if (currentStep.value === 2) {
    return true // All fields have defaults
  }
  return false
})

// Event handlers
const onSourceSelected = (): void => {
  if (selectedSourceId.value !== null) {
    // Clear new source form
    newSource.value = { name: '', index_pattern: '', enabled: true }
  }
}

const onChannelSelected = (): void => {
  if (selectedChannelId.value !== null) {
    // Clear new channel form
    newChannel.value = { name: '', description: '', enabled: true }
  }
}

// Navigation
const nextStep = async (): Promise<void> => {
  error.value = null

  if (currentStep.value === 0) {
    // Step 1 -> 2: Create source if needed
    if (selectedSourceId.value === null) {
      // Create new source
      saving.value = true
      try {
        const response = await publisherApi.sources.create(newSource.value)
        createdSourceId.value = response.data.id
        // Set route source_id for preview
        route.value.source_id = response.data.id
        currentStep.value++
      } catch (err: unknown) {
        const axiosError = err as { response?: { data?: { error?: string } } }
        error.value = axiosError.response?.data?.error || 'Failed to create source'
      } finally {
        saving.value = false
      }
    } else {
      createdSourceId.value = selectedSourceId.value
      // Set route source_id for preview
      route.value.source_id = selectedSourceId.value
      currentStep.value++
    }
  } else if (currentStep.value === 1) {
    // Step 2 -> 3: Create channel if needed
    if (selectedChannelId.value === null) {
      // Create new channel
      saving.value = true
      try {
        const response = await publisherApi.channels.create(newChannel.value)
        createdChannelId.value = response.data.id
        // Set route channel_id for preview
        route.value.channel_id = response.data.id
        currentStep.value++
      } catch (err: unknown) {
        const axiosError = err as { response?: { data?: { error?: string } } }
        error.value = axiosError.response?.data?.error || 'Failed to create channel'
      } finally {
        saving.value = false
      }
    } else {
      createdChannelId.value = selectedChannelId.value
      // Set route channel_id for preview
      route.value.channel_id = selectedChannelId.value
      currentStep.value++
    }
  } else if (currentStep.value === 2) {
    // Step 3 -> Success: Create route
    saving.value = true
    try {
      // Parse topics
      const topics = topicsInput.value
        .split(',')
        .map(t => t.trim())
        .filter(t => t.length > 0)

      const payload: CreateRouteRequest = {
        source_id: createdSourceId.value!,
        channel_id: createdChannelId.value!,
        min_quality_score: route.value.min_quality_score,
        topics: topics.length > 0 ? topics : null,
        enabled: true,
      }

      await publisherApi.routes.create(payload)
      currentStep.value++
      emit('success')
    } catch (err: unknown) {
      const axiosError = err as { response?: { data?: { error?: string } } }
      error.value = axiosError.response?.data?.error || 'Failed to create route'
    } finally {
      saving.value = false
    }
  }
}

const previousStep = (): void => {
  if (currentStep.value > 0) {
    currentStep.value--
    error.value = null
  }
}

const close = (): void => {
  isOpen.value = false
  emit('close')
}

const handleBackdropClick = (): void => {
  if (currentStep.value === 3) {
    // Allow closing on success screen
    close()
  }
}

const resetAndRestart = (): void => {
  // Reset all state
  currentStep.value = 0
  selectedSourceId.value = null
  selectedChannelId.value = null
  createdSourceId.value = null
  createdChannelId.value = null
  newSource.value = { name: '', index_pattern: '', enabled: true }
  newChannel.value = { name: '', description: '', enabled: true }
  route.value = {
    source_id: 0,
    channel_id: 0,
    min_quality_score: 50,
    topics: null,
    enabled: true,
  }
  topicsInput.value = ''
  error.value = null

  // Reload sources and channels
  loadExistingSources()
  loadExistingChannels()
}

// Display helpers
const getSourceDisplayName = (): string => {
  if (selectedSourceId.value) {
    const source = existingSources.value.find(s => s.id === selectedSourceId.value)
    return source ? `${source.name} (${source.index_pattern})` : 'Selected Source'
  }
  return newSource.value.name || 'New Source'
}

const getChannelDisplayName = (): string => {
  if (selectedChannelId.value) {
    const channel = existingChannels.value.find(c => c.id === selectedChannelId.value)
    return channel ? channel.name : 'Selected Channel'
  }
  return newChannel.value.name || 'New Channel'
}

// Public method to open wizard
defineExpose({
  open: () => {
    isOpen.value = true
    loadExistingSources()
    loadExistingChannels()
  },
})

// Handle preview refresh
const handlePreviewRefresh = async (filters: { sourceId?: string; minQualityScore: number; topics: string[] }): Promise<void> => {
  if (!filters.sourceId) {
    routePreviewPanel.value?.setError('Source ID is required')
    return
  }

  routePreviewPanel.value?.setLoading(true)

  try {
    const response = await publisherApi.routes.preview({
      source_id: filters.sourceId,
      min_quality_score: filters.minQualityScore.toString(),
      topics: filters.topics.join(','),
    })

    const articles = (response.data.sample_articles || []).map((article: any) => ({
      title: article.title,
      quality_score: article.quality_score,
      topics: article.topics || [],
      published_date: article.published_date,
    }))

    routePreviewPanel.value?.setResults(response.data.estimated_count || 0, articles)
  } catch (err: unknown) {
    const axiosError = err as { response?: { data?: { error?: string } } }
    routePreviewPanel.value?.setError(
      axiosError.response?.data?.error || 'Failed to load preview'
    )
  }
}

// Watch for route changes and refresh preview
watch(
  () => [route.value.source_id, route.value.min_quality_score, topicsInput.value],
  () => {
    if (route.value.source_id && route.value.channel_id && routePreviewPanel.value) {
      // Small delay to avoid too many API calls
      setTimeout(() => {
        routePreviewPanel.value?.refresh()
      }, 500)
    }
  },
  { deep: true }
)

onMounted(() => {
  if (isOpen.value) {
    loadExistingSources()
    loadExistingChannels()
  }
})
</script>

<style scoped>
.slider::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 20px;
  height: 20px;
  background: #2563eb;
  cursor: pointer;
  border-radius: 50%;
}

.slider::-moz-range-thumb {
  width: 20px;
  height: 20px;
  background: #2563eb;
  cursor: pointer;
  border-radius: 50%;
}
</style>
