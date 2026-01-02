<template>
  <div
    class="fixed inset-0 z-50 overflow-y-auto"
    @click.self="$emit('close')"
  >
    <div class="flex items-center justify-center min-h-screen px-4">
      <div
        class="fixed inset-0 bg-gray-500 bg-opacity-75 transition-opacity"
        @click="$emit('close')"
      />
      <div class="relative bg-white rounded-lg shadow-xl max-w-lg w-full p-6">
        <div class="flex justify-between items-center mb-4">
          <h2 class="text-xl font-semibold text-gray-900">
            Create Elasticsearch Index
          </h2>
          <button
            class="text-gray-400 hover:text-gray-500"
            @click="$emit('close')"
          >
            <span class="sr-only">Close</span>
            <XMarkIcon class="h-6 w-6" />
          </button>
        </div>

        <ErrorAlert
          v-if="error"
          :message="error"
          class="mb-4"
        />

        <div class="border-b border-gray-200 mb-4">
          <nav class="-mb-px flex gap-4">
            <button
              :class="[
                'py-2 px-1 border-b-2 font-medium text-sm',
                activeTab === 'single'
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              ]"
              @click="activeTab = 'single'"
            >
              Single Index
            </button>
            <button
              :class="[
                'py-2 px-1 border-b-2 font-medium text-sm',
                activeTab === 'source'
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              ]"
              @click="activeTab = 'source'"
            >
              For Source
            </button>
          </nav>
        </div>

        <form
          v-if="activeTab === 'single'"
          @submit.prevent="createSingleIndex"
        >
          <div class="mb-4">
            <label class="block text-sm font-medium text-gray-700 mb-1">
              Index Name *
            </label>
            <input
              v-model="singleForm.index_name"
              type="text"
              placeholder="e.g., sudbury_com_raw_content"
              required
              class="w-full px-3 py-2 border border-gray-300 rounded-md"
            >
            <p class="mt-1 text-xs text-gray-500">
              Full Elasticsearch index name
            </p>
          </div>

          <div class="mb-4">
            <label class="block text-sm font-medium text-gray-700 mb-1">
              Index Type *
            </label>
            <select
              v-model="singleForm.index_type"
              required
              class="w-full px-3 py-2 border border-gray-300 rounded-md"
            >
              <option value="">
                Select type...
              </option>
              <option
                v-for="option in INDEX_TYPE_OPTIONS"
                :key="option.value"
                :value="option.value"
              >
                {{ option.label }}
              </option>
            </select>
          </div>

          <div class="mb-4">
            <label class="block text-sm font-medium text-gray-700 mb-1">
              Source Name
            </label>
            <input
              v-model="singleForm.source_name"
              type="text"
              placeholder="e.g., sudbury_com"
              class="w-full px-3 py-2 border border-gray-300 rounded-md"
            >
            <p class="mt-1 text-xs text-gray-500">
              Optional: Associate with a source
            </p>
          </div>

          <div class="flex justify-end gap-3 mt-6">
            <button
              type="button"
              class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
              @click="$emit('close')"
            >
              Cancel
            </button>
            <button
              type="submit"
              class="px-4 py-2 bg-green-600 text-white rounded-md hover:bg-green-700 disabled:opacity-50"
              :disabled="creating"
            >
              {{ creating ? 'Creating...' : 'Create Index' }}
            </button>
          </div>
        </form>

        <form
          v-else
          @submit.prevent="createSourceIndexes"
        >
          <div class="mb-4">
            <label class="block text-sm font-medium text-gray-700 mb-1">
              Source Name *
            </label>
            <input
              v-model="sourceForm.source_name"
              type="text"
              placeholder="e.g., sudbury_com"
              required
              class="w-full px-3 py-2 border border-gray-300 rounded-md"
            >
            <p class="mt-1 text-xs text-gray-500">
              Will create indexes for this source
            </p>
          </div>

          <div class="mb-4">
            <label class="block text-sm font-medium text-gray-700 mb-2">
              Index Types
            </label>
            <div class="space-y-2">
              <label
                v-for="option in INDEX_TYPE_OPTIONS"
                :key="option.value"
                class="flex items-start"
              >
                <input
                  v-model="sourceForm.index_types"
                  type="checkbox"
                  :value="option.value"
                  class="mt-1 mr-2 rounded border-gray-300 text-blue-600"
                >
                <div>
                  <div class="text-sm font-medium text-gray-700">
                    {{ option.label }}
                  </div>
                  <div class="text-xs text-gray-500">
                    {{ option.description }}
                  </div>
                </div>
              </label>
            </div>
            <p class="mt-2 text-xs text-gray-500">
              Leave unchecked to create all default types (raw_content, classified_content)
            </p>
          </div>

          <div class="flex justify-end gap-3 mt-6">
            <button
              type="button"
              class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
              @click="$emit('close')"
            >
              Cancel
            </button>
            <button
              type="submit"
              class="px-4 py-2 bg-green-600 text-white rounded-md hover:bg-green-700 disabled:opacity-50"
              :disabled="creating"
            >
              {{ creating ? 'Creating...' : 'Create Indexes' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { XMarkIcon } from '@heroicons/vue/24/outline'
import { indexManagerApi } from '../../api/client'
import type { CreateIndexRequest, IndexType } from '../../types/indexManager'
import { INDEX_TYPE_OPTIONS } from '../../types/indexManager'
import type { ApiError } from '../../types/common'
import ErrorAlert from '../common/ErrorAlert.vue'

const emit = defineEmits(['close', 'created'])

const activeTab = ref<'single' | 'source'>('single')
const creating = ref(false)
const error = ref<string | null>(null)

const singleForm = ref<CreateIndexRequest>({
  index_name: '',
  index_type: '' as IndexType,
  source_name: '',
})

const sourceForm = ref<{
  source_name: string
  index_types: IndexType[]
}>({
  source_name: '',
  index_types: [],
})

const createSingleIndex = async (): Promise<void> => {
  creating.value = true
  error.value = null
  try {
    await indexManagerApi.indexes.create(singleForm.value)
    emit('created')
  } catch (err: unknown) {
    const axiosError = err as ApiError
    error.value = axiosError.response?.data?.error || 'Failed to create index'
  } finally {
    creating.value = false
  }
}

const createSourceIndexes = async (): Promise<void> => {
  creating.value = true
  error.value = null
  try {
    const payload = sourceForm.value.index_types.length > 0
      ? { index_types: sourceForm.value.index_types }
      : undefined

    await indexManagerApi.sources.createIndexes(
      sourceForm.value.source_name,
      payload
    )
    emit('created')
  } catch (err: unknown) {
    const axiosError = err as ApiError
    error.value = axiosError.response?.data?.error || 'Failed to create indexes'
  } finally {
    creating.value = false
  }
}
</script>
