<template>
  <div
    class="text-center py-12 sm:py-16"
    role="status"
    aria-live="polite"
  >
    <div
      class="mx-auto w-14 h-14 rounded-2xl bg-[var(--nc-bg-muted)] flex items-center justify-center text-[var(--nc-text-muted)]"
      aria-hidden="true"
    >
      <svg
        class="w-7 h-7"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        stroke-width="1.5"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M9 13h6m-3-3v6m-9 1V7a2 2 0 012-2h6l2 2h6a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2z"
        />
      </svg>
    </div>
    <h2 class="mt-4 text-lg font-semibold text-[var(--nc-text)]">
      No results found
    </h2>
    <p class="mt-1 text-sm text-[var(--nc-text-secondary)]">
      {{ message }}
    </p>
    <p
      v-if="query"
      class="mt-1 text-sm text-[var(--nc-text-muted)]"
    >
      Query: <strong class="text-[var(--nc-text)]">{{ query }}</strong>
    </p>

    <div
      v-if="hasActiveFilters"
      class="mt-6"
    >
      <p class="text-sm text-[var(--nc-text-secondary)]">
        Active filters may be limiting results.
      </p>
      <button
        type="button"
        class="mt-3 inline-flex items-center rounded-lg border border-transparent bg-[var(--nc-accent)] px-4 py-2 text-sm font-medium text-white hover:bg-[var(--nc-accent-hover)] focus:outline-none focus:ring-2 focus:ring-[var(--nc-accent)] focus:ring-offset-2 transition-colors duration-[var(--nc-duration)]"
        @click="clearFilters"
      >
        Clear filters
      </button>
    </div>

    <div
      v-if="suggestedTopics.length > 0"
      class="mt-8"
    >
      <p class="text-sm font-medium text-[var(--nc-text)]">
        Try searching by topic:
      </p>
      <div class="mt-2 flex flex-wrap justify-center gap-2">
        <button
          v-for="topic in suggestedTopics"
          :key="topic.key"
          type="button"
          class="inline-flex items-center rounded-full bg-[var(--nc-bg-muted)] px-3 py-1.5 text-sm text-[var(--nc-text)] hover:bg-[var(--nc-border)] focus:outline-none focus:ring-2 focus:ring-[var(--nc-primary)] focus:ring-offset-2 transition-colors duration-[var(--nc-duration)]"
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
