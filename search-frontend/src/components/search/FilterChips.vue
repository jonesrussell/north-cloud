<template>
  <div class="flex flex-wrap items-center gap-2">
    <template
      v-for="chip in activeChips"
      :key="chip.id"
    >
      <span
        class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-3 py-1 text-sm text-gray-700"
      >
        <span class="font-medium">{{ chip.label }}:</span>
        <span>{{ chip.value }}</span>
        <button
          type="button"
          class="rounded-full p-0.5 hover:bg-gray-200 focus:outline-none focus:ring-2 focus:ring-blue-500"
          :aria-label="`Remove filter ${chip.label} ${chip.value}`"
          @click="chip.remove()"
        >
          <svg
            class="h-3.5 w-3.5 text-gray-500"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
      </span>
    </template>
    <button
      type="button"
      class="inline-flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500"
      aria-label="Open filters"
      aria-expanded="false"
      :aria-controls="drawerId"
      @click="$emit('open-drawer')"
    >
      <svg
        class="h-4 w-4 text-gray-500"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z"
        />
      </svg>
      Filters
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { SearchFilters } from '@/types/search'

const props = defineProps<{
  filters: SearchFilters
  drawerId?: string
}>()

const emit = defineEmits<{
  'open-drawer': []
  'remove-topic': [topic: string]
  'remove-source': [source: string]
  'clear-content-type': []
  'clear-min-quality': []
  'clear-dates': []
}>()

interface Chip {
  id: string
  label: string
  value: string
  remove: () => void
}

const activeChips = computed((): Chip[] => {
  const chips: Chip[] = []
  const f = props.filters

  ;(f.topics ?? []).forEach((t) => {
    chips.push({
      id: `topic-${t}`,
      label: 'Topic',
      value: t.replace(/_/g, ' '),
      remove: () => emit('remove-topic', t),
    })
  })
  ;(f.source_names ?? []).forEach((s) => {
    chips.push({
      id: `source-${s}`,
      label: 'Source',
      value: s,
      remove: () => emit('remove-source', s),
    })
  })
  if (f.content_type) {
    chips.push({
      id: 'content-type',
      label: 'Type',
      value: f.content_type,
      remove: () => emit('clear-content-type'),
    })
  }
  if ((f.min_quality_score ?? 0) > 0) {
    chips.push({
      id: 'min-quality',
      label: 'Min quality',
      value: String(f.min_quality_score),
      remove: () => emit('clear-min-quality'),
    })
  }
  if (f.from_date || f.to_date) {
    chips.push({
      id: 'dates',
      label: 'Date',
      value: [f.from_date, f.to_date].filter(Boolean).join(' – '),
      remove: () => emit('clear-dates'),
    })
  }
  return chips
})
</script>
