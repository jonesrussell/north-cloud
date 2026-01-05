<template>
  <div
    class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 overflow-y-auto"
    @click.self="$emit('close')"
  >
    <div class="bg-white rounded-lg shadow-xl max-w-3xl w-full mx-4 my-8">
      <div class="p-6">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-lg font-medium text-gray-900">
            Edit Document
          </h3>
          <button
            class="text-gray-400 hover:text-gray-500"
            @click="$emit('close')"
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

        <form @submit.prevent="handleSubmit">
          <div class="space-y-4">
            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">
                Title
              </label>
              <input
                v-model="formData.title"
                type="text"
                class="w-full px-3 py-2 border border-gray-300 rounded-md"
              >
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">
                URL
              </label>
              <input
                v-model="formData.url"
                type="url"
                class="w-full px-3 py-2 border border-gray-300 rounded-md"
              >
            </div>

            <div class="grid grid-cols-2 gap-4">
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">
                  Content Type
                </label>
                <select
                  v-model="formData.content_type"
                  class="w-full px-3 py-2 border border-gray-300 rounded-md"
                >
                  <option value="">
                    Select type
                  </option>
                  <option value="article">
                    Article
                  </option>
                  <option value="page">
                    Page
                  </option>
                </select>
              </div>

              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">
                  Quality Score (0-100)
                </label>
                <input
                  v-model.number="formData.quality_score"
                  type="number"
                  min="0"
                  max="100"
                  class="w-full px-3 py-2 border border-gray-300 rounded-md"
                >
              </div>
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">
                Topics (comma-separated)
              </label>
              <input
                v-model="topicsInput"
                type="text"
                placeholder="crime, news, local"
                class="w-full px-3 py-2 border border-gray-300 rounded-md"
              >
            </div>

            <div>
              <label class="flex items-center">
                <input
                  v-model="formData.is_crime_related"
                  type="checkbox"
                  class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                >
                <span class="ml-2 text-sm font-medium text-gray-700">
                  Is Crime Related
                </span>
              </label>
            </div>

            <div class="grid grid-cols-2 gap-4">
              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">
                  Published Date
                </label>
                <input
                  v-model="publishedDateInput"
                  type="datetime-local"
                  class="w-full px-3 py-2 border border-gray-300 rounded-md"
                >
              </div>

              <div>
                <label class="block text-sm font-medium text-gray-700 mb-1">
                  Crawled At
                </label>
                <input
                  v-model="crawledAtInput"
                  type="datetime-local"
                  class="w-full px-3 py-2 border border-gray-300 rounded-md"
                >
              </div>
            </div>

            <div>
              <label class="block text-sm font-medium text-gray-700 mb-1">
                Body / Raw Text
              </label>
              <textarea
                v-model="formData.body"
                rows="6"
                class="w-full px-3 py-2 border border-gray-300 rounded-md"
              />
            </div>
          </div>

          <div class="mt-6 flex justify-end gap-3">
            <button
              type="button"
              class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
              @click="$emit('close')"
            >
              Cancel
            </button>
            <button
              type="submit"
              :disabled="saving"
              class="px-4 py-2 bg-blue-600 text-white rounded-md text-sm font-medium hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <span
                v-if="saving"
                class="mr-2"
              >
                <svg
                  class="animate-spin h-4 w-4 inline"
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                >
                  <circle
                    class="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="4"
                  />
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  />
                </svg>
              </span>
              Save
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { indexManagerApi } from '../../api/client'
import type { Document } from '../../types/indexManager'
import type { ApiError } from '../../types/common'

interface Props {
  indexName: string
  document: Document
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved'): void
}>()

const saving = ref(false)
const error = ref<string | null>(null)

const formData = ref<Partial<Document>>({
  id: props.document.id,
  title: props.document.title || '',
  url: props.document.url || '',
  content_type: props.document.content_type || '',
  quality_score: props.document.quality_score,
  topics: props.document.topics || [],
  is_crime_related: props.document.is_crime_related || false,
  body: props.document.body || props.document.raw_text || '',
  published_date: props.document.published_date,
  crawled_at: props.document.crawled_at,
})

const topicsInput = computed({
  get: () => (formData.value.topics || []).join(', '),
  set: (value: string) => {
    formData.value.topics = value
      .split(',')
      .map((t) => t.trim())
      .filter((t) => t.length > 0)
  },
})

const publishedDateInput = computed({
  get: () => {
    if (!formData.value.published_date) return ''
    const date = new Date(formData.value.published_date)
    // Convert to local datetime-local format (YYYY-MM-DDTHH:mm)
    const year = date.getFullYear()
    const month = String(date.getMonth() + 1).padStart(2, '0')
    const day = String(date.getDate()).padStart(2, '0')
    const hours = String(date.getHours()).padStart(2, '0')
    const minutes = String(date.getMinutes()).padStart(2, '0')
    return `${year}-${month}-${day}T${hours}:${minutes}`
  },
  set: (value: string) => {
    if (!value) {
      formData.value.published_date = undefined
      return
    }
    const date = new Date(value)
    formData.value.published_date = date.toISOString()
  },
})

const crawledAtInput = computed({
  get: () => {
    if (!formData.value.crawled_at) return ''
    const date = new Date(formData.value.crawled_at)
    // Convert to local datetime-local format (YYYY-MM-DDTHH:mm)
    const year = date.getFullYear()
    const month = String(date.getMonth() + 1).padStart(2, '0')
    const day = String(date.getDate()).padStart(2, '0')
    const hours = String(date.getHours()).padStart(2, '0')
    const minutes = String(date.getMinutes()).padStart(2, '0')
    return `${year}-${month}-${day}T${hours}:${minutes}`
  },
  set: (value: string) => {
    if (!value) {
      formData.value.crawled_at = undefined
      return
    }
    const date = new Date(value)
    formData.value.crawled_at = date.toISOString()
  },
})

const handleSubmit = async (): Promise<void> => {
  saving.value = true
  error.value = null

  try {
    await indexManagerApi.documents.update(props.indexName, props.document.id, formData.value as Document)
    emit('saved')
    emit('close')
  } catch (err: unknown) {
    const axiosError = err as ApiError
    error.value = axiosError.response?.data?.error || 'Failed to update document'
  } finally {
    saving.value = false
  }
}

// Update form data when document prop changes
watch(() => props.document, (newDoc) => {
  formData.value = {
    id: newDoc.id,
    title: newDoc.title || '',
    url: newDoc.url || '',
    content_type: newDoc.content_type || '',
    quality_score: newDoc.quality_score,
    topics: newDoc.topics || [],
    is_crime_related: newDoc.is_crime_related || false,
    body: newDoc.body || newDoc.raw_text || '',
    published_date: newDoc.published_date,
    crawled_at: newDoc.crawled_at,
  }
}, { deep: true })
</script>
