<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import DataTable from '@/shared/components/DataTable.vue'
import type { Column } from '@/shared/components/DataTable.vue'
import StatusBadge from '@/shared/components/StatusBadge.vue'
import ErrorBanner from '@/shared/components/ErrorBanner.vue'
import LoadingSkeleton from '@/shared/components/LoadingSkeleton.vue'
import StartCrawlDialog from '../components/StartCrawlDialog.vue'
import { useCrawlJobs } from '../composables/useCrawlApi'
import { formatDate, formatInterval, asJob } from '../utils'

const router = useRouter()
const showStartDialog = ref(false)

const { data, isLoading, isError, error, refetch } = useCrawlJobs()

const columns: Column[] = [
  { key: 'source_name', label: 'Source', sortable: true },
  { key: 'status', label: 'Status', sortable: true },
  { key: 'type', label: 'Type', sortable: false },
  { key: 'started_at', label: 'Started', sortable: true },
  { key: 'next_run_at', label: 'Next Run', sortable: true },
  { key: 'interval_minutes', label: 'Interval', sortable: false },
  { key: 'actions', label: 'Actions', sortable: false },
]

function navigateToDetail(row: Record<string, unknown>) {
  void router.push({ name: 'crawl-job-detail', params: { id: asJob(row).id } })
}
</script>

<template>
  <div>
    <div class="flex items-center justify-between mb-6">
      <h1 class="text-2xl font-bold text-slate-100">Crawl Jobs</h1>
      <button
        @click="showStartDialog = true"
        class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded hover:bg-blue-500"
      >
        Start Crawl
      </button>
    </div>

    <LoadingSkeleton v-if="isLoading" :lines="8" />

    <ErrorBanner
      v-else-if="isError"
      :message="error?.message ?? 'Failed to load crawl jobs'"
      @retry="refetch"
    />

    <DataTable
      v-else
      :columns="columns"
      :rows="(data?.jobs as unknown as Record<string, unknown>[]) ?? []"
      :total="data?.total"
      :loading="isLoading"
    >
      <template #source_name="{ row }">
        <button
          @click="navigateToDetail(row)"
          class="text-blue-400 hover:text-blue-300 hover:underline"
        >
          {{ asJob(row).source_name ?? asJob(row).source_id }}
        </button>
      </template>

      <template #status="{ row }">
        <StatusBadge :status="asJob(row).status" />
      </template>

      <template #started_at="{ row }">
        {{ formatDate(asJob(row).started_at) }}
      </template>

      <template #next_run_at="{ row }">
        {{ formatDate(asJob(row).next_run_at) }}
      </template>

      <template #interval_minutes="{ row }">
        {{ formatInterval(asJob(row).interval_minutes) }}
      </template>

      <template #actions="{ row }">
        <button
          @click="navigateToDetail(row)"
          class="text-sm text-slate-400 hover:text-slate-200"
        >
          View
        </button>
      </template>
    </DataTable>

    <StartCrawlDialog v-if="showStartDialog" @close="showStartDialog = false" />
  </div>
</template>
