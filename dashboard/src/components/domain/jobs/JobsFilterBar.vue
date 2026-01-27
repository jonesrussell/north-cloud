<script setup lang="ts">
import { computed } from 'vue'
import { Search, X, Filter } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { useJobsStore } from '@/stores/jobs'
import type { JobStatus } from '@/types/crawler'

interface Props {
  showSourceFilter?: boolean
  sources?: Array<{ id: string; name: string }>
}

withDefaults(defineProps<Props>(), {
  showSourceFilter: false,
  sources: () => [],
})

const jobsStore = useJobsStore()

const statusOptions: Array<{ value: JobStatus; label: string; color: string }> = [
  { value: 'running', label: 'Running', color: 'bg-blue-500' },
  { value: 'scheduled', label: 'Scheduled', color: 'bg-indigo-500' },
  { value: 'pending', label: 'Pending', color: 'bg-yellow-500' },
  { value: 'completed', label: 'Completed', color: 'bg-green-500' },
  { value: 'failed', label: 'Failed', color: 'bg-red-500' },
  { value: 'paused', label: 'Paused', color: 'bg-orange-500' },
  { value: 'cancelled', label: 'Cancelled', color: 'bg-gray-500' },
]

const activeFiltersCount = computed(() => {
  let count = 0
  if (jobsStore.filters.status) count++
  if (jobsStore.filters.source_id) count++
  if (jobsStore.filters.search) count++
  if (jobsStore.filters.schedule_enabled !== undefined) count++
  return count
})

const hasActiveFilters = computed(() => activeFiltersCount.value > 0)

function toggleStatusFilter(status: JobStatus) {
  if (jobsStore.filters.status === status) {
    jobsStore.setFilter('status', undefined)
  } else {
    jobsStore.setFilter('status', status)
  }
}

function handleSearchInput(value: string) {
  jobsStore.setFilter('search', value || undefined)
}

function handleSourceChange(event: Event) {
  const target = event.target as HTMLSelectElement
  jobsStore.setFilter('source_id', target.value || undefined)
}
</script>

<template>
  <div class="space-y-3">
    <!-- Search and Source Filter Row -->
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
      <!-- Search Input -->
      <div class="relative flex-1">
        <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          :model-value="jobsStore.filters.search || ''"
          placeholder="Search jobs by ID, source, or URL..."
          class="pl-9"
          @update:model-value="handleSearchInput"
        />
      </div>

      <!-- Source Filter -->
      <div
        v-if="showSourceFilter && sources.length > 0"
        class="sm:w-48"
      >
        <select
          :value="jobsStore.filters.source_id || ''"
          class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          @change="handleSourceChange"
        >
          <option value="">
            All Sources
          </option>
          <option
            v-for="source in sources"
            :key="source.id"
            :value="source.id"
          >
            {{ source.name }}
          </option>
        </select>
      </div>

      <!-- Clear Filters Button -->
      <Button
        v-if="hasActiveFilters"
        variant="outline"
        size="sm"
        class="shrink-0"
        @click="jobsStore.clearFilters()"
      >
        <X class="mr-1 h-4 w-4" />
        Clear ({{ activeFiltersCount }})
      </Button>
    </div>

    <!-- Status Filter Pills -->
    <div class="flex flex-wrap items-center gap-2">
      <div class="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Filter class="h-4 w-4" />
        <span>Status:</span>
      </div>

      <button
        v-for="option in statusOptions"
        :key="option.value"
        :class="[
          'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium transition-colors',
          jobsStore.filters.status === option.value
            ? 'bg-primary text-primary-foreground'
            : 'bg-muted text-muted-foreground hover:bg-muted/80',
        ]"
        @click="toggleStatusFilter(option.value)"
      >
        <span
          :class="['h-2 w-2 rounded-full', option.color]"
        />
        {{ option.label }}
        <Badge
          v-if="jobsStore.statusCounts[option.value] > 0"
          variant="secondary"
          class="ml-0.5 h-4 min-w-4 px-1 text-[10px]"
        >
          {{ jobsStore.statusCounts[option.value] }}
        </Badge>
      </button>
    </div>
  </div>
</template>
