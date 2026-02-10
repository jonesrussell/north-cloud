<script setup lang="ts">
import { Search, X } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import type { DiscoveredLinkFilters } from '@/features/intake/api/discoveredLinks'

defineProps<{
  filters: DiscoveredLinkFilters
  hasActiveFilters: boolean
  activeFilterCount: number
  sources?: Array<{ id: string; name: string }>
}>()

defineEmits<{
  (e: 'update:search', value: string): void
  (e: 'update:status', value: string): void
  (e: 'update:source_id', value: string): void
  (e: 'clear-filters'): void
}>()

const statusOptions = [
  { value: '', label: 'All' },
  { value: 'pending', label: 'Pending' },
  { value: 'processing', label: 'Processing' },
  { value: 'completed', label: 'Completed' },
  { value: 'failed', label: 'Failed' },
]
</script>

<template>
  <div class="space-y-3">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
      <div class="relative flex-1">
        <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          :model-value="filters.search || ''"
          placeholder="Search links by URL..."
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
