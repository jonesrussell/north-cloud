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
      <div class="relative inline-block w-full max-w-4xl px-4 pt-5 pb-4 overflow-hidden text-left align-bottom transition-all transform bg-white rounded-lg shadow-xl sm:my-8 sm:align-middle sm:p-6">
        <!-- Header -->
        <div class="flex items-center justify-between mb-6">
          <div>
            <h2 class="text-2xl font-bold text-gray-900">
              {{ title }}
            </h2>
            <p class="mt-1 text-sm text-gray-600">
              {{ subtitle }}
            </p>
          </div>
          <button
            class="text-gray-400 hover:text-gray-500 focus:outline-none"
            @click="close"
          >
            <span class="sr-only">Close</span>
            <XMarkIcon class="w-6 h-6" />
          </button>
        </div>

        <!-- Loading State -->
        <div
          v-if="loading"
          class="flex flex-col items-center justify-center py-12"
        >
          <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600" />
          <p class="mt-4 text-sm text-gray-600">
            {{ loadingMessage || 'Running test...' }}
          </p>
        </div>

        <!-- Results -->
        <div
          v-else-if="results"
          class="space-y-6"
        >
          <!-- Summary Stats -->
          <div class="grid grid-cols-3 gap-4">
            <div class="bg-blue-50 rounded-lg p-4">
              <div class="flex items-center">
                <DocumentTextIcon class="w-8 h-8 text-blue-600" />
                <div class="ml-3">
                  <p class="text-sm font-medium text-blue-900">
                    Articles Found
                  </p>
                  <p class="text-2xl font-bold text-blue-600">
                    {{ results.articles_found || 0 }}
                  </p>
                </div>
              </div>
            </div>

            <div class="bg-green-50 rounded-lg p-4">
              <div class="flex items-center">
                <CheckCircleIcon class="w-8 h-8 text-green-600" />
                <div class="ml-3">
                  <p class="text-sm font-medium text-green-900">
                    Success Rate
                  </p>
                  <p class="text-2xl font-bold text-green-600">
                    {{ results.success_rate || 0 }}%
                  </p>
                </div>
              </div>
            </div>

            <div class="bg-yellow-50 rounded-lg p-4">
              <div class="flex items-center">
                <ExclamationTriangleIcon class="w-8 h-8 text-yellow-600" />
                <div class="ml-3">
                  <p class="text-sm font-medium text-yellow-900">
                    Warnings
                  </p>
                  <p class="text-2xl font-bold text-yellow-600">
                    {{ results.warnings?.length || 0 }}
                  </p>
                </div>
              </div>
            </div>
          </div>

          <!-- Warnings -->
          <div
            v-if="results.warnings && results.warnings.length > 0"
            class="bg-yellow-50 border border-yellow-200 rounded-lg p-4"
          >
            <div class="flex">
              <ExclamationTriangleIcon class="w-5 h-5 text-yellow-600 mt-0.5" />
              <div class="ml-3 flex-1">
                <h3 class="text-sm font-medium text-yellow-800">
                  Warnings
                </h3>
                <ul class="mt-2 text-sm text-yellow-700 list-disc list-inside space-y-1">
                  <li
                    v-for="(warning, index) in results.warnings"
                    :key="index"
                  >
                    {{ warning }}
                  </li>
                </ul>
              </div>
            </div>
          </div>

          <!-- Sample Articles -->
          <div v-if="results.sample_articles && results.sample_articles.length > 0">
            <h3 class="text-lg font-medium text-gray-900 mb-3">
              Sample Articles
            </h3>
            <div class="space-y-3">
              <div
                v-for="(article, index) in results.sample_articles"
                :key="index"
                class="border border-gray-200 rounded-lg p-4 hover:bg-gray-50"
              >
                <div class="flex items-start justify-between">
                  <div class="flex-1 min-w-0">
                    <h4 class="text-sm font-medium text-gray-900 truncate">
                      {{ article.title || 'Untitled' }}
                    </h4>
                    <p
                      v-if="article.body"
                      class="mt-1 text-sm text-gray-600 line-clamp-2"
                    >
                      {{ article.body.substring(0, 150) }}{{ article.body.length > 150 ? '...' : '' }}
                    </p>
                    <div class="mt-2 flex items-center space-x-4 text-xs text-gray-500">
                      <span v-if="article.published_date">
                        📅 {{ new Date(article.published_date).toLocaleDateString() }}
                      </span>
                      <span v-if="article.author">
                        ✍️ {{ article.author }}
                      </span>
                      <span v-if="article.quality_score">
                        ⭐ Quality: {{ article.quality_score }}
                      </span>
                    </div>
                  </div>
                  <a
                    v-if="article.url"
                    :href="article.url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="ml-4 text-blue-600 hover:text-blue-800"
                  >
                    <ArrowTopRightOnSquareIcon class="w-5 h-5" />
                  </a>
                </div>
              </div>
            </div>
          </div>

          <!-- No Results -->
          <div
            v-else
            class="text-center py-8 bg-gray-50 rounded-lg"
          >
            <DocumentTextIcon class="mx-auto h-12 w-12 text-gray-400" />
            <p class="mt-2 text-sm text-gray-600">
              No articles found. Check your selectors or URL.
            </p>
          </div>
        </div>

        <!-- Error State -->
        <div
          v-else-if="error"
          class="bg-red-50 border border-red-200 rounded-lg p-4"
        >
          <div class="flex">
            <XCircleIcon class="w-5 h-5 text-red-600 mt-0.5" />
            <div class="ml-3">
              <h3 class="text-sm font-medium text-red-800">
                Test Failed
              </h3>
              <p class="mt-2 text-sm text-red-700">
                {{ error }}
              </p>
            </div>
          </div>
        </div>

        <!-- Actions -->
        <div class="mt-6 flex justify-end gap-3">
          <button
            v-if="results && onSave"
            type="button"
            class="px-4 py-2 bg-green-600 text-white rounded-md hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-green-500"
            @click="handleSave"
          >
            Looks Good! Save Configuration
          </button>
          <button
            type="button"
            class="px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500"
            @click="close"
          >
            {{ results ? 'Close' : 'Cancel' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import {
  XMarkIcon,
  DocumentTextIcon,
  ExclamationTriangleIcon,
  CheckCircleIcon,
  XCircleIcon,
  ArrowTopRightOnSquareIcon,
} from '@heroicons/vue/24/outline'

interface TestResults {
  articles_found: number
  success_rate: number
  warnings?: string[]
  sample_articles?: Array<{
    title?: string
    body?: string
    url?: string
    published_date?: string
    author?: string
    quality_score?: number
  }>
}

interface Props {
  title?: string
  subtitle?: string
  loadingMessage?: string
  onSave?: () => void
}

const props = withDefaults(defineProps<Props>(), {
  title: 'Test Results',
  subtitle: 'Review the test results before saving',
  loadingMessage: 'Running test...',
  onSave: undefined,
})

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'save'): void
}>()

const isOpen = ref(false)
const loading = ref(false)
const results = ref<TestResults | null>(null)
const error = ref<string | null>(null)

function open(testResults?: TestResults) {
  isOpen.value = true
  loading.value = false
  results.value = testResults || null
  error.value = null
}

function setLoading(isLoading: boolean, message?: string) {
  loading.value = isLoading
  if (message) {
    props.loadingMessage
  }
}

function setResults(testResults: TestResults) {
  results.value = testResults
  loading.value = false
  error.value = null
}

function setError(errorMessage: string) {
  error.value = errorMessage
  loading.value = false
  results.value = null
}

function close() {
  isOpen.value = false
  emit('close')
}

function handleSave() {
  emit('save')
  close()
}

defineExpose({
  open,
  setLoading,
  setResults,
  setError,
  close,
})
</script>
