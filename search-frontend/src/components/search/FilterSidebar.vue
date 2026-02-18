<template>
  <aside
    class="space-y-6"
    aria-label="Refine search"
    role="search"
  >
    <div class="flex items-center justify-between">
      <h2 class="text-sm font-semibold text-gray-900">
        Refine results
      </h2>
      <button
        v-if="hasActiveFilters"
        type="button"
        class="text-sm text-blue-600 hover:text-blue-800 focus:outline-none focus:underline"
        @click="clearAll"
      >
        Clear all
      </button>
    </div>

    <!-- Topics -->
    <div
      v-if="topicBuckets.length > 0"
      class="space-y-2"
    >
      <h3 class="text-xs font-medium text-gray-500 uppercase tracking-wider">
        Topics
      </h3>
      <ul
        class="space-y-1"
        role="list"
      >
        <li
          v-for="bucket in topicBuckets"
          :key="bucket.key"
          class="flex items-center gap-2"
        >
          <input
            :id="`topic-${bucket.key}`"
            type="checkbox"
            :checked="isTopicSelected(bucket.key)"
            class="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
            :aria-label="`Filter by topic ${formatTopicLabel(bucket.key)}`"
            @change="toggleTopic(bucket.key)"
          >
          <label
            :for="`topic-${bucket.key}`"
            class="flex-1 cursor-pointer text-sm text-gray-700"
          >
            {{ formatTopicLabel(bucket.key) }}
            <span class="text-gray-400">({{ bucket.count }})</span>
          </label>
        </li>
      </ul>
    </div>

    <!-- Content type -->
    <div
      v-if="contentTypeBuckets.length > 0"
      class="space-y-2"
    >
      <h3 class="text-xs font-medium text-gray-500 uppercase tracking-wider">
        Content type
      </h3>
      <select
        v-model="localContentType"
        class="block w-full rounded-md border-gray-300 text-sm focus:border-blue-500 focus:ring-blue-500"
        aria-label="Filter by content type"
        @change="applyContentType"
      >
        <option value="">
          All
        </option>
        <option
          v-for="bucket in contentTypeBuckets"
          :key="bucket.key"
          :value="bucket.key"
        >
          {{ formatContentTypeLabel(bucket.key) }} ({{ bucket.count }})
        </option>
      </select>
    </div>

    <!-- Sources -->
    <div
      v-if="sourceBuckets.length > 0"
      class="space-y-2"
    >
      <h3 class="text-xs font-medium text-gray-500 uppercase tracking-wider">
        Sources
      </h3>
      <ul
        class="space-y-1 max-h-48 overflow-y-auto"
        role="list"
      >
        <li
          v-for="bucket in sourceBuckets"
          :key="bucket.key"
          class="flex items-center gap-2"
        >
          <input
            :id="`source-${bucket.key}`"
            type="checkbox"
            :checked="isSourceSelected(bucket.key)"
            class="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
            :aria-label="`Filter by source ${bucket.key}`"
            @change="toggleSource(bucket.key)"
          >
          <label
            :for="`source-${bucket.key}`"
            class="flex-1 cursor-pointer truncate text-sm text-gray-700"
            :title="bucket.key"
          >
            {{ bucket.key }}
            <span class="text-gray-400">({{ bucket.count }})</span>
          </label>
        </li>
      </ul>
    </div>

    <!-- Minimum quality -->
    <div class="space-y-2">
      <h3 class="text-xs font-medium text-gray-500 uppercase tracking-wider">
        Minimum quality
      </h3>
      <div class="flex items-center gap-2">
        <input
          v-model.number="localMinQuality"
          type="range"
          min="0"
          max="100"
          step="10"
          class="flex-1"
          aria-label="Minimum quality score"
          @change="applyMinQuality"
        >
        <span class="w-8 text-sm text-gray-600">{{ localMinQuality }}</span>
      </div>
    </div>

    <!-- Date range -->
    <div class="space-y-2">
      <h3 class="text-xs font-medium text-gray-500 uppercase tracking-wider">
        Date range
      </h3>
      <div class="grid grid-cols-1 gap-2">
        <label class="text-xs text-gray-500">From</label>
        <input
          v-model="localFromDate"
          type="date"
          class="rounded-md border-gray-300 text-sm focus:border-blue-500 focus:ring-blue-500"
          aria-label="From date"
          @change="applyDateRange"
        >
        <label class="text-xs text-gray-500 mt-1">To</label>
        <input
          v-model="localToDate"
          type="date"
          class="rounded-md border-gray-300 text-sm focus:border-blue-500 focus:ring-blue-500"
          aria-label="To date"
          @change="applyDateRange"
        >
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { SearchFilters, FacetsFromApi, FacetBucketItem } from '@/types/search'

const props = withDefaults(
  defineProps<{
    facets: FacetsFromApi | null
    filters: SearchFilters
  }>(),
  {}
)

const emit = defineEmits<{
  'update:filters': [filters: SearchFilters]
}>()

const topicBuckets = computed((): FacetBucketItem[] => props.facets?.topics ?? [])
const contentTypeBuckets = computed((): FacetBucketItem[] => props.facets?.content_types ?? [])
const sourceBuckets = computed((): FacetBucketItem[] => props.facets?.sources ?? [])

const localContentType = ref(props.filters.content_type ?? '')
const localMinQuality = ref(props.filters.min_quality_score ?? 0)
const localFromDate = ref(props.filters.from_date ?? '')
const localToDate = ref(props.filters.to_date ?? '')

watch(
  () => props.filters,
  (f) => {
    localContentType.value = f.content_type ?? ''
    localMinQuality.value = f.min_quality_score ?? 0
    localFromDate.value = f.from_date ?? ''
    localToDate.value = f.to_date ?? ''
  },
  { deep: true }
)

const hasActiveFilters = computed((): boolean => {
  const f = props.filters
  const hasTopics = f.topics && f.topics.length > 0
  const hasSources = f.source_names && f.source_names.length > 0
  const hasContentType = !!f.content_type
  const hasMinQuality = (f.min_quality_score ?? 0) > 0
  const hasFromDate = !!f.from_date
  const hasToDate = !!f.to_date
  return hasTopics || hasSources || hasContentType || hasMinQuality || hasFromDate || hasToDate
})

function isTopicSelected(key: string): boolean {
  return (props.filters.topics ?? []).includes(key)
}

function isSourceSelected(key: string): boolean {
  return (props.filters.source_names ?? []).includes(key)
}

function formatTopicLabel(key: string): string {
  return key.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}

function formatContentTypeLabel(key: string): string {
  return key.charAt(0).toUpperCase() + key.slice(1).toLowerCase()
}

function toggleTopic(key: string): void {
  const current = props.filters.topics ?? []
  const next = current.includes(key) ? current.filter((t) => t !== key) : [...current, key]
  emit('update:filters', { ...props.filters, topics: next })
}

function toggleSource(key: string): void {
  const current = props.filters.source_names ?? []
  const next = current.includes(key) ? current.filter((s) => s !== key) : [...current, key]
  emit('update:filters', { ...props.filters, source_names: next })
}

function applyContentType(): void {
  emit('update:filters', {
    ...props.filters,
    content_type: localContentType.value || null,
  })
}

function applyMinQuality(): void {
  emit('update:filters', {
    ...props.filters,
    min_quality_score: localMinQuality.value,
  })
}

function applyDateRange(): void {
  emit('update:filters', {
    ...props.filters,
    from_date: localFromDate.value || null,
    to_date: localToDate.value || null,
  })
}

function clearAll(): void {
  localContentType.value = ''
  localMinQuality.value = 0
  localFromDate.value = ''
  localToDate.value = ''
  emit('update:filters', {
    topics: [],
    content_type: null,
    min_quality_score: 0,
    from_date: null,
    to_date: null,
    source_names: [],
  })
}
</script>
