<script setup lang="ts">
import { Search, X } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import type { FrontierFilters } from '@/features/intake/api/frontier'

defineProps<{
  filters: FrontierFilters
  hasActiveFilters: boolean
  activeFilterCount: number
  sources?: Array<{ id: string; name: string }>
}>()

defineEmits<{
  (e: 'update:search', value: string): void
  (e: 'update:status', value: string): void
  (e: 'update:source_id', value: string): void
  (e: 'update:host', value: string): void
  (e: 'update:origin', value: string): void
  (e: 'clear-filters'): void
}>()

const statusOptions = [
  { value: '', label: 'All Statuses' },
  { value: 'pending', label: 'Pending' },
  { value: 'fetching', label: 'Fetching' },
  { value: 'fetched', label: 'Fetched' },
  { value: 'failed', label: 'Failed' },
  { value: 'dead', label: 'Dead' },
]

const originOptions = [
  { value: '', label: 'All Origins' },
  { value: 'feed', label: 'Feed' },
  { value: 'sitemap', label: 'Sitemap' },
  { value: 'spider', label: 'Spider' },
  { value: 'manual', label: 'Manual' },
]
</script>

<template>
  <div class="space-y-3">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:flex-wrap">
      <div class="relative flex-1 min-w-48">
        <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          :model-value="filters.search || ''"
          placeholder="Search by URL..."
          class="pl-9"
          @update:model-value="$emit('update:search', $event)"
        />
      </div>

      <div class="sm:w-36">
        <select
          :value="filters.status || ''"
          class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
          @change="$emit('update:status', ($event.target as HTMLSelectElement).value || '')"
        >
          <option
            v-for="opt in statusOptions"
            :key="opt.value"
            :value="opt.value"
          >
            {{ opt.label }}
          </option>
        </select>
      </div>

      <div class="sm:w-36">
        <select
          :value="filters.origin || ''"
          class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
          @change="$emit('update:origin', ($event.target as HTMLSelectElement).value || '')"
        >
          <option
            v-for="opt in originOptions"
            :key="opt.value"
            :value="opt.value"
          >
            {{ opt.label }}
          </option>
        </select>
      </div>

      <div
        v-if="sources && sources.length > 0"
        class="sm:w-48"
      >
        <select
          :value="filters.source_id || ''"
          class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
          @change="$emit('update:source_id', ($event.target as HTMLSelectElement).value || '')"
        >
          <option value="">
            All Sources
          </option>
          <option
            v-for="src in sources"
            :key="src.id"
            :value="src.id"
          >
            {{ src.name }}
          </option>
        </select>
      </div>

      <div class="min-w-36">
        <Input
          :model-value="filters.host || ''"
          placeholder="Filter by host..."
          @update:model-value="$emit('update:host', $event)"
        />
      </div>

      <Button
        v-if="hasActiveFilters"
        variant="outline"
        size="sm"
        class="shrink-0"
        @click="$emit('clear-filters')"
      >
        <X class="mr-1 h-4 w-4" />
        Clear ({{ activeFilterCount }})
      </Button>
    </div>
  </div>
</template>
