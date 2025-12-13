<template>
  <div>
    <div class="sm:flex sm:items-center sm:justify-between mb-6">
      <div>
        <h2 class="text-2xl font-bold text-gray-900">Sources</h2>
        <p class="mt-1 text-sm text-gray-600">Manage content sources for crawling</p>
      </div>
      <div class="mt-4 sm:mt-0">
        <router-link
          to="/sources/new"
          class="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
        >
          <PlusIcon class="h-5 w-5 mr-2" />
          Add Source
        </router-link>
      </div>
    </div>

    <div v-if="loading" class="text-center py-12">
      <div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
      <p class="mt-2 text-sm text-gray-600">Loading sources...</p>
    </div>

    <div v-else-if="error" class="rounded-md bg-red-50 p-4 mb-4">
      <div class="flex">
        <ExclamationCircleIcon class="h-5 w-5 text-red-400" />
        <div class="ml-3">
          <h3 class="text-sm font-medium text-red-800">Error loading sources</h3>
          <div class="mt-2 text-sm text-red-700">{{ error }}</div>
        </div>
      </div>
    </div>

    <div v-else-if="sources.length === 0" class="text-center py-12 bg-white rounded-lg border border-gray-200">
      <DocumentTextIcon class="mx-auto h-12 w-12 text-gray-400" />
      <h3 class="mt-2 text-sm font-medium text-gray-900">No sources</h3>
      <p class="mt-1 text-sm text-gray-500">Get started by creating a new source.</p>
      <div class="mt-6">
        <router-link
          to="/sources/new"
          class="inline-flex items-center px-4 py-2 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700"
        >
          <PlusIcon class="h-5 w-5 mr-2" />
          Add Source
        </router-link>
      </div>
    </div>

    <div v-else class="bg-white shadow overflow-hidden sm:rounded-md">
      <ul class="divide-y divide-gray-200">
        <li v-for="source in sources" :key="source.id" class="px-6 py-4 hover:bg-gray-50">
          <div class="flex items-center justify-between">
            <div class="flex-1 min-w-0">
              <div class="flex items-center">
                <p class="text-sm font-medium text-gray-900 truncate">{{ source.name }}</p>
                <span
                  v-if="source.enabled"
                  class="ml-2 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 text-green-800"
                >
                  Enabled
                </span>
                <span
                  v-else
                  class="ml-2 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800"
                >
                  Disabled
                </span>
              </div>
              <div class="mt-1 flex items-center text-sm text-gray-500">
                <span>{{ source.url }}</span>
                <span v-if="source.city_name" class="ml-2 text-xs text-gray-400">
                  • City: {{ source.city_name }}
                </span>
              </div>
              <div class="mt-1 text-xs text-gray-500">
                Index: {{ source.article_index }}
              </div>
            </div>
            <div class="ml-4 flex-shrink-0 flex space-x-2">
              <router-link
                :to="`/sources/${source.id}/edit`"
                class="inline-flex items-center px-3 py-1.5 border border-gray-300 shadow-sm text-xs font-medium rounded text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
              >
                <PencilIcon class="h-4 w-4 mr-1" />
                Edit
              </router-link>
              <button
                @click="confirmDelete(source)"
                class="inline-flex items-center px-3 py-1.5 border border-red-300 shadow-sm text-xs font-medium rounded text-red-700 bg-white hover:bg-red-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500"
              >
                <TrashIcon class="h-4 w-4 mr-1" />
                Delete
              </button>
            </div>
          </div>
        </li>
      </ul>
    </div>

    <!-- Delete Confirmation Modal -->
    <div
      v-if="sourceToDelete"
      class="fixed z-10 inset-0 overflow-y-auto"
      @click.self="sourceToDelete = null"
    >
      <div class="flex items-center justify-center min-h-screen pt-4 px-4 pb-20 text-center sm:block sm:p-0">
        <div class="fixed inset-0 bg-gray-500 bg-opacity-75 transition-opacity"></div>
        <div class="inline-block align-bottom bg-white rounded-lg text-left overflow-hidden shadow-xl transform transition-all sm:my-8 sm:align-middle sm:max-w-lg sm:w-full">
          <div class="bg-white px-4 pt-5 pb-4 sm:p-6 sm:pb-4">
            <div class="sm:flex sm:items-start">
              <div class="mx-auto flex-shrink-0 flex items-center justify-center h-12 w-12 rounded-full bg-red-100 sm:mx-0 sm:h-10 sm:w-10">
                <ExclamationTriangleIcon class="h-6 w-6 text-red-600" />
              </div>
              <div class="mt-3 text-center sm:mt-0 sm:ml-4 sm:text-left">
                <h3 class="text-lg leading-6 font-medium text-gray-900">Delete source</h3>
                <div class="mt-2">
                  <p class="text-sm text-gray-500">
                    Are you sure you want to delete "{{ sourceToDelete.name }}"? This action cannot be undone.
                  </p>
                </div>
              </div>
            </div>
          </div>
          <div class="bg-gray-50 px-4 py-3 sm:px-6 sm:flex sm:flex-row-reverse">
            <button
              @click="handleDelete"
              class="w-full inline-flex justify-center rounded-md border border-transparent shadow-sm px-4 py-2 bg-red-600 text-base font-medium text-white hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500 sm:ml-3 sm:w-auto sm:text-sm"
            >
              Delete
            </button>
            <button
              @click="sourceToDelete = null"
              class="mt-3 w-full inline-flex justify-center rounded-md border border-gray-300 shadow-sm px-4 py-2 bg-white text-base font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 sm:mt-0 sm:ml-3 sm:w-auto sm:text-sm"
            >
              Cancel
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { sourcesApi } from '../api/client'
import {
  PlusIcon,
  PencilIcon,
  TrashIcon,
  ExclamationCircleIcon,
  ExclamationTriangleIcon,
  DocumentTextIcon,
} from '@heroicons/vue/24/outline'

const sources = ref([])
const loading = ref(true)
const error = ref(null)
const sourceToDelete = ref(null)

const loadSources = async () => {
  loading.value = true
  error.value = null
  try {
    sources.value = await sourcesApi.list()
  } catch (err) {
    error.value = err.response?.data?.error || err.message || 'Failed to load sources'
  } finally {
    loading.value = false
  }
}

const confirmDelete = (source) => {
  sourceToDelete.value = source
}

const handleDelete = async () => {
  if (!sourceToDelete.value) return
  
  try {
    await sourcesApi.delete(sourceToDelete.value.id)
    await loadSources()
    sourceToDelete.value = null
  } catch (err) {
    error.value = err.response?.data?.error || err.message || 'Failed to delete source'
    sourceToDelete.value = null
  }
}

onMounted(() => {
  loadSources()
})
</script>

