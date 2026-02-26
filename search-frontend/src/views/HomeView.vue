<template>
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-12">
    <!-- Top Stories -->
    <ChannelSection
      title="Top Stories"
      slug=""
      :items="topStories.items.value"
      :loading="topStories.loading.value"
      :error="topStories.error.value"
    />

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
import ChannelSection from '@/components/news/ChannelSection.vue'
import { useFeed } from '@/composables/useFeed'

const topStories = useFeed()

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
