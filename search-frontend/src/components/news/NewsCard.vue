<template>
  <article
    class="group relative rounded-xl border border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] overflow-hidden transition-shadow hover:shadow-[var(--nc-shadow-lg)]"
    style="transition-duration: var(--nc-duration); transition-timing-function: var(--nc-ease-out)"
  >
    <a
      :href="item.url"
      target="_blank"
      rel="noopener noreferrer"
      class="block"
      :aria-label="`Read article: ${item.title}`"
    >
      <!-- 16:9 thumbnail -->
      <div class="relative aspect-video overflow-hidden">
        <img
          v-if="showImage"
          :src="item.og_image"
          :alt="item.title"
          loading="lazy"
          class="h-full w-full object-cover transition-transform group-hover:scale-105"
          style="transition-duration: var(--nc-duration-slow); transition-timing-function: var(--nc-ease-out)"
          @error="onImageError"
        >
        <div
          v-else
          class="h-full w-full"
          :class="channelColor"
          style="opacity: 0.15"
        />
      </div>

      <!-- Content -->
      <div class="p-4">
        <!-- Source + time -->
        <div class="mb-1.5 flex items-center gap-2 text-xs text-[var(--nc-text-muted)]">
          <span class="font-medium text-[var(--nc-text-secondary)]">{{ item.source }}</span>
          <span
            v-if="relativeTime"
            aria-hidden="true"
          >&middot;</span>
          <time
            v-if="relativeTime"
            :datetime="item.published_at"
          >{{ relativeTime }}</time>
        </div>

        <!-- Headline -->
        <h3
          class="font-semibold leading-snug text-[var(--nc-text)] line-clamp-2 transition-colors group-hover:text-[var(--nc-primary)]"
          style="transition-duration: var(--nc-duration)"
        >
          {{ item.title }}
        </h3>

        <!-- Snippet -->
        <p
          v-if="showSnippet && item.snippet"
          class="mt-1.5 text-sm leading-relaxed text-[var(--nc-text-muted)] line-clamp-1"
        >
          {{ item.snippet }}
        </p>
      </div>
    </a>
  </article>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { formatRelativeTime } from '@/utils/dateFormatter'
import type { FeedItem } from '@/types/search'

interface Props {
  item: FeedItem
  showSnippet?: boolean
  channelColor?: string
}

const props = withDefaults(defineProps<Props>(), {
  showSnippet: true,
  channelColor: 'bg-[var(--nc-primary)]',
})

const imageErrored = ref(false)

const showImage = computed((): boolean => {
  return Boolean(props.item.og_image) && !imageErrored.value
})

const relativeTime = computed((): string => {
  return formatRelativeTime(props.item.published_at)
})

function onImageError(): void {
  imageErrored.value = true
}
</script>
