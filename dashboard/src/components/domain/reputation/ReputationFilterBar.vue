<script setup lang="ts">
import { Search, X } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import type { ReputationFilters } from '@/features/scheduling/api/reputation'

defineProps<{
  filters: ReputationFilters
  hasActiveFilters: boolean
  activeFilterCount: number
  categories?: string[]
}>()

defineEmits<{
  (e: 'update:search', value: string): void
  (e: 'update:category', value: string): void
  (e: 'clear-filters'): void
}>()
</script>

<template>
  <div class="space-y-3">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
      <div class="relative flex-1">
        <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          :model-value="filters.search || ''"
          placeholder="Search by source name..."
          class="pl-9"
          @update:model-value="$emit('update:search', $event)"
        />
      </div>

      <div
        v-if="categories && categories.length > 0"
        class="sm:w-48"
      >
        <select
          :value="filters.category || ''"
          class="flex h-10 w-full rounded-md border border-input bg-background px-3 py-2 text-sm"
          @change="$emit('update:category', ($event.target as HTMLSelectElement).value || '')"
        >
          <option value="">
            All Categories
          </option>
          <option
            v-for="cat in categories"
            :key="cat"
            :value="cat"
          >
            {{ cat }}
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
