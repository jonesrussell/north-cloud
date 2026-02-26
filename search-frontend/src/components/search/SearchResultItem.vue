<template>
  <!-- eslint-disable vue/no-v-html -->
  <article
    class="result-card rounded-xl border border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] p-5 transition-colors hover:border-[var(--nc-border-strong)]"
    style="transition-duration: var(--nc-duration); transition-timing-function: var(--nc-ease-out)"
    role="listitem"
  >
    <a
      :href="result.click_url || result.url"
      target="_blank"
      rel="noopener noreferrer"
      class="block group"
      :aria-label="`Open result: ${result.title}`"
    >
      <div class="flex gap-4">
        <!-- Content -->
        <div class="flex-1 min-w-0">
          <!-- Source + time -->
          <div class="flex items-center gap-2 mb-2 text-sm text-[var(--nc-text-muted)]">
            <span class="font-medium text-[var(--nc-text-secondary)]">
              {{ sourceBadge }}
            </span>
            <span
              v-if="formattedDate"
              aria-hidden="true"
            >&middot;</span>
            <span v-if="formattedDate">{{ formattedDate }}</span>
          </div>

          <!-- Title -->
          <h2
            class="text-lg font-semibold text-[var(--nc-text)] group-hover:text-[var(--nc-primary)] mb-1.5 transition-colors duration-[var(--nc-duration)]"
          >
            <span
              v-if="highlightedTitle"
              v-html="highlightedTitle"
            />
            <span v-else>{{ result.title }}</span>
          </h2>

          <!-- Snippet -->
          <p
            v-if="snippet"
            class="text-sm text-[var(--nc-text-secondary)] leading-relaxed line-clamp-2 mb-3 result-snippet"
            v-html="snippet"
          />
          <p
            v-else
            class="text-sm text-[var(--nc-text-secondary)] leading-relaxed line-clamp-2 mb-3"
          >
            {{ truncatedText }}
          </p>

          <!-- Meta: topics + quality -->
          <div class="flex flex-wrap items-center gap-x-3 gap-y-1.5 text-sm">
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
            <span
              v-if="result.quality_score !== undefined && result.quality_score !== null"
              class="inline-flex items-center px-2.5 py-0.5 rounded-md text-xs font-medium"
              :class="qualityBadgeClass"
              :title="`Quality score: ${result.quality_score} out of 100`"
            >
              Quality {{ result.quality_score }}
            </span>
          </div>
        </div>

        <!-- Thumbnail -->
        <img
          v-if="result.og_image && !imageErrored"
          :src="result.og_image"
          :alt="result.title"
          class="hidden sm:block flex-shrink-0 w-[120px] h-[90px] rounded-lg object-cover bg-[var(--nc-bg-muted)]"
          loading="lazy"
          @error="imageErrored = true"
        >
      </div>
    </a>
  </article>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { formatDate } from '@/utils/dateFormatter'
import { parseHighlight, sanitizeHighlight } from '@/utils/highlightHelper'
import type { SearchResult } from '@/types/search'

interface Props {
  result: SearchResult
}

const props = defineProps<Props>()

const imageErrored = ref(false)

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
