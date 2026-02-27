<template>
  <section>
    <!-- Header: title + "See more" link -->
    <div class="flex items-baseline justify-between mb-4">
      <h2 class="font-semibold text-2xl sm:text-3xl text-[var(--nc-text)]">
        <span
          class="inline-block w-2 h-5 rounded-sm mr-2 align-middle"
          :class="channelColor"
        />
        {{ title }}
      </h2>
      <RouterLink
        :to="seeMoreLink"
        class="text-sm font-medium text-[var(--nc-primary)] hover:text-[var(--nc-primary-hover)] transition-colors duration-[var(--nc-duration-fast)] whitespace-nowrap"
      >
        See more &rarr;
      </RouterLink>
    </div>

    <!-- Loading state: skeleton cards -->
    <div
      v-if="loading"
      class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5"
      aria-busy="true"
      aria-label="Loading articles"
    >
      <div
        v-for="i in SKELETON_COUNT"
        :key="i"
        class="rounded-xl border border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] overflow-hidden animate-pulse"
      >
        <div class="aspect-video bg-[var(--nc-bg-muted)]" />
        <div class="p-4 space-y-2">
          <div class="h-4 bg-[var(--nc-bg-muted)] rounded w-3/4" />
          <div class="h-4 bg-[var(--nc-bg-muted)] rounded w-full" />
          <div class="h-3 bg-[var(--nc-bg-muted)] rounded w-1/2" />
        </div>
      </div>
    </div>

    <!-- Error state -->
    <div
      v-else-if="error"
      class="text-center py-10 text-[var(--nc-text-muted)]"
    >
      <p>Failed to load articles. Please try again later.</p>
    </div>

    <!-- Empty state -->
    <div
      v-else-if="items.length === 0"
      class="text-center py-10 text-[var(--nc-text-muted)]"
    >
      <p>No articles available yet.</p>
    </div>

    <!-- Content state: grid of NewsCards -->
    <div
      v-else
      class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5"
    >
      <NewsCard
        v-for="item in items"
        :key="item.id"
        :item="item"
        :channel-color="channelColor"
      />
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import NewsCard from './NewsCard.vue'
import type { FeedItem } from '@/types/search'

const SKELETON_COUNT = 6

interface Props {
  title: string
  slug: string
  items: FeedItem[]
  loading?: boolean
  error?: boolean
  channelColor?: string
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
  error: false,
  channelColor: 'bg-[var(--nc-primary)]',
})

const seeMoreLink = computed((): string => {
  if (!props.slug) return '/search'
  return `/search?topics=${encodeURIComponent(props.slug)}`
})
</script>
