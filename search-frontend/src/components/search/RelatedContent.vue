<template>
  <div
    v-if="hasContent"
    class="space-y-4 rounded-xl border border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] p-4"
    aria-label="Related content"
  >
    <div
      v-if="relatedTopics.length > 0"
      class="space-y-2"
    >
      <h3 class="text-xs font-semibold text-[var(--nc-text-muted)] uppercase tracking-wider">
        Related topics
      </h3>
      <div class="flex flex-wrap gap-2">
        <button
          v-for="bucket in relatedTopics"
          :key="bucket.key"
          type="button"
          class="inline-flex items-center rounded-full bg-[var(--nc-bg-muted)] px-3 py-1.5 text-sm text-[var(--nc-text)] hover:bg-[var(--nc-primary-muted)] hover:text-[var(--nc-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--nc-primary)] transition-colors duration-[var(--nc-duration)]"
          @click="onTopicClick(bucket.key)"
        >
          {{ formatTopicLabel(bucket.key) }}
          <span class="ml-1 text-[var(--nc-text-muted)]">({{ bucket.count }})</span>
        </button>
      </div>
    </div>

    <div
      v-if="firstSourceName"
      class="space-y-2"
    >
      <h3 class="text-xs font-semibold text-[var(--nc-text-muted)] uppercase tracking-wider">
        More from this source
      </h3>
      <button
        type="button"
        class="text-sm font-medium text-[var(--nc-primary)] hover:text-[var(--nc-primary-hover)] focus:outline-none focus:underline transition-colors duration-[var(--nc-duration)]"
        @click="onSourceClick(firstSourceName)"
      >
        More from {{ firstSourceName }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { FacetsFromApi, FacetBucketItem, SearchResult } from '@/types/search'

const props = defineProps<{
  facets: FacetsFromApi | null
  firstResult: SearchResult | null
}>()

const emit = defineEmits<{
  'topic-click': [topic: string]
  'source-click': [source: string]
}>()

const maxRelatedTopics = 10

const relatedTopics = computed((): FacetBucketItem[] => {
  const list = props.facets?.topics ?? []
  return list.slice(0, maxRelatedTopics)
})

const firstSourceName = computed((): string => {
  const r = props.firstResult
  if (!r) return ''
  return r.source_name ?? r.source ?? ''
})

const hasContent = computed((): boolean => {
  return relatedTopics.value.length > 0 || !!firstSourceName.value
})

function formatTopicLabel(key: string): string {
  return key.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}

function onTopicClick(topic: string): void {
  emit('topic-click', topic)
}

function onSourceClick(source: string): void {
  emit('source-click', source)
}
</script>
