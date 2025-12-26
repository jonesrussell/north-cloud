<template>
  <div class="bg-white p-6 rounded-lg shadow-sm hover:shadow-md transition-shadow">
    <a
      :href="result.url"
      target="_blank"
      rel="noopener noreferrer"
      class="block group"
    >
      <!-- Title -->
      <h3 class="text-xl font-semibold text-blue-600 group-hover:text-blue-800 mb-2">
        <span v-if="highlightedTitle" v-html="highlightedTitle"></span>
        <span v-else>{{ result.title }}</span>
      </h3>

      <!-- URL -->
      <div class="text-sm text-green-700 mb-2">
        {{ displayUrl }}
      </div>

      <!-- Snippet -->
      <p v-if="snippet" class="text-gray-700 mb-3" v-html="snippet"></p>
      <p v-else class="text-gray-700 mb-3">{{ truncatedText }}</p>

      <!-- Metadata -->
      <div class="flex items-center space-x-4 text-sm text-gray-500">
        <span v-if="result.published_date">
          {{ formattedDate }}
        </span>
        <span v-if="result.quality_score" class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium" :class="qualityBadgeClass">
          Quality: {{ result.quality_score }}
        </span>
        <div v-if="result.topics && result.topics.length" class="flex flex-wrap gap-1">
          <span
            v-for="topic in result.topics"
            :key="topic"
            class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800"
          >
            {{ topic }}
          </span>
        </div>
      </div>
    </a>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { formatDate } from '@/utils/dateFormatter'
import { parseHighlight, sanitizeHighlight } from '@/utils/highlightHelper'

const props = defineProps({
  result: {
    type: Object,
    required: true,
  },
})

const highlightedTitle = computed(() => {
  if (props.result.highlight && props.result.highlight.title) {
    return sanitizeHighlight(props.result.highlight.title[0])
  }
  return null
})

const snippet = computed(() => {
  if (props.result.highlight) {
    const bodyHighlight = parseHighlight(props.result.highlight, 'body', 200) || parseHighlight(props.result.highlight, 'raw_text', 200)
    return bodyHighlight ? sanitizeHighlight(bodyHighlight) : null
  }
  return null
})

const truncatedText = computed(() => {
  const text = props.result.body || props.result.raw_text || ''
  return text.length > 200 ? text.substring(0, 200) + '...' : text
})

const displayUrl = computed(() => {
  try {
    const url = new URL(props.result.url)
    return url.hostname + url.pathname
  } catch {
    return props.result.url
  }
})

const formattedDate = computed(() => {
  return formatDate(props.result.published_date)
})

const qualityBadgeClass = computed(() => {
  const score = props.result.quality_score || 0
  if (score >= 80) return 'bg-green-100 text-green-800'
  if (score >= 60) return 'bg-yellow-100 text-yellow-800'
  return 'bg-gray-100 text-gray-800'
})
</script>

<style scoped>
/* Highlight styling for matched terms */
:deep(em) {
  background-color: #fef08a;
  font-style: normal;
  font-weight: 600;
}
</style>
