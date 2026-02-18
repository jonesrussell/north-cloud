<template>
  <div
    class="text-center py-12"
    role="status"
    aria-live="polite"
  >
    <svg
      class="mx-auto h-12 w-12 text-gray-400"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        vector-effect="non-scaling-stroke"
        stroke-linecap="round"
        stroke-linejoin="round"
        stroke-width="2"
        d="M9 13h6m-3-3v6m-9 1V7a2 2 0 012-2h6l2 2h6a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2z"
      />
    </svg>
    <h3 class="mt-2 text-sm font-medium text-gray-900">
      No results found
    </h3>
    <p class="mt-1 text-sm text-gray-500">
      {{ message }}
    </p>
    <p
      v-if="query"
      class="mt-1 text-sm text-gray-600"
    >
      Query: <strong>{{ query }}</strong>
    </p>

    <!-- Active filters summary + Clear -->
    <div
      v-if="hasActiveFilters"
      class="mt-6"
    >
      <p class="text-sm text-gray-600">
        Active filters may be limiting results.
      </p>
      <button
        type="button"
        class="mt-3 inline-flex items-center rounded-md border border-transparent bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
        @click="clearFilters"
      >
        Clear filters
      </button>
    </div>

    <!-- Suggested topics -->
    <div
      v-if="suggestedTopics.length > 0"
      class="mt-8"
    >
      <p class="text-sm font-medium text-gray-700">
        Try searching by topic:
      </p>
      <div class="mt-2 flex flex-wrap justify-center gap-2">
        <button
          v-for="topic in suggestedTopics"
          :key="topic.key"
          type="button"
          class="inline-flex items-center rounded-full bg-gray-100 px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-200 focus:outline-none focus:ring-2 focus:ring-blue-500"
          @click="onSuggestedTopicClick(topic.key)"
        >
          {{ formatTopicLabel(topic.key) }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { SearchFilters, FacetsFromApi, FacetBucketItem } from '@/types/search'

const props = withDefaults(
  defineProps<{
    query?: string
    message?: string
    filters?: SearchFilters
    facets?: FacetsFromApi | null
    clearFilters?: () => void
  }>(),
  {
    query: '',
    message: 'Try different keywords or adjust your search filters.',
    filters: () => ({}),
    facets: () => null,
    clearFilters: () => {},
  }
)

const emit = defineEmits<{
  'suggested-topic': [topic: string]
}>()

const hasActiveFilters = computed((): boolean => {
  const f = props.filters ?? {}
  return (
    (f.topics?.length ?? 0) > 0 ||
    (f.source_names?.length ?? 0) > 0 ||
    !!f.content_type ||
    (f.min_quality_score ?? 0) > 0 ||
    !!f.from_date ||
    !!f.to_date
  )
})

const suggestedTopics = computed((): FacetBucketItem[] => {
  const list = props.facets?.topics ?? []
  return list.slice(0, 8)
})

function formatTopicLabel(key: string): string {
  return key.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}

function onSuggestedTopicClick(topic: string): void {
  emit('suggested-topic', topic)
}
</script>
