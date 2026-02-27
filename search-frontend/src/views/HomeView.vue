<template>
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-12">
    <!-- Hero: Top Stories -->
    <section
      v-if="topStories.loading.value"
      aria-busy="true"
      aria-label="Loading top stories"
    >
      <h2 class="font-semibold text-2xl sm:text-3xl text-[var(--nc-text)] mb-4">
        Top Stories
      </h2>
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <div class="lg:col-span-2 rounded-xl bg-[var(--nc-bg-elevated)] border border-[var(--nc-border)] overflow-hidden animate-pulse">
          <div class="aspect-video bg-[var(--nc-bg-muted)]" />
        </div>
        <div class="space-y-4">
          <div
            v-for="i in heroSideCount"
            :key="i"
            class="rounded-xl bg-[var(--nc-bg-elevated)] border border-[var(--nc-border)] overflow-hidden animate-pulse"
          >
            <div class="aspect-video bg-[var(--nc-bg-muted)]" />
            <div class="p-3 space-y-2">
              <div class="h-4 bg-[var(--nc-bg-muted)] rounded w-3/4" />
              <div class="h-3 bg-[var(--nc-bg-muted)] rounded w-1/2" />
            </div>
          </div>
        </div>
      </div>
    </section>

    <section v-else-if="topStories.items.value.length >= heroMinItems">
      <h2 class="font-semibold text-2xl sm:text-3xl text-[var(--nc-text)] mb-4">
        Top Stories
      </h2>
      <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <!-- Hero card (2/3 width) -->
        <a
          :href="heroItem.url"
          target="_blank"
          rel="noopener noreferrer"
          class="lg:col-span-2 group relative rounded-xl overflow-hidden border border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] hover:border-[var(--nc-border-strong)] transition-colors"
          style="transition-duration: var(--nc-duration)"
          :aria-label="`Read article: ${heroItem.title}`"
        >
          <div class="relative aspect-video">
            <img
              v-if="heroItem.og_image"
              :src="heroItem.og_image"
              :alt="heroItem.title"
              loading="lazy"
              class="h-full w-full object-cover transition-transform group-hover:scale-105"
              style="transition-duration: var(--nc-duration-slow); transition-timing-function: var(--nc-ease-out)"
            >
            <div
              v-else
              class="h-full w-full bg-[var(--nc-bg-muted)]"
            />
            <!-- Gradient scrim -->
            <div class="absolute inset-0 bg-gradient-to-t from-black/80 via-black/30 to-transparent" />
            <!-- Overlay text -->
            <div class="absolute bottom-0 left-0 right-0 p-5 sm:p-6">
              <div class="mb-2 flex items-center gap-2 text-xs text-white/70">
                <span class="font-medium text-white/90">{{ heroItem.source }}</span>
                <span
                  v-if="heroTime"
                  aria-hidden="true"
                >&middot;</span>
                <time
                  v-if="heroTime"
                  :datetime="heroItem.published_at"
                >{{ heroTime }}</time>
              </div>
              <h3
                class="text-xl sm:text-2xl font-semibold text-white leading-snug line-clamp-3 group-hover:text-[var(--nc-primary-hover)] transition-colors"
                style="transition-duration: var(--nc-duration)"
              >
                {{ heroItem.title }}
              </h3>
            </div>
          </div>
        </a>

        <!-- Side cards (1/3 width) -->
        <div class="space-y-4">
          <NewsCard
            v-for="item in sideItems"
            :key="item.id"
            :item="item"
            :show-snippet="false"
          />
        </div>
      </div>
    </section>

    <!-- Fallback: regular grid if fewer than 2 items -->
    <ChannelSection
      v-else
      title="Top Stories"
      slug=""
      :items="topStories.items.value"
      :loading="topStories.loading.value"
      :error="topStories.error.value"
    />

    <!-- Trending strip -->
    <section>
      <h3 class="text-sm font-semibold text-[var(--nc-text-muted)] uppercase tracking-wider mb-3">
        Trending on North Cloud
      </h3>
      <div class="overflow-x-auto flex gap-2 pb-2">
        <router-link
          v-for="topic in trendingTopics"
          :key="topic.slug"
          :to="`/search?topics=${topic.slug}`"
          class="rounded-full px-3 py-1.5 text-sm bg-[var(--nc-bg-surface)] text-[var(--nc-text-secondary)] border border-[var(--nc-border)] hover:border-[var(--nc-primary)] hover:text-[var(--nc-primary)] transition-colors whitespace-nowrap"
          style="transition-duration: var(--nc-duration-fast)"
        >
          {{ topic.label }}
        </router-link>
      </div>
    </section>

    <!-- Channel sections -->
    <ChannelSection
      v-for="channel in channels"
      :key="channel.slug"
      :title="channel.title"
      :slug="channel.slug"
      :items="channel.feed.items.value"
      :loading="channel.feed.loading.value"
      :error="channel.feed.error.value"
      :channel-color="channel.color"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import ChannelSection from '@/components/news/ChannelSection.vue'
import NewsCard from '@/components/news/NewsCard.vue'
import { useFeed } from '@/composables/useFeed'
import { formatRelativeTime } from '@/utils/dateFormatter'

const heroMinItems = 2
const heroSideCount = 2

const topStories = useFeed()

const heroItem = computed(() => topStories.items.value[0])
const sideItems = computed(() => topStories.items.value.slice(1, 4))
const heroTime = computed(() => {
  if (!heroItem.value) return ''
  return formatRelativeTime(heroItem.value.published_at)
})

const trendingTopics = [
  { label: 'Crime', slug: 'crime' },
  { label: 'Mining', slug: 'mining' },
  { label: 'Entertainment', slug: 'entertainment' },
  { label: 'Local News', slug: 'local_news' },
  { label: 'Technology', slug: 'technology' },
  { label: 'Politics', slug: 'politics' },
  { label: 'Sports', slug: 'sports' },
]

const channels = [
  {
    title: 'Crime',
    slug: 'crime',
    color: 'bg-[var(--nc-error)]',
    feed: useFeed('crime'),
  },
  {
    title: 'Mining',
    slug: 'mining',
    color: 'bg-[var(--nc-accent)]',
    feed: useFeed('mining'),
  },
  {
    title: 'Entertainment',
    slug: 'entertainment',
    color: 'bg-[var(--nc-primary)]',
    feed: useFeed('entertainment'),
  },
]
</script>
