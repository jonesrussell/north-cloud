<template>
  <div>
    <PageHeader
      title="Elasticsearch Indexes"
      subtitle="Manage Elasticsearch indexes for content storage and retrieval"
    />

    <div class="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
      <div
        v-if="stats"
        class="bg-white shadow rounded-lg p-4"
      >
        <div class="text-sm font-medium text-gray-500">
          Total Indexes
        </div>
        <div class="mt-2 text-2xl font-semibold text-gray-900">
          {{ stats.total_indexes }}
        </div>
      </div>

      <div
        v-if="stats"
        class="bg-white shadow rounded-lg p-4"
      >
        <div class="text-sm font-medium text-gray-500">
          Total Documents
        </div>
        <div class="mt-2 text-2xl font-semibold text-gray-900">
          {{ stats.total_documents.toLocaleString() }}
        </div>
      </div>

      <div
        v-if="stats"
        class="bg-white shadow rounded-lg p-4"
      >
        <div class="text-sm font-medium text-gray-500">
          Cluster Health
        </div>
        <div class="mt-2">
          <StatusBadge
            :status="stats.cluster_health"
            :custom-label="stats.cluster_health.toUpperCase()"
          />
        </div>
      </div>

      <div class="bg-white shadow rounded-lg p-4 flex items-center justify-center">
        <button
          class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
          @click="openCreateModal"
        >
          + Create Index
        </button>
      </div>
    </div>

    <div class="bg-white shadow rounded-lg p-4 mb-4">
      <div class="flex gap-4 items-end">
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">
            Filter by Type
          </label>
          <select
            v-model="filterType"
            class="px-3 py-2 border border-gray-300 rounded-md"
            @change="loadIndexes"
          >
            <option value="">
              All Types
            </option>
            <option value="raw_content">
              Raw Content
            </option>
            <option value="classified_content">
              Classified Content
            </option>
            <option value="article">
              Article (Deprecated)
            </option>
            <option value="page">
              Page (Deprecated)
            </option>
          </select>
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">
            Filter by Source
          </label>
          <input
            v-model="filterSource"
            type="text"
            placeholder="e.g., sudbury_com"
            class="px-3 py-2 border border-gray-300 rounded-md"
            @input="debouncedLoadIndexes"
          >
        </div>
        <button
          class="px-4 py-2 bg-gray-600 text-white rounded-md hover:bg-gray-700"
          @click="loadIndexes"
        >
          Refresh
        </button>
      </div>
    </div>

    <LoadingSpinner
      v-if="loading"
      text="Loading indexes..."
    />

    <ErrorAlert
      v-else-if="error"
      :message="error"
      class="mb-4"
    />

    <div
      v-else
      class="bg-white shadow rounded-lg overflow-hidden"
    >
      <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200">
          <thead class="bg-gray-50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Index Name
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Type
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Source
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Health
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Documents
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Size
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody class="bg-white divide-y divide-gray-200">
            <tr
              v-for="index in indexes"
              :key="index.name"
              class="hover:bg-gray-50"
            >
              <td class="px-6 py-4">
                <code class="text-sm font-medium text-gray-900">
                  {{ index.name }}
                </code>
              </td>
              <td class="px-6 py-4 whitespace-nowrap">
                <span class="text-sm text-gray-600">
                  {{ formatIndexType(index.type) }}
                </span>
              </td>
              <td class="px-6 py-4 whitespace-nowrap">
                <span class="text-sm text-gray-600">
                  {{ index.source_name || '-' }}
                </span>
              </td>
              <td class="px-6 py-4 whitespace-nowrap">
                <StatusBadge
                  v-if="index.health"
                  :status="index.health"
                  :custom-label="index.health.toUpperCase()"
                />
                <span
                  v-else
                  class="text-sm text-gray-400"
                >-</span>
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-600">
                {{ index.document_count?.toLocaleString() || '0' }}
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-600">
                {{ index.size || '-' }}
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm font-medium">
                <button
                  class="text-blue-600 hover:text-blue-900 mr-4"
                  @click="viewIndexHealth(index)"
                >
                  Health
                </button>
                <button
                  class="text-red-600 hover:text-red-900"
                  @click="confirmDeleteIndex(index)"
                >
                  Delete
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div
        v-if="!loading && indexes.length === 0"
        class="text-center py-12 text-gray-500"
      >
        No indexes found. Click "Create Index" to get started.
      </div>
    </div>

    <CreateIndexModal
      v-if="showCreateModal"
      @close="closeCreateModal"
      @created="handleIndexCreated"
    />

    <IndexHealthModal
      v-if="selectedIndex"
      :index="selectedIndex"
      @close="selectedIndex = null"
    />

    <ConfirmModal
      v-if="indexToDelete"
      :show="true"
      title="Delete Index"
      :message="`Are you sure you want to delete the index '${indexToDelete.name}'? This action cannot be undone and will permanently delete all documents in this index.`"
      confirm-text="Delete"
      :loading="deleting"
      @confirm="handleDeleteIndex"
      @cancel="indexToDelete = null"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { indexManagerApi } from '../../api/client'
import type { Index, IndexStats } from '../../types/indexManager'
import PageHeader from '../../components/common/PageHeader.vue'
import LoadingSpinner from '../../components/common/LoadingSpinner.vue'
import ErrorAlert from '../../components/common/ErrorAlert.vue'
import StatusBadge from '../../components/common/StatusBadge.vue'
import ConfirmModal from '../../components/common/ConfirmModal.vue'
import CreateIndexModal from '../../components/indexes/CreateIndexModal.vue'
import IndexHealthModal from '../../components/indexes/IndexHealthModal.vue'

const indexes = ref<Index[]>([])
const stats = ref<IndexStats | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

const filterType = ref('')
const filterSource = ref('')

const showCreateModal = ref(false)
const selectedIndex = ref<Index | null>(null)
const indexToDelete = ref<Index | null>(null)
const deleting = ref(false)

const loadIndexes = async (): Promise<void> => {
  loading.value = true
  error.value = null
  try {
    const params: { type?: string; source?: string } = {}
    if (filterType.value) params.type = filterType.value
    if (filterSource.value) params.source = filterSource.value

    const response = await indexManagerApi.indexes.list(params)
    indexes.value = response.data.indices || []
  } catch (err: any) {
    error.value = err.response?.data?.error || 'Failed to load indexes'
  } finally {
    loading.value = false
  }
}

const loadStats = async (): Promise<void> => {
  try {
    const response = await indexManagerApi.stats.get()
    stats.value = response.data
  } catch (err: any) {
    console.error('Failed to load stats:', err)
  }
}

const openCreateModal = (): void => {
  showCreateModal.value = true
}

const closeCreateModal = (): void => {
  showCreateModal.value = false
}

const handleIndexCreated = async (): Promise<void> => {
  closeCreateModal()
  await Promise.all([loadIndexes(), loadStats()])
}

const viewIndexHealth = (index: Index): void => {
  selectedIndex.value = index
}

const confirmDeleteIndex = (index: Index): void => {
  indexToDelete.value = index
}

const handleDeleteIndex = async (): Promise<void> => {
  if (!indexToDelete.value) return

  deleting.value = true
  try {
    await indexManagerApi.indexes.delete(indexToDelete.value.name)
    await Promise.all([loadIndexes(), loadStats()])
    indexToDelete.value = null
  } catch (err: any) {
    error.value = err.response?.data?.error || 'Failed to delete index'
    indexToDelete.value = null
  } finally {
    deleting.value = false
  }
}

const formatIndexType = (type: string): string => {
  return type.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase())
}

let filterTimeout: number
const debouncedLoadIndexes = (): void => {
  clearTimeout(filterTimeout)
  filterTimeout = window.setTimeout(() => loadIndexes(), 500)
}

onMounted(() => {
  Promise.all([loadIndexes(), loadStats()])
})
</script>
