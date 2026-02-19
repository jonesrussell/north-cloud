<template>
  <!-- eslint-disable vue/no-v-html -->
  <article
    class="result-card rounded-xl border border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] p-5 sm:p-6 transition-shadow duration-[var(--nc-duration)] hover:shadow-[var(--nc-shadow)]"
    :class="featured ? 'ring-2 ring-[var(--nc-primary)]/20 ring-offset-2 ring-offset-[var(--nc-bg)]' : ''"
    role="listitem"
  >
    <a
      :href="result.click_url || result.url"
      target="_blank"
      rel="noopener noreferrer"
      class="block group"
      :aria-label="`Open result: ${result.title}`"
    >
      <!-- Badges -->
      <div class="flex flex-wrap items-center gap-2 mb-2">
        <span
          v-if="contentTypeLabel"
          class="inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium bg-[var(--nc-bg-muted)] text-[var(--nc-text-secondary)]"
        >
          {{ contentTypeLabel }}
        </span>
        <span
          v-if="sourceBadge"
          class="inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium bg-[var(--nc-success-muted)] text-[var(--nc-success)] border border-[var(--nc-success)]/20"
        >
          {{ sourceBadge }}
        </span>
      </div>

      <!-- Title -->
      <h2
        class="font-semibold text-[var(--nc-primary)] group-hover:text-[var(--nc-primary-hover)] mb-1.5 transition-colors duration-[var(--nc-duration)]"
        :class="featured ? 'text-xl sm:text-2xl' : 'text-lg sm:text-xl'"
      >
        <span
          v-if="highlightedTitle"
          v-html="highlightedTitle"
        />
        <span v-else>{{ result.title }}</span>
      </h2>

      <!-- URL -->
      <p class="text-sm text-[var(--nc-success)] mb-2 truncate">
        {{ displayUrl }}
      </p>

      <!-- Snippet -->
      <p
        v-if="snippet"
        class="text-[var(--nc-text-secondary)] text-sm leading-relaxed mb-3 result-snippet"
        v-html="snippet"
      />
      <p
        v-else
        class="text-[var(--nc-text-secondary)] text-sm leading-relaxed mb-3"
      >
        {{ truncatedText }}
      </p>

      <!-- Meta -->
      <div class="flex flex-wrap items-center gap-x-4 gap-y-1.5 text-sm text-[var(--nc-text-muted)]">
        <span v-if="result.published_date">
          {{ formattedDate }}
        </span>
        <span
          v-if="result.quality_score !== undefined && result.quality_score !== null"
          class="inline-flex items-center px-2.5 py-0.5 rounded-md text-xs font-medium"
          :class="qualityBadgeClass"
          :title="`Quality score: ${result.quality_score} out of 100`"
        >
          Quality {{ result.quality_score }}
        </span>
        <div
          v-if="result.topics && result.topics.length"
          class="flex flex-wrap gap-1"
        >
          <span
            v-for="topic in result.topics"
            :key="topic"
            class="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium bg-[var(--nc-primary-muted)] text-[var(--nc-primary)]"
          >
            {{ topic }}
          </span>
        </div>
      </div>
    </a>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { formatDate } from '@/utils/dateFormatter'
import { parseHighlight, sanitizeHighlight } from '@/utils/highlightHelper'
import type { SearchResult } from '@/types/search'

interface Props {
  result: SearchResult
  featured?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  featured: false,
})

const contentTypeLabel = computed((): string => {
  const ct = props.result.content_type
  if (!ct) return ''
  return ct.charAt(0).toUpperCase() + ct.slice(1).toLowerCase()
})

const sourceBadge = computed((): string => {
  return props.result.source_name ?? props.result.source ?? ''
})

const highlightedTitle = computed((): string | null => {
  if (props.result.highlight && props.result.highlight.title && props.result.highlight.title.length > 0) {
    return sanitizeHighlight(props.result.highlight.title[0])
  }
  return null
})

const snippet = computed((): string | null => {
  if (props.result.highlight) {
    const bodyHighlight = parseHighlight(props.result.highlight, 'body', 200) || parseHighlight(props.result.highlight, 'raw_text', 200)
    return bodyHighlight ? sanitizeHighlight(bodyHighlight) : null
  }
  return null
})

const truncatedText = computed((): string => {
  const text = props.result.body || props.result.raw_text || ''
  return text.length > 200 ? text.substring(0, 200) + '...' : text
})

const displayUrl = computed((): string => {
  try {
    const url = new URL(props.result.url)
    return url.hostname + url.pathname
  } catch {
    return props.result.url
  }
})

const formattedDate = computed((): string => {
  return formatDate(props.result.published_date)
})

const qualityBadgeClass = computed((): string => {
  const score = props.result.quality_score || 0
  if (score >= 80) return 'bg-[var(--nc-success-muted)] text-[var(--nc-success)]'
  if (score >= 60) return 'bg-[var(--nc-warning-muted)] text-[var(--nc-warning)]'
  return 'bg-[var(--nc-bg-muted)] text-[var(--nc-text-muted)]'
})
</script>

<style scoped>
.result-snippet :deep(em) {
  background-color: var(--nc-highlight-bg);
  color: var(--nc-highlight-fg);
  font-style: normal;
  font-weight: 600;
  padding: 0 0.1em;
  border-radius: 2px;
}
</style>
