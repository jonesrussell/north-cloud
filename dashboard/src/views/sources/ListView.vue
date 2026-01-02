<template>
  <div>
    <PageHeader
      title="Sources"
      subtitle="Manage content sources for crawling"
    >
      <template #actions>
        <div class="flex gap-2">
          <button
            class="inline-flex items-center px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500"
            @click="openQuickCreate"
          >
            <PlusIcon class="h-5 w-5 mr-2" />
            Quick Add
          </button>
          <router-link
            to="/sources/new"
            class="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            Advanced Form
          </router-link>
        </div>
      </template>
    </PageHeader>

    <!-- Loading State -->
    <LoadingSpinner
      v-if="loading"
      size="lg"
      text="Loading sources..."
      :full-page="true"
    />

    <!-- Error State -->
    <ErrorAlert
      v-else-if="error"
      title="Error loading sources"
      :message="error"
      class="mb-6"
    />

    <!-- Empty State -->
    <div
      v-else-if="sources.length === 0"
      class="text-center py-12 bg-white rounded-lg border border-gray-200"
    >
      <DocumentTextIcon class="mx-auto h-12 w-12 text-gray-400" />
      <h3 class="mt-2 text-sm font-medium text-gray-900">
        No sources
      </h3>
      <p class="mt-1 text-sm text-gray-500">
        Get started by creating a new source.
      </p>
      <div class="mt-6 flex justify-center gap-3">
        <button
          class="inline-flex items-center px-4 py-2 border border-transparent shadow-sm text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700"
          @click="openQuickCreate"
        >
          <PlusIcon class="h-5 w-5 mr-2" />
          Quick Add
        </button>
        <router-link
          to="/sources/new"
          class="inline-flex items-center px-4 py-2 border border-gray-300 shadow-sm text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50"
        >
          Advanced Form
        </router-link>
      </div>
    </div>

    <!-- Sources List -->
    <div
      v-else
      class="bg-white shadow overflow-hidden sm:rounded-md"
    >
      <ul class="divide-y divide-gray-200">
        <li
          v-for="source in sources"
          :key="source.id"
          class="px-6 py-4 hover:bg-gray-50"
        >
          <div class="flex items-center justify-between">
            <div class="flex-1 min-w-0">
              <div class="flex items-center">
                <p class="text-sm font-medium text-gray-900 truncate">
                  {{ source.name }}
                </p>
                <StatusBadge
                  :status="source.enabled ? 'enabled' : 'disabled'"
                  class="ml-2"
                />
              </div>
              <div class="mt-1 flex items-center text-sm text-gray-500">
                <span class="truncate">{{ source.url }}</span>
              </div>
            </div>
            <div class="ml-4 flex-shrink-0 flex space-x-2">
              <router-link
                :to="`/sources/${source.id}/edit`"
                class="inline-flex items-center px-3 py-1.5 border border-gray-300 shadow-sm text-xs font-medium rounded text-gray-700 bg-white hover:bg-gray-50"
              >
                <PencilIcon class="h-4 w-4 mr-1" />
                Edit
              </router-link>
              <button
                class="inline-flex items-center px-3 py-1.5 border border-red-300 shadow-sm text-xs font-medium rounded text-red-700 bg-white hover:bg-red-50"
                @click="confirmDelete(source)"
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
    <ConfirmModal
      :show="!!sourceToDelete"
      title="Delete Source"
      :message="`Are you sure you want to delete '${sourceToDelete?.name}'? This action cannot be undone.`"
      type="danger"
      confirm-text="Delete"
      :loading="deleting"
      @confirm="handleDelete"
      @cancel="sourceToDelete = null"
    />

    <!-- Quick Create Modal -->
    <SourceQuickCreateModal
      ref="quickCreateModalRef"
      @created="onSourceCreated"
    />
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { PlusIcon, PencilIcon, TrashIcon, DocumentTextIcon } from '@heroicons/vue/24/outline'
import { sourcesApi } from '../../api/client'
import {
  PageHeader,
  LoadingSpinner,
  ErrorAlert,
  StatusBadge,
  ConfirmModal,
} from '../../components/common'
import SourceQuickCreateModal from '../../components/SourceQuickCreateModal.vue'

const sources = ref([])
const loading = ref(true)
const error = ref(null)
const sourceToDelete = ref(null)
const deleting = ref(false)
const quickCreateModalRef = ref(null)

const loadSources = async () => {
  loading.value = true
  error.value = null
  try {
    const response = await sourcesApi.list()
    sources.value = response.data?.sources || response.data || []
  } catch (err) {
    error.value = err.response?.data?.error || err.message || 'Failed to load sources'
    console.error('[ListView] Error loading sources:', err)
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
    deleting.value = true
    await sourcesApi.delete(sourceToDelete.value.id)
    await loadSources()
    sourceToDelete.value = null
  } catch (err) {
    error.value = err.response?.data?.error || err.message || 'Failed to delete source'
    console.error('[ListView] Error deleting source:', err)
    sourceToDelete.value = null
  } finally {
    deleting.value = false
  }
}

const openQuickCreate = () => {
  quickCreateModalRef.value?.open()
}

const onSourceCreated = () => {
  loadSources()
}

onMounted(() => {
  loadSources()
})
</script>
