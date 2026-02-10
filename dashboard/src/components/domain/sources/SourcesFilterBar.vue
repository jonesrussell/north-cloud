<script setup lang="ts">
import { Search, X } from 'lucide-vue-next'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import type { SourceFilters } from '@/features/scheduling/api/sources'

interface Props {
  filters: SourceFilters
  hasActiveFilters: boolean
  activeFilterCount: number
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:search', value: string): void
  (e: 'update:enabled', value: boolean | undefined): void
  (e: 'clear-filters'): void
}>()

const enabledOptions = [
  { value: undefined as boolean | undefined, label: 'All' },
  { value: true, label: 'Active' },
  { value: false, label: 'Inactive' },
] as const
</script>

<template>
  <div class="space-y-3">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
      <div class="relative flex-1">
        <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          :model-value="filters.search || ''"
          placeholder="Search sources by name or URL..."
          class="pl-9"
          @update:model-value="emit('update:search', $event)"
        />
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <button
          v-for="opt in enabledOptions"
          :key="String(opt.value)"
          :class="[
            'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium transition-colors',
            (opt.value === undefined && filters.enabled === undefined) || filters.enabled === opt.value
              ? 'bg-primary text-primary-foreground'
              : 'bg-muted text-muted-foreground hover:bg-muted/80',
          ]"
          @click="emit('update:enabled', opt.value)"
        >
          {{ opt.label }}
        </button>
      </div>

      <Button
        v-if="hasActiveFilters"
        variant="outline"
        size="sm"
        class="shrink-0"
        @click="emit('clear-filters')"
      >
        <X class="mr-1 h-4 w-4" />
        Clear ({{ activeFilterCount }})
      </Button>
    </div>
  </div>
</template>
