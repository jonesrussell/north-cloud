<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import DataTable from '@/shared/components/DataTable.vue'
import type { Column } from '@/shared/components/DataTable.vue'
import StatusBadge from '@/shared/components/StatusBadge.vue'
import ErrorBanner from '@/shared/components/ErrorBanner.vue'
import ConfirmDialog from '@/shared/components/ConfirmDialog.vue'
import { useToast } from '@/shared/composables/useToast'
import { useSourceList, useDeleteSource, useToggleSource } from '../composables/useSourceApi'
import { getSourceStatus, formatDate, getErrorMessage } from '../utils'
import type { Source, SourceListParams } from '../types'

const router = useRouter()
const { success, error: showError } = useToast()

const listParams = ref<SourceListParams>({
  page: 1,
  limit: 20,
  sort_by: 'name',
  sort_order: 'asc',
})

const { data, isLoading, isError, error, refetch } = useSourceList(listParams)
const deleteMutation = useDeleteSource()
const toggleMutation = useToggleSource()

const rows = computed(() => (data.value?.data ?? []) as unknown as Record<string, unknown>[])
const total = computed(() => data.value?.total ?? 0)

const columns: Column[] = [
  { key: 'name', label: 'Name', sortable: true },
  { key: 'url', label: 'URL', sortable: true },
  { key: 'type', label: 'Type', sortable: false },
  { key: 'status', label: 'Status' },
  { key: 'updated_at', label: 'Last Updated', sortable: true },
  { key: 'actions', label: 'Actions' },
]

const deleteTarget = ref<Source | null>(null)
const showDeleteDialog = ref(false)

function handleSort(key: string) {
  if (listParams.value.sort_by === key) {
    listParams.value = {
      ...listParams.value,
      sort_order: listParams.value.sort_order === 'asc' ? 'desc' : 'asc',
    }
  } else {
    listParams.value = { ...listParams.value, sort_by: key, sort_order: 'asc' }
  }
}


function confirmDelete(row: Record<string, unknown>) {
  deleteTarget.value = row as unknown as Source
  showDeleteDialog.value = true
}

async function handleDelete() {
  if (!deleteTarget.value) return
  try {
    await deleteMutation.mutateAsync(deleteTarget.value.id)
    success(`Source "${deleteTarget.value.name}" deleted.`)
  } catch {
    showError('Failed to delete source.')
  } finally {
    showDeleteDialog.value = false
    deleteTarget.value = null
  }
}

async function handleToggle(row: Record<string, unknown>) {
  const source = row as unknown as Source
  const enabling = !source.enabled
  try {
    await toggleMutation.mutateAsync({ id: source.id, enabled: enabling })
    success(`Source "${source.name}" ${enabling ? 'enabled' : 'disabled'}.`)
  } catch {
    showError(`Failed to ${enabling ? 'enable' : 'disable'} source.`)
  }
}

const errorMessage = computed(() => getErrorMessage(error.value, 'Failed to load sources.'))
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-bold">Sources</h1>
      <button
        class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded hover:bg-blue-500"
        @click="router.push({ name: 'source-create' })"
      >
        Add Source
      </button>
    </div>

    <ErrorBanner v-if="isError" :message="errorMessage" @retry="refetch()" />

    <div class="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden">
      <DataTable
        :columns="columns"
        :rows="rows"
        :loading="isLoading"
        :total="total"
        :sort-key="listParams.sort_by"
        :sort-dir="listParams.sort_order"
        @sort="handleSort"
      >
        <template #name="{ row }">
          <router-link
            :to="{ name: 'source-detail', params: { id: row.id } }"
            class="text-blue-400 hover:text-blue-300 font-medium"
          >
            {{ row.name }}
          </router-link>
        </template>

        <template #url="{ row }">
          <span class="text-slate-400 text-xs truncate max-w-xs block">{{ row.url }}</span>
        </template>

        <template #type="{ row }">
          <span class="text-slate-400 text-xs capitalize">{{ row.type }}</span>
        </template>

        <template #status="{ row }">
          <StatusBadge :status="getSourceStatus(row as unknown as Source)" />
        </template>

        <template #updated_at="{ row }">
          <span class="text-slate-400 text-xs">{{ formatDate(row.updated_at as string) }}</span>
        </template>

        <template #actions="{ row }">
          <div class="flex gap-2">
            <button
              class="text-xs text-slate-400 hover:text-slate-200"
              @click="router.push({ name: 'source-edit', params: { id: row.id } })"
            >
              Edit
            </button>
            <button
              class="text-xs"
              :class="
                (row as Record<string, unknown>).enabled
                  ? 'text-amber-400 hover:text-amber-300'
                  : 'text-green-400 hover:text-green-300'
              "
              @click="handleToggle(row as Record<string, unknown>)"
            >
              {{ (row as Record<string, unknown>).enabled ? 'Disable' : 'Enable' }}
            </button>
            <button
              class="text-xs text-red-400 hover:text-red-300"
              @click="confirmDelete(row as Record<string, unknown>)"
            >
              Delete
            </button>
          </div>
        </template>
      </DataTable>
    </div>

    <ConfirmDialog
      :open="showDeleteDialog"
      title="Delete Source"
      :message="`Are you sure you want to delete '${deleteTarget?.name ?? ''}'? This action cannot be undone.`"
      confirm-label="Delete"
      :danger="true"
      @confirm="handleDelete"
      @cancel="showDeleteDialog = false"
    />
  </div>
</template>
