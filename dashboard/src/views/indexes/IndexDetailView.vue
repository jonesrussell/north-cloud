<template>
  <div>
    <PageHeader
      :title="indexName"
      subtitle="View and manage documents in this index"
      back-link="/indexes"
      back-text="Back to Indexes"
    />

    <!-- Index Info -->
    <div
      v-if="indexInfo"
      class="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6"
    >
      <div class="bg-white shadow rounded-lg p-4">
        <div class="text-sm font-medium text-gray-500">
          Type
        </div>
        <div class="mt-2 text-lg font-semibold text-gray-900">
          {{ formatIndexType(indexInfo.type) }}
        </div>
      </div>
      <div class="bg-white shadow rounded-lg p-4">
        <div class="text-sm font-medium text-gray-500">
          Health
        </div>
        <div class="mt-2">
          <StatusBadge
            v-if="indexInfo.health"
            :status="indexInfo.health"
            :custom-label="indexInfo.health.toUpperCase()"
          />
        </div>
      </div>
      <div class="bg-white shadow rounded-lg p-4">
        <div class="text-sm font-medium text-gray-500">
          Total Documents
        </div>
        <div class="mt-2 text-lg font-semibold text-gray-900">
          {{ indexInfo.document_count?.toLocaleString() || '0' }}
        </div>
      </div>
      <div class="bg-white shadow rounded-lg p-4">
        <div class="text-sm font-medium text-gray-500">
          Source
        </div>
        <div class="mt-2 text-lg font-semibold text-gray-900">
          {{ indexInfo.source_name || '-' }}
        </div>
      </div>
    </div>

    <!-- Search and Filters -->
    <div class="bg-white shadow rounded-lg p-4 mb-4">
      <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">
            Search
          </label>
          <input
            v-model="searchQuery"
            type="text"
            placeholder="Search in title, URL, body..."
            class="w-full px-3 py-2 border border-gray-300 rounded-md"
            @input="debouncedSearch"
          >
        </div>
        <div>
          <label class="block text-sm font-medium text-gray-700 mb-1">
            Content Type
          </label>
          <select
            v-model="filters.content_type"
            class="w-full px-3 py-2 border border-gray-300 rounded-md"
            @change="loadDocuments"
          >
            <option value="">
              All Types
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
            Crime Related
          </label>
          <select
            v-model="crimeRelatedFilter"
            class="w-full px-3 py-2 border border-gray-300 rounded-md"
            @change="updateCrimeFilter"
          >
            <option value="">
              All
            </option>
            <option value="true">
              Yes
            </option>
            <option value="false">
              No
            </option>
          </select>
        </div>
      </div>
      <div class="flex gap-2">
        <button
          class="px-4 py-2 bg-gray-600 text-white rounded-md hover:bg-gray-700"
          @click="clearFilters"
        >
          Clear Filters
        </button>
        <button
          class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
          @click="loadDocuments"
        >
          Refresh
        </button>
      </div>
    </div>

    <!-- Bulk Actions Toolbar -->
    <BulkActionsToolbar
      v-if="selectedDocuments.length > 0"
      :selected-count="selectedDocuments.length"
      :selected-ids="selectedDocuments"
      :available-actions="bulkActions"
      @cancel="clearSelection"
    />

    <LoadingSpinner
      v-if="loading"
      text="Loading documents..."
    />

    <ErrorAlert
      v-else-if="error"
      :message="error"
      class="mb-4"
    />

    <!-- Documents Table -->
    <div
      v-else
      class="bg-white shadow rounded-lg overflow-hidden"
    >
      <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200">
          <thead class="bg-gray-50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                <input
                  type="checkbox"
                  :checked="allSelected"
                  :indeterminate="someSelected"
                  class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                  @change="toggleSelectAll"
                >
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                ID
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Title
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                URL
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Content Type
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Quality Score
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Published Date
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody class="bg-white divide-y divide-gray-200">
            <tr
              v-for="document in documents"
              :key="document.id"
              class="hover:bg-gray-50"
            >
              <td class="px-6 py-4 whitespace-nowrap">
                <input
                  type="checkbox"
                  :checked="isSelected(document.id)"
                  class="rounded border-gray-300 text-blue-600 focus:ring-blue-500"
                  @change="toggleSelection(document.id)"
                >
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm font-mono text-gray-500">
                {{ document.id.substring(0, 8) }}...
              </td>
              <td class="px-6 py-4 text-sm text-gray-900">
                {{ document.title || '-' }}
              </td>
              <td class="px-6 py-4 text-sm text-gray-600 max-w-xs truncate">
                <a
                  v-if="document.url"
                  :href="document.url"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="text-blue-600 hover:text-blue-900 hover:underline"
                  :title="document.url"
                >
                  {{ document.url }}
                </a>
                <span v-else>-</span>
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-600">
                {{ document.content_type || '-' }}
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-600">
                {{ document.quality_score ?? '-' }}
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-600">
                {{ formatDate(document.published_date) }}
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-sm font-medium">
                <button
                  class="text-blue-600 hover:text-blue-900 mr-4"
                  @click="editDocument(document)"
                >
                  Edit
                </button>
                <button
                  class="text-red-600 hover:text-red-900"
                  @click="confirmDeleteDocument(document)"
                >
                  Delete
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div
        v-if="!loading && documents.length === 0"
        class="text-center py-12 text-gray-500"
      >
        No documents found.
      </div>

      <!-- Pagination -->
      <div
        v-if="totalPages > 1"
        class="bg-white px-4 py-3 border-t border-gray-200 sm:px-6 flex items-center justify-between"
      >
        <div class="flex-1 flex justify-between sm:hidden">
          <button
            :disabled="currentPage === 1"
            class="relative inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
            @click="previousPage"
          >
            Previous
          </button>
          <button
            :disabled="currentPage >= totalPages"
            class="ml-3 relative inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
            @click="nextPage"
          >
            Next
          </button>
        </div>
        <div class="hidden sm:flex-1 sm:flex sm:items-center sm:justify-between">
          <div>
            <p class="text-sm text-gray-700">
              Showing
              <span class="font-medium">{{ ((currentPage - 1) * pageSize) + 1 }}</span>
              to
              <span class="font-medium">{{ Math.min(currentPage * pageSize, totalHits) }}</span>
              of
              <span class="font-medium">{{ totalHits }}</span>
              results
            </p>
          </div>
          <div class="flex items-center gap-2">
            <select
              v-model="pageSize"
              class="px-3 py-1 border border-gray-300 rounded-md text-sm"
              @change="onPageSizeChange"
            >
              <option :value="10">
                10 per page
              </option>
              <option :value="20">
                20 per page
              </option>
              <option :value="50">
                50 per page
              </option>
              <option :value="100">
                100 per page
              </option>
            </select>
            <nav
              class="relative z-0 inline-flex rounded-md shadow-sm -space-x-px"
              aria-label="Pagination"
            >
              <button
                :disabled="currentPage === 1"
                class="relative inline-flex items-center px-2 py-2 rounded-l-md border border-gray-300 bg-white text-sm font-medium text-gray-500 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                @click="goToPage(1)"
              >
                First
              </button>
              <button
                :disabled="currentPage === 1"
                class="relative inline-flex items-center px-2 py-2 border border-gray-300 bg-white text-sm font-medium text-gray-500 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                @click="previousPage"
              >
                Previous
              </button>
              <button
                :disabled="currentPage >= totalPages"
                class="relative inline-flex items-center px-2 py-2 border border-gray-300 bg-white text-sm font-medium text-gray-500 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                @click="nextPage"
              >
                Next
              </button>
              <button
                :disabled="currentPage >= totalPages"
                class="relative inline-flex items-center px-2 py-2 rounded-r-md border border-gray-300 bg-white text-sm font-medium text-gray-500 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                @click="goToPage(totalPages)"
              >
                Last
              </button>
            </nav>
          </div>
        </div>
      </div>
    </div>

    <!-- Edit Document Modal -->
    <EditEntryModal
      v-if="documentToEdit"
      :index-name="indexName"
      :document="documentToEdit"
      @close="documentToEdit = null"
      @saved="handleDocumentSaved"
    />

    <!-- Delete Confirmation Modal -->
    <ConfirmModal
      v-if="documentToDelete"
      :show="true"
      title="Delete Document"
      :message="`Are you sure you want to delete this document? This action cannot be undone.`"
      confirm-text="Delete"
      type="danger"
      :loading="deleting"
      @confirm="handleDeleteDocument"
      @cancel="documentToDelete = null"
    />

    <!-- Bulk Delete Confirmation Modal -->
    <ConfirmModal
      v-if="showBulkDeleteConfirm"
      :show="true"
      title="Delete Selected Documents"
      :message="`Are you sure you want to delete ${selectedDocuments.length} document(s)? This action cannot be undone.`"
      confirm-text="Delete"
      type="danger"
      :loading="bulkDeleting"
      @confirm="handleBulkDelete"
      @cancel="showBulkDeleteConfirm = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { TrashIcon } from '@heroicons/vue/24/outline'
import { indexManagerApi } from '../../api/client'
import type { Document, DocumentFilters, Index } from '../../types/indexManager'
import type { ApiError } from '../../types/common'
import type { BulkAction } from '../../components/common/BulkActionsToolbar.vue'
import PageHeader from '../../components/common/PageHeader.vue'
import LoadingSpinner from '../../components/common/LoadingSpinner.vue'
import ErrorAlert from '../../components/common/ErrorAlert.vue'
import StatusBadge from '../../components/common/StatusBadge.vue'
import ConfirmModal from '../../components/common/ConfirmModal.vue'
import BulkActionsToolbar from '../../components/common/BulkActionsToolbar.vue'
import EditEntryModal from '../../components/indexes/EditEntryModal.vue'

const route = useRoute()
const indexName = computed(() => route.params.index_name as string)

const documents = ref<Document[]>([])
const indexInfo = ref<Index | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

const searchQuery = ref('')
const filters = ref<DocumentFilters>({})
const crimeRelatedFilter = ref('')

const currentPage = ref(1)
const pageSize = ref(20)
const totalHits = ref(0)
const totalPages = ref(0)

const selectedDocuments = ref<string[]>([])
const documentToEdit = ref<Document | null>(null)
const documentToDelete = ref<Document | null>(null)
const deleting = ref(false)
const showBulkDeleteConfirm = ref(false)
const bulkDeleting = ref(false)

const allSelected = computed(() => {
  return documents.value.length > 0 && selectedDocuments.value.length === documents.value.length
})

const someSelected = computed(() => {
  return selectedDocuments.value.length > 0 && selectedDocuments.value.length < documents.value.length
})

const bulkActions = computed<BulkAction[]>(() => [
  {
    id: 'delete',
    label: 'Delete Selected',
    variant: 'danger',
    icon: TrashIcon,
    handler: () => {
      showBulkDeleteConfirm.value = true
      return Promise.resolve()
    },
  },
])

const loadIndexInfo = async (): Promise<void> => {
  try {
    const response = await indexManagerApi.indexes.get(indexName.value)
    indexInfo.value = response.data
  } catch (err: unknown) {
    const axiosError = err as ApiError
    console.error('Failed to load index info:', axiosError)
  }
}

const loadDocuments = async (): Promise<void> => {
  loading.value = true
  error.value = null
  try {
    const request = {
      query: searchQuery.value || undefined,
      filters: {
        ...filters.value,
        is_crime_related: crimeRelatedFilter.value ? crimeRelatedFilter.value === 'true' : undefined,
      },
      pagination: {
        page: currentPage.value,
        size: pageSize.value,
      },
      sort: {
        field: 'relevance',
        order: 'desc',
      },
    }

    const response = await indexManagerApi.documents.query(indexName.value, request)
    documents.value = response.data.documents || []
    totalHits.value = response.data.total_hits || 0
    totalPages.value = response.data.total_pages || 0

    // Clear selection when documents change
    selectedDocuments.value = []
  } catch (err: unknown) {
    const axiosError = err as ApiError
    error.value = axiosError.response?.data?.error || 'Failed to load documents'
  } finally {
    loading.value = false
  }
}

const updateCrimeFilter = (): void => {
  filters.value.is_crime_related = crimeRelatedFilter.value ? crimeRelatedFilter.value === 'true' : undefined
  loadDocuments()
}

const clearFilters = (): void => {
  searchQuery.value = ''
  filters.value = {}
  crimeRelatedFilter.value = ''
  currentPage.value = 1
  loadDocuments()
}

let searchTimeout: number
const debouncedSearch = (): void => {
  clearTimeout(searchTimeout)
  searchTimeout = window.setTimeout(() => {
    currentPage.value = 1
    loadDocuments()
  }, 500)
}

const previousPage = (): void => {
  if (currentPage.value > 1) {
    currentPage.value--
    loadDocuments()
  }
}

const nextPage = (): void => {
  if (currentPage.value < totalPages.value) {
    currentPage.value++
    loadDocuments()
  }
}

const goToPage = (page: number): void => {
  currentPage.value = page
  loadDocuments()
}

const onPageSizeChange = (): void => {
  currentPage.value = 1
  loadDocuments()
}

const isSelected = (documentId: string): boolean => {
  return selectedDocuments.value.includes(documentId)
}

const toggleSelection = (documentId: string): void => {
  const index = selectedDocuments.value.indexOf(documentId)
  if (index > -1) {
    selectedDocuments.value.splice(index, 1)
  } else {
    selectedDocuments.value.push(documentId)
  }
}

const toggleSelectAll = (): void => {
  if (allSelected.value) {
    selectedDocuments.value = []
  } else {
    selectedDocuments.value = documents.value.map((doc) => doc.id)
  }
}

const clearSelection = (): void => {
  selectedDocuments.value = []
}

const editDocument = (document: Document): void => {
  documentToEdit.value = document
}

const handleDocumentSaved = async (): Promise<void> => {
  documentToEdit.value = null
  await loadDocuments()
}

const confirmDeleteDocument = (document: Document): void => {
  documentToDelete.value = document
}

const handleDeleteDocument = async (): Promise<void> => {
  if (!documentToDelete.value) return

  deleting.value = true
  try {
    await indexManagerApi.documents.delete(indexName.value, documentToDelete.value.id)
    await loadDocuments()
    documentToDelete.value = null
  } catch (err: unknown) {
    const axiosError = err as ApiError
    error.value = axiosError.response?.data?.error || 'Failed to delete document'
    documentToDelete.value = null
  } finally {
    deleting.value = false
  }
}

const handleBulkDelete = async (): Promise<void> => {
  if (selectedDocuments.value.length === 0) return

  bulkDeleting.value = true
  try {
    await indexManagerApi.documents.bulkDelete(indexName.value, selectedDocuments.value)
    showBulkDeleteConfirm.value = false
    await loadDocuments()
    selectedDocuments.value = []
  } catch (err: unknown) {
    const axiosError = err as ApiError
    error.value = axiosError.response?.data?.error || 'Failed to delete documents'
    showBulkDeleteConfirm.value = false
  } finally {
    bulkDeleting.value = false
  }
}

const formatIndexType = (type: string): string => {
  return type.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase())
}

const formatDate = (dateString: string | undefined): string => {
  if (!dateString) return 'N/A'
  try {
    const date = new Date(dateString)
    return date.toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  } catch {
    return 'Invalid date'
  }
}

onMounted(() => {
  loadIndexInfo()
  loadDocuments()
})
</script>
