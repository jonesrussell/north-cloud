<template>
  <div>
    <PageHeader
      title="Sources"
      subtitle="Manage content sources for crawling"
    >
      <template #actions>
        <div class="flex gap-2">
          <button
            v-if="sources.length > 0"
            class="inline-flex items-center px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500"
            @click="exportSources"
          >
            <ArrowDownTrayIcon class="h-5 w-5 mr-2" />
            Export
          </button>
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
      <table class="min-w-full divide-y divide-gray-200">
        <thead class="bg-gray-50">
          <tr>
            <th class="px-6 py-3 text-left">
              <input
                type="checkbox"
                :checked="bulkOps.selectAll.value"
                :indeterminate="bulkOps.hasSelection.value && !bulkOps.selectAll.value"
                class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
                @change="bulkOps.toggleSelectAll(sources)"
              >
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Name
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              URL
            </th>
            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
              Status
            </th>
            <th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
              Actions
            </th>
          </tr>
        </thead>
        <tbody class="bg-white divide-y divide-gray-200">
          <tr
            v-for="source in sources"
            :key="source.id"
            :class="bulkOps.isSelected(source.id) ? 'bg-blue-50' : 'hover:bg-gray-50'"
          >
            <td class="px-6 py-4 whitespace-nowrap">
              <input
                type="checkbox"
                :checked="bulkOps.isSelected(source.id)"
                class="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
                @change="bulkOps.toggleItem(source.id)"
              >
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <div class="text-sm font-medium text-gray-900">
                {{ source.name }}
              </div>
            </td>
            <td class="px-6 py-4">
              <div class="text-sm text-gray-500 truncate max-w-md">
                {{ source.url }}
              </div>
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <StatusBadge :status="source.enabled ? 'enabled' : 'disabled'" />
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium space-x-2">
              <button
                class="inline-flex items-center px-2 py-1 border border-gray-300 shadow-sm text-xs font-medium rounded text-gray-700 bg-white hover:bg-gray-50"
                @click="cloneSource(source)"
                title="Clone source"
              >
                <DocumentDuplicateIcon class="h-4 w-4" />
              </button>
              <router-link
                :to="`/sources/${source.id}/edit`"
                class="inline-flex items-center px-2 py-1 border border-gray-300 shadow-sm text-xs font-medium rounded text-gray-700 bg-white hover:bg-gray-50"
                title="Edit source"
              >
                <PencilIcon class="h-4 w-4" />
              </router-link>
              <button
                class="inline-flex items-center px-2 py-1 border border-red-300 shadow-sm text-xs font-medium rounded text-red-700 bg-white hover:bg-red-50"
                @click="confirmDelete(source)"
                title="Delete source"
              >
                <TrashIcon class="h-4 w-4" />
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Bulk Actions Toolbar -->
    <BulkActionsToolbar
      :selected-count="bulkOps.selectedCount.value"
      :selected-ids="bulkOps.selectedIds.value"
      :available-actions="bulkActions"
      @cancel="bulkOps.clearSelection()"
    />

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
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import {
  PlusIcon,
  PencilIcon,
  TrashIcon,
  DocumentTextIcon,
  ArrowDownTrayIcon,
  DocumentDuplicateIcon,
  CheckIcon,
  XMarkIcon
} from '@heroicons/vue/24/outline'
import { PowerIcon } from '@heroicons/vue/24/solid'
import { sourcesApi } from '../../api/client'
import {
  PageHeader,
  LoadingSpinner,
  ErrorAlert,
  StatusBadge,
  ConfirmModal,
} from '../../components/common'
import BulkActionsToolbar from '../../components/common/BulkActionsToolbar.vue'
import SourceQuickCreateModal from '../../components/SourceQuickCreateModal.vue'
import { useBulkOperations } from '../../composables/useBulkOperations'

const router = useRouter()
const sources = ref([])
const loading = ref(true)
const error = ref(null)
const sourceToDelete = ref(null)
const deleting = ref(false)
const quickCreateModalRef = ref(null)

// Bulk operations
const bulkOps = useBulkOperations({
  onSuccess: (action, count) => {
    console.log(`[ListView] Bulk ${action} completed for ${count} items`)
  },
  onError: (action, err) => {
    error.value = `Bulk ${action} failed: ${err.message}`
    console.error(`[ListView] Bulk ${action} error:`, err)
  }
})

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

// Clone source
const cloneSource = async (source) => {
  try {
    // Create a copy with "(Copy)" appended to name
    const clonedSource = {
      ...source,
      id: undefined, // Remove ID so it creates new
      name: `${source.name} (Copy)`,
      created_at: undefined,
      updated_at: undefined,
    }

    await sourcesApi.create(clonedSource)
    await loadSources()
  } catch (err) {
    error.value = err.response?.data?.error || err.message || 'Failed to clone source'
    console.error('[ListView] Error cloning source:', err)
  }
}

// Export sources to JSON
const exportSources = () => {
  const dataStr = JSON.stringify({ sources: sources.value }, null, 2)
  const dataBlob = new Blob([dataStr], { type: 'application/json' })
  const url = URL.createObjectURL(dataBlob)
  const link = document.createElement('a')
  link.href = url
  link.download = `sources-export-${new Date().toISOString().split('T')[0]}.json`
  link.click()
  URL.revokeObjectURL(url)
}

// Bulk enable sources
const bulkEnable = async (ids) => {
  await bulkOps.performBulkAction('enable', async (selectedIds) => {
    // Update each source to enabled=true
    await Promise.all(
      selectedIds.map(id => {
        const source = sources.value.find(s => s.id === id)
        if (!source) return Promise.resolve()
        return sourcesApi.update(id, { ...source, enabled: true })
      })
    )
    await loadSources()
  })
}

// Bulk disable sources
const bulkDisable = async (ids) => {
  await bulkOps.performBulkAction('disable', async (selectedIds) => {
    // Update each source to enabled=false
    await Promise.all(
      selectedIds.map(id => {
        const source = sources.value.find(s => s.id === id)
        if (!source) return Promise.resolve()
        return sourcesApi.update(id, { ...source, enabled: false })
      })
    )
    await loadSources()
  })
}

// Bulk delete sources
const bulkDelete = async (ids) => {
  if (!confirm(`Are you sure you want to delete ${ids.length} source(s)? This action cannot be undone.`)) {
    return
  }

  await bulkOps.performBulkAction('delete', async (selectedIds) => {
    await Promise.all(selectedIds.map(id => sourcesApi.delete(id)))
    await loadSources()
  })
}

// Bulk export selected sources
const bulkExport = async (ids) => {
  const selectedSources = sources.value.filter(s => ids.includes(s.id))
  const dataStr = JSON.stringify({ sources: selectedSources }, null, 2)
  const dataBlob = new Blob([dataStr], { type: 'application/json' })
  const url = URL.createObjectURL(dataBlob)
  const link = document.createElement('a')
  link.href = url
  link.download = `sources-export-selected-${new Date().toISOString().split('T')[0]}.json`
  link.click()
  URL.revokeObjectURL(url)
}

// Define bulk actions
const bulkActions = computed(() => [
  {
    id: 'enable',
    label: 'Enable',
    variant: 'success',
    icon: CheckIcon,
    handler: bulkEnable
  },
  {
    id: 'disable',
    label: 'Disable',
    variant: 'default',
    icon: XMarkIcon,
    handler: bulkDisable
  },
  {
    id: 'export',
    label: 'Export Selected',
    variant: 'default',
    icon: ArrowDownTrayIcon,
    handler: bulkExport
  },
  {
    id: 'delete',
    label: 'Delete',
    variant: 'danger',
    icon: TrashIcon,
    handler: bulkDelete
  }
])

onMounted(() => {
  loadSources()
})
</script>
