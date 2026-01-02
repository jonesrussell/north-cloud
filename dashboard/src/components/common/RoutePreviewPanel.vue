<template>
  <div class="bg-white border border-gray-200 rounded-lg p-6">
    <div class="flex items-center justify-between mb-4">
      <h3 class="text-lg font-medium text-gray-900">
        Preview Published Articles
      </h3>
      <button
        v-if="!loading && !error"
        class="text-sm text-blue-600 hover:text-blue-800 font-medium"
        @click="refresh"
      >
        Refresh Preview
      </button>
    </div>

    <!-- Loading State -->
    <div
      v-if="loading"
      class="flex flex-col items-center justify-center py-8"
    >
      <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
      <p class="mt-3 text-sm text-gray-600">
        Loading preview...
      </p>
    </div>

    <!-- Error State -->
    <div
      v-else-if="error"
      class="bg-red-50 border border-red-200 rounded-lg p-4"
    >
      <div class="flex">
        <ExclamationCircleIcon class="w-5 h-5 text-red-600 mt-0.5" />
        <div class="ml-3">
          <p class="text-sm text-red-700">
            {{ error }}
          </p>
        </div>
      </div>
    </div>

    <!-- Preview Results -->
    <div v-else>
      <!-- Summary -->
      <div class="bg-blue-50 border border-blue-200 rounded-lg p-4 mb-4">
        <div class="flex items-start">
          <InformationCircleIcon class="w-5 h-5 text-blue-600 mt-0.5" />
          <div class="ml-3 flex-1">
            <p class="text-sm font-medium text-blue-900">
              Estimated Publishing Volume
            </p>
            <p class="mt-1 text-lg font-bold text-blue-600">
              ~{{ estimatedCount }} articles/day
            </p>
            <p
              v-if="estimatedCount === 0"
              class="mt-1 text-sm text-blue-700"
            >
              ⚠️ No articles match these filters. Adjust your quality threshold or topics.
            </p>
          </div>
        </div>
      </div>

      <!-- Sample Articles Table -->
      <div
        v-if="sampleArticles.length > 0"
        class="overflow-hidden border border-gray-200 rounded-lg"
      >
        <table class="min-w-full divide-y divide-gray-200">
          <thead class="bg-gray-50">
            <tr>
              <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Title
              </th>
              <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Quality
              </th>
              <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Topics
              </th>
              <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Date
              </th>
            </tr>
          </thead>
          <tbody class="bg-white divide-y divide-gray-200">
            <tr
              v-for="(article, index) in sampleArticles"
              :key="index"
              class="hover:bg-gray-50"
            >
              <td class="px-4 py-3">
                <div class="text-sm font-medium text-gray-900 truncate max-w-md">
                  {{ article.title || 'Untitled' }}
                </div>
              </td>
              <td class="px-4 py-3 whitespace-nowrap">
                <div class="flex items-center">
                  <div
                    :class="[
                      'text-xs font-medium px-2 py-1 rounded-full',
                      article.quality_score >= 80
                        ? 'bg-green-100 text-green-800'
                        : article.quality_score >= 60
                        ? 'bg-blue-100 text-blue-800'
                        : article.quality_score >= 40
                        ? 'bg-yellow-100 text-yellow-800'
                        : 'bg-red-100 text-red-800'
                    ]"
                  >
                    {{ article.quality_score || 0 }}
                  </div>
                </div>
              </td>
              <td class="px-4 py-3">
                <div class="flex flex-wrap gap-1">
                  <span
                    v-for="(topic, idx) in (article.topics || []).slice(0, 3)"
                    :key="idx"
                    class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-800"
                  >
                    {{ topic }}
                  </span>
                  <span
                    v-if="(article.topics || []).length > 3"
                    class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-600"
                  >
                    +{{ (article.topics || []).length - 3 }}
                  </span>
                </div>
              </td>
              <td class="px-4 py-3 whitespace-nowrap text-sm text-gray-500">
                {{ article.published_date ? new Date(article.published_date).toLocaleDateString() : 'N/A' }}
              </td>
            </tr>
          </tbody>
        </table>

        <div class="bg-gray-50 px-4 py-3 border-t border-gray-200">
          <p class="text-xs text-gray-600">
            Showing first {{ sampleArticles.length }} articles. Total matching: {{ estimatedCount }}
          </p>
        </div>
      </div>

      <!-- Empty State -->
      <div
        v-else
        class="text-center py-8 bg-gray-50 rounded-lg border border-gray-200"
      >
        <DocumentTextIcon class="mx-auto h-12 w-12 text-gray-400" />
        <p class="mt-2 text-sm text-gray-600">
          No articles match the current filters
        </p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import {
  ExclamationCircleIcon,
  InformationCircleIcon,
  DocumentTextIcon,
} from '@heroicons/vue/24/outline'

interface Article {
  title?: string
  quality_score?: number
  topics?: string[]
  published_date?: string
}

interface Props {
  sourceId?: string
  minQualityScore?: number
  topics?: string[]
  autoRefresh?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  sourceId: undefined,
  minQualityScore: 50,
  topics: () => [],
  autoRefresh: true,
})

const emit = defineEmits<{
  (e: 'refresh', filters: { sourceId?: string; minQualityScore: number; topics: string[] }): void
}>()

const loading = ref(false)
const error = ref<string | null>(null)
const estimatedCount = ref(0)
const sampleArticles = ref<Article[]>([])

// Watch for filter changes and auto-refresh
if (props.autoRefresh) {
  watch(
    () => [props.sourceId, props.minQualityScore, props.topics],
    () => {
      refresh()
    },
    { deep: true }
  )
}

function refresh() {
  emit('refresh', {
    sourceId: props.sourceId,
    minQualityScore: props.minQualityScore,
    topics: props.topics,
  })
}

function setLoading(isLoading: boolean) {
  loading.value = isLoading
  if (isLoading) {
    error.value = null
  }
}

function setResults(count: number, articles: Article[]) {
  estimatedCount.value = count
  sampleArticles.value = articles
  loading.value = false
  error.value = null
}

function setError(errorMessage: string) {
  error.value = errorMessage
  loading.value = false
  estimatedCount.value = 0
  sampleArticles.value = []
}

defineExpose({
  refresh,
  setLoading,
  setResults,
  setError,
})
</script>
