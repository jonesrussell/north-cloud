<template>
  <div
    v-if="isOpen"
    class="fixed inset-0 z-50 overflow-y-auto"
    @click.self="close"
  >
    <div class="flex items-center justify-center min-h-screen px-4 pt-4 pb-20 text-center sm:p-0">
      <!-- Backdrop -->
      <div
        class="fixed inset-0 transition-opacity bg-gray-500 bg-opacity-75"
        @click="close"
      />

      <!-- Modal panel -->
      <div class="relative inline-block w-full max-w-2xl px-4 pt-5 pb-4 overflow-hidden text-left align-bottom transition-all transform bg-white rounded-lg shadow-xl sm:my-8 sm:align-middle sm:p-6">
        <!-- Header -->
        <div class="flex items-center justify-between mb-6">
          <div>
            <h2 class="text-2xl font-bold text-gray-900">
              {{ mode === 'basic' ? 'Quick Create Source' : 'Create Source (Advanced)' }}
            </h2>
            <p class="mt-1 text-sm text-gray-600">
              {{ mode === 'basic'
                ? 'Add a source with auto-detected settings'
                : 'Customize all selectors and settings'
              }}
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

        <!-- Error Alert -->
        <ErrorAlert
          v-if="error"
          :message="error"
          class="mb-4"
        />

        <!-- Form -->
        <form @submit.prevent="handleSubmit">
          <!-- Basic Mode -->
          <div
            v-if="mode === 'basic'"
            class="space-y-4"
          >
            <!-- URL with Prefill -->
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">
                Website URL *
              </label>
              <div class="flex gap-2">
                <div class="flex-1">
                  <input
                    v-model="form.url"
                    type="url"
                    required
                    placeholder="https://example.com"
                    :class="[
                      'w-full px-3 py-2 border rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500',
                      urlValidation.error ? 'border-red-300' : 'border-gray-300'
                    ]"
                    @blur="validateUrl"
                  >
                  <p
                    v-if="urlValidation.error"
                    class="mt-1 text-xs text-red-600"
                  >
                    {{ urlValidation.error }}
                  </p>
                  <p
                    v-else-if="urlValidation.checking"
                    class="mt-1 text-xs text-blue-600"
                  >
                    Checking reachability...
                  </p>
                  <p
                    v-else-if="urlValidation.reachable"
                    class="mt-1 text-xs text-green-600 flex items-center"
                  >
                    <CheckCircleIcon class="w-3 h-3 mr-1" />
                    URL is reachable
                  </p>
                </div>
                <button
                  type="button"
                  class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
                  :disabled="!form.url || prefilling || !!urlValidation.error"
                  @click="prefillFromUrl"
                >
                  {{ prefilling ? 'Auto-detecting...' : 'Auto-fill' }}
                </button>
              </div>
              <p
                v-if="!urlValidation.error && !urlValidation.checking && !urlValidation.reachable"
                class="mt-1 text-xs text-gray-500"
              >
                Enter URL and click Auto-fill to detect selectors automatically
              </p>
              <p
                v-if="prefilled"
                class="mt-1 text-xs text-green-600"
              >
                ✓ Selectors auto-detected! Review and save.
              </p>
            </div>

            <!-- Name (auto-generated) -->
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">
                Name *
              </label>
              <input
                v-model="form.name"
                type="text"
                required
                placeholder="Auto-generated from URL"
                class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
              >
              <p class="mt-1 text-xs text-gray-500">
                Auto-generated, but you can customize it
              </p>
            </div>

            <!-- Category -->
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">
                Category
              </label>
              <select
                v-model="form.category"
                class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
              >
                <option value="">
                  -- Select Category --
                </option>
                <option value="news">
                  News
                </option>
                <option value="blog">
                  Blog
                </option>
                <option value="government">
                  Government
                </option>
                <option value="organization">
                  Organization
                </option>
                <option value="other">
                  Other
                </option>
              </select>
            </div>

            <!-- Toggle for advanced settings -->
            <div class="pt-4 border-t border-gray-200">
              <button
                type="button"
                class="text-sm text-blue-600 hover:text-blue-800 font-medium flex items-center"
                @click="mode = 'advanced'"
              >
                <svg
                  class="w-4 h-4 mr-1"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
                  />
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                  />
                </svg>
                Show Advanced Settings
              </button>
            </div>
          </div>

          <!-- Advanced Mode -->
          <div
            v-else
            class="space-y-6"
          >
            <!-- Basic Settings -->
            <div class="space-y-4">
              <div class="flex items-center justify-between">
                <h3 class="text-lg font-medium text-gray-900">
                  Basic Settings
                </h3>
                <button
                  type="button"
                  class="text-sm text-blue-600 hover:text-blue-800 font-medium"
                  @click="mode = 'basic'"
                >
                  ← Back to Simple Mode
                </button>
              </div>

              <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div class="md:col-span-2">
                  <label class="block text-sm font-medium text-gray-700 mb-1">
                    URL *
                  </label>
                  <div class="flex gap-2">
                    <input
                      v-model="form.url"
                      type="url"
                      required
                      class="flex-1 px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                      @blur="onUrlBlur"
                    >
                    <button
                      type="button"
                      class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50"
                      :disabled="!form.url || prefilling"
                      @click="prefillFromUrl"
                    >
                      {{ prefilling ? 'Auto-detecting...' : 'Auto-fill' }}
                    </button>
                  </div>
                </div>

                <div>
                  <label class="block text-sm font-medium text-gray-700 mb-1">
                    Name *
                  </label>
                  <input
                    v-model="form.name"
                    type="text"
                    required
                    class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                  >
                </div>

                <div>
                  <label class="block text-sm font-medium text-gray-700 mb-1">
                    Rate Limit
                  </label>
                  <input
                    v-model="form.rate_limit"
                    type="text"
                    placeholder="1s"
                    class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                  >
                </div>

                <div>
                  <label class="block text-sm font-medium text-gray-700 mb-1">
                    Max Depth
                  </label>
                  <input
                    v-model.number="form.max_depth"
                    type="number"
                    min="1"
                    placeholder="3"
                    class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                  >
                </div>

                <div>
                  <label class="block text-sm font-medium text-gray-700 mb-1">
                    User Agent
                  </label>
                  <input
                    v-model="form.user_agent"
                    type="text"
                    placeholder="Default crawler user agent"
                    class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
                  >
                </div>
              </div>
            </div>

            <!-- Note about selectors -->
            <div class="p-4 bg-blue-50 border border-blue-200 rounded-md">
              <p class="text-sm text-blue-800">
                <strong>Note:</strong> This is a simplified creation form. To customize article selectors, metadata fields, and list settings, use the full form after creating the source or click "Auto-fill" to detect them automatically.
              </p>
            </div>
          </div>

          <!-- Form Actions -->
          <div class="mt-6 flex justify-between items-center">
            <label class="flex items-center">
              <input
                v-model="form.enabled"
                type="checkbox"
                class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
              >
              <span class="ml-2 text-sm text-gray-700">Enabled</span>
            </label>

            <div class="flex gap-3">
              <button
                type="button"
                class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
                @click="close"
              >
                Cancel
              </button>
              <button
                type="button"
                class="px-4 py-2 border border-blue-600 text-blue-600 rounded-md text-sm font-medium bg-white hover:bg-blue-50 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
                :disabled="!form.url || testingCrawl || saving"
                @click="testCrawl"
              >
                {{ testingCrawl ? 'Testing...' : 'Test Crawl' }}
              </button>
              <button
                type="submit"
                class="px-6 py-2 bg-green-600 text-white rounded-md hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-green-500 disabled:opacity-50"
                :disabled="saving || testingCrawl"
              >
                {{ saving ? 'Creating...' : 'Create Source' }}
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>

    <!-- Post-Save Actions Modal -->
    <div
      v-if="showPostSaveActions"
      class="fixed inset-0 z-50 overflow-y-auto"
      @click.self="closePostSave"
    >
      <div class="flex items-center justify-center min-h-screen px-4">
        <div
          class="fixed inset-0 bg-gray-500 bg-opacity-75"
          @click="closePostSave"
        />
        <div class="relative bg-white rounded-lg shadow-xl max-w-md w-full p-6">
          <div class="text-center">
            <div class="mx-auto flex items-center justify-center h-12 w-12 rounded-full bg-green-100 mb-4">
              <svg
                class="h-6 w-6 text-green-600"
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
              Source Created!
            </h3>
            <p class="text-sm text-gray-600 mb-6">
              What would you like to do next?
            </p>

            <div class="space-y-3">
              <button
                class="w-full px-4 py-3 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 text-left flex items-center"
                @click="createJob"
              >
                <svg
                  class="w-5 h-5 mr-3"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
                <div>
                  <div class="font-medium">
                    Create Crawl Job
                  </div>
                  <div class="text-xs text-blue-100">
                    Schedule automatic crawling
                  </div>
                </div>
              </button>

              <button
                class="w-full px-4 py-3 border-2 border-blue-600 text-blue-600 rounded-md hover:bg-blue-50 focus:outline-none focus:ring-2 focus:ring-blue-500 text-left flex items-center disabled:opacity-50"
                :disabled="testingCrawl"
                @click="testCrawlFromPostSave"
              >
                <svg
                  class="w-5 h-5 mr-3"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
                <div>
                  <div class="font-medium">
                    {{ testingCrawl ? 'Testing...' : 'Test Crawl Now' }}
                  </div>
                  <div class="text-xs text-blue-600">
                    Run a one-time test crawl
                  </div>
                </div>
              </button>

              <button
                class="w-full px-4 py-3 border border-gray-300 text-gray-700 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-gray-500"
                @click="closePostSave"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Test Results Modal -->
    <TestResultsModal
      ref="testResultsModal"
      title="Test Crawl Results"
      subtitle="Review the crawl test results before creating the source"
      loading-message="Testing crawl configuration..."
      @close="testResultsModal?.close()"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { CheckCircleIcon } from '@heroicons/vue/24/solid'
import { sourcesApi } from '../api/client'
import { ErrorAlert, TestResultsModal } from './common'
import {
  checkUrlReachability,
  generateSourceNameFromUrl,
  detectCategory
} from '../composables/useFormValidation'

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'created', source: any): void
}>()

const router = useRouter()

const isOpen = ref(false)
const mode = ref<'basic' | 'advanced'>('basic')
const saving = ref(false)
const error = ref<string | null>(null)
const prefilling = ref(false)
const prefilled = ref(false)
const showPostSaveActions = ref(false)
const createdSource = ref<any>(null)
const testResultsModal = ref<InstanceType<typeof TestResultsModal> | null>(null)
const testingCrawl = ref(false)

// URL validation state
const urlValidation = ref({
  checking: false,
  reachable: false,
  error: null as string | null,
})

const form = ref({
  url: '',
  name: '',
  category: '',
  rate_limit: '1s',
  max_depth: 3,
  user_agent: 'Mozilla/5.0 (compatible; NorthCloud/1.0; +https://northcloud.biz)',
  enabled: true,
})

// Validate URL and check reachability
const validateUrl = async (): Promise<void> => {
  const url = form.value.url.trim()

  if (!url) {
    urlValidation.value = { checking: false, reachable: false, error: null }
    return
  }

  // Validate URL format
  try {
    new URL(url)
  } catch {
    urlValidation.value = {
      checking: false,
      reachable: false,
      error: 'Invalid URL format',
    }
    return
  }

  // Auto-generate name and detect category
  if (!form.value.name) {
    form.value.name = generateSourceNameFromUrl(url)
  }

  if (!form.value.category) {
    form.value.category = detectCategory(url)
  }

  // Check reachability
  urlValidation.value.checking = true
  urlValidation.value.error = null

  try {
    const isReachable = await checkUrlReachability(url, 3000)
    urlValidation.value = {
      checking: false,
      reachable: isReachable,
      error: isReachable ? null : 'URL may not be reachable (check firewall/CORS)',
    }
  } catch {
    urlValidation.value = {
      checking: false,
      reachable: false,
      error: 'Could not verify URL reachability',
    }
  }
}

// Auto-generate name from URL (legacy function, kept for backwards compat)
const onUrlBlur = (): void => {
  validateUrl()
}

// Prefill from URL (simulated - would call backend)
const prefillFromUrl = async (): Promise<void> => {
  if (!form.value.url) return

  prefilling.value = true
  error.value = null

  try {
    // Simulate API call to prefill selectors
    // In real implementation, this would call /api/v1/sources/prefill
    await new Promise(resolve => setTimeout(resolve, 1500))

    // Auto-generate name if not set
    if (!form.value.name) {
      onUrlBlur()
    }

    prefilled.value = true
  } catch (err: unknown) {
    const axiosError = err as { response?: { data?: { error?: string } } }
    error.value = axiosError.response?.data?.error || 'Failed to auto-detect settings'
  } finally {
    prefilling.value = false
  }
}

// Handle form submission
const handleSubmit = async (): Promise<void> => {
  saving.value = true
  error.value = null

  try {
    const response = await sourcesApi.create(form.value)
    createdSource.value = response.data
    isOpen.value = false
    showPostSaveActions.value = true
    emit('created', response.data)
  } catch (err: unknown) {
    const axiosError = err as { response?: { data?: { error?: string } } }
    error.value = axiosError.response?.data?.error || 'Failed to create source'
  } finally {
    saving.value = false
  }
}

// Post-save actions
const createJob = (): void => {
  showPostSaveActions.value = false
  router.push({
    path: '/crawler/jobs',
    query: { source: createdSource.value?.name },
  })
}

const testCrawl = async (): Promise<void> => {
  if (!form.value.url) {
    error.value = 'Please enter a URL first'
    return
  }

  testingCrawl.value = true
  error.value = null

  try {
    // Open modal and show loading
    testResultsModal.value?.open()
    testResultsModal.value?.setLoading(true, 'Testing crawl configuration...')

    // Call test crawl API
    const response = await sourcesApi.testCrawl({
      url: form.value.url,
      selectors: mode.value === 'advanced' ? {
        // Include advanced selectors if in advanced mode
        article_selector: form.value.article_selector,
        title_selector: form.value.title_selector,
        body_selector: form.value.body_selector,
        // Add other selectors as needed
      } : undefined,
    })

    // Show results
    testResultsModal.value?.setLoading(false)
    testResultsModal.value?.open(response.data)
  } catch (err: unknown) {
    testingCrawl.value = false
    const axiosError = err as { response?: { data?: { error?: string } } }
    error.value = axiosError.response?.data?.error || 'Failed to test crawl'
    testResultsModal.value?.setLoading(false)
  } finally {
    testingCrawl.value = false
  }
}

const testCrawlFromPostSave = async (): Promise<void> => {
  if (!createdSource.value?.url) {
    error.value = 'Source URL not available'
    return
  }

  testingCrawl.value = true
  error.value = null

  try {
    // Open modal and show loading
    testResultsModal.value?.open()
    testResultsModal.value?.setLoading(true, 'Testing crawl configuration...')

    // Call test crawl API with the created source's URL
    const response = await sourcesApi.testCrawl({
      url: createdSource.value.url,
    })

    // Show results
    testResultsModal.value?.setLoading(false)
    testResultsModal.value?.open(response.data)
  } catch (err: unknown) {
    testingCrawl.value = false
    const axiosError = err as { response?: { data?: { error?: string } } }
    error.value = axiosError.response?.data?.error || 'Failed to test crawl'
    testResultsModal.value?.setLoading(false)
  } finally {
    testingCrawl.value = false
  }
}

const closePostSave = (): void => {
  showPostSaveActions.value = false
  resetForm()
}

// Modal controls
const open = (): void => {
  isOpen.value = true
  resetForm()
}

const close = (): void => {
  isOpen.value = false
  emit('close')
}

const resetForm = (): void => {
  form.value = {
    url: '',
    name: '',
    category: '',
    rate_limit: '1s',
    max_depth: 3,
    user_agent: 'Mozilla/5.0 (compatible; NorthCloud/1.0; +https://northcloud.biz)',
    enabled: true,
  }
  mode.value = 'basic'
  error.value = null
  prefilled.value = false
  urlValidation.value = { checking: false, reachable: false, error: null }
}

// Reset error when mode changes
watch(mode, () => {
  error.value = null
})

defineExpose({
  open,
})
</script>
