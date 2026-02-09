<template>
  <div v-if="hasContent" class="space-y-4" aria-label="Related content">
    <!-- Related topics -->
    <div v-if="relatedTopics.length > 0" class="space-y-2">
      <h3 class="text-xs font-medium text-gray-500 uppercase tracking-wider">Related topics</h3>
      <div class="flex flex-wrap gap-2">
        <button
          v-for="bucket in relatedTopics"
          :key="bucket.key"
          type="button"
          class="inline-flex items-center rounded-full bg-gray-100 px-3 py-1.5 text-sm text-gray-700 hover:bg-gray-200 focus:outline-none focus:ring-2 focus:ring-blue-500"
          @click="onTopicClick(bucket.key)"
        >
          {{ formatTopicLabel(bucket.key) }}
          <span class="ml-1 text-gray-400">({{ bucket.count }})</span>
        </button>
      </div>
    </div>

    <!-- More from this source -->
    <div v-if="firstSourceName" class="space-y-2">
      <h3 class="text-xs font-medium text-gray-500 uppercase tracking-wider">More from this source</h3>
      <button
        type="button"
        class="text-sm text-blue-600 hover:text-blue-800 focus:outline-none focus:underline"
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
