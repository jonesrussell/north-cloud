<script setup lang="ts">
import { Search, X, Filter } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { useIndexes } from '../composables/useIndexes'
import type { IndexType, HealthStatus } from '@/types/indexManager'

const indexes = useIndexes()

const typeOptions: Array<{ value: IndexType | ''; label: string }> = [
  { value: '', label: 'All Types' },
  { value: 'raw_content', label: 'Raw Content' },
  { value: 'classified_content', label: 'Classified Content' },
  { value: 'article', label: 'Article' },
  { value: 'page', label: 'Page' },
]

const healthOptions: Array<{ value: HealthStatus | ''; label: string; color: string }> = [
  { value: '', label: 'All Health', color: 'bg-gray-400' },
  { value: 'green', label: 'Green', color: 'bg-emerald-500' },
  { value: 'yellow', label: 'Yellow', color: 'bg-amber-500' },
  { value: 'red', label: 'Red', color: 'bg-rose-500' },
]

function handleSearchInput(value: string) {
  indexes.setFilter('search', value || undefined)
}

function handleTypeChange(event: Event) {
  const target = event.target as HTMLSelectElement
  indexes.setFilter('type', (target.value as IndexType) || undefined)
}

function toggleHealthFilter(health: HealthStatus | '') {
  if (health === '') {
    indexes.setFilter('health', undefined)
  } else if (indexes.filters.value.health === health) {
    indexes.setFilter('health', undefined)
  } else {
    indexes.setFilter('health', health)
  }
}
</script>

<template>
  <div class="space-y-3">
    <!-- Search and Type Filter Row -->
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
      <!-- Search Input -->
      <div class="relative flex-1">
        <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          :model-value="indexes.filters.value.search || ''"
          placeholder="Search indexes by name..."
          class="pl-9"
          @update:model-value="handleSearchInput"
        />
      </div>

      <!-- Type Filter -->
      <div class="sm:w-48">
        <select
          :value="indexes.filters.value.type || ''"
          class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          @change="handleTypeChange"
        >
          <option
            v-for="option in typeOptions"
            :key="option.value"
            :value="option.value"
          >
            {{ option.label }}
          </option>
        </select>
      </div>

      <!-- Clear Filters Button -->
      <Button
        v-if="indexes.hasActiveFilters.value"
        variant="outline"
        size="sm"
        class="shrink-0"
        @click="indexes.clearFilters()"
      >
        <X class="mr-1 h-4 w-4" />
        Clear ({{ indexes.activeFilterCount.value }})
      </Button>
    </div>

    <!-- Health Filter Pills -->
    <div class="flex flex-wrap items-center gap-2">
      <div class="flex items-center gap-1.5 text-sm text-muted-foreground">
        <Filter class="h-4 w-4" />
        <span>Health:</span>
      </div>

      <button
        v-for="option in healthOptions"
        :key="option.value"
        :class="[
          'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium transition-colors',
          (option.value === '' && !indexes.filters.value.health) || indexes.filters.value.health === option.value
            ? 'bg-primary text-primary-foreground'
            : 'bg-muted text-muted-foreground hover:bg-muted/80',
        ]"
        @click="toggleHealthFilter(option.value)"
      >
        <span
          :class="['h-2 w-2 rounded-full', option.color]"
        />
        {{ option.label }}
      </button>
    </div>
  </div>
</template>
