<script setup lang="ts">
import { X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import type { ContentFilters } from '@/types/socialPublisher'

interface Props {
  filters: ContentFilters
  hasActiveFilters: boolean
  activeFilterCount: number
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:status', value: string | undefined): void
  (e: 'update:type', value: string | undefined): void
  (e: 'clear-filters'): void
}>()

const statusOptions = [
  { value: undefined as string | undefined, label: 'All Statuses' },
  { value: 'delivered', label: 'Delivered' },
  { value: 'failed', label: 'Failed' },
  { value: 'pending', label: 'Pending' },
] as const

const typeOptions = [
  { value: undefined as string | undefined, label: 'All Types' },
  { value: 'social_update', label: 'Social Update' },
  { value: 'blog_post', label: 'Blog Post' },
  { value: 'news_article', label: 'News Article' },
] as const
</script>

<template>
  <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
    <div class="flex flex-wrap items-center gap-2">
      <span class="text-sm font-medium text-muted-foreground">Status:</span>
      <button
        v-for="opt in statusOptions"
        :key="String(opt.value)"
        :class="[
          'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium transition-colors',
          (opt.value === undefined && !filters.status) || filters.status === opt.value
            ? 'bg-primary text-primary-foreground'
            : 'bg-muted text-muted-foreground hover:bg-muted/80',
        ]"
        @click="emit('update:status', opt.value)"
      >
        {{ opt.label }}
      </button>
    </div>

    <div class="flex flex-wrap items-center gap-2">
      <span class="text-sm font-medium text-muted-foreground">Type:</span>
      <button
        v-for="opt in typeOptions"
        :key="String(opt.value)"
        :class="[
          'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium transition-colors',
          (opt.value === undefined && !filters.type) || filters.type === opt.value
            ? 'bg-primary text-primary-foreground'
            : 'bg-muted text-muted-foreground hover:bg-muted/80',
        ]"
        @click="emit('update:type', opt.value)"
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
</template>
