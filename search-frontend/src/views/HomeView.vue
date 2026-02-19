<template>
  <div class="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8">
    <div class="pt-12 sm:pt-16 pb-20 sm:pb-24">
      <!-- Hero -->
      <div
        class="text-center mb-10 sm:mb-12 home-hero"
        role="banner"
      >
        <h1 class="font-display text-4xl sm:text-5xl lg:text-6xl font-normal text-[var(--nc-text)] tracking-tight mb-3">
          North Cloud
        </h1>
        <p class="text-lg sm:text-xl text-[var(--nc-text-secondary)] max-w-xl mx-auto home-hero-sub">
          Search across our knowledge and content.
        </p>
      </div>

      <!-- Search bar -->
      <div class="mb-10 home-search">
        <SearchBar
          v-model="query"
          @search="handleSearch"
        />
      </div>

      <!-- Suggested topics -->
      <div
        v-if="suggestedTopics.length > 0"
        class="mb-8 home-suggested"
      >
        <p class="text-center text-sm font-medium text-[var(--nc-text-muted)] mb-3">
          Suggested topics
        </p>
        <div class="flex flex-wrap justify-center gap-2">
          <button
            v-for="(topic, i) in suggestedTopics"
            :key="topic.query"
            type="button"
            class="inline-flex items-center rounded-full bg-[var(--nc-bg-muted)] px-4 py-2 text-sm font-medium text-[var(--nc-text)] hover:bg-[var(--nc-border)] focus:outline-none focus:ring-2 focus:ring-[var(--nc-primary)] focus:ring-offset-2 transition-colors duration-[var(--nc-duration)] home-topic"
            :style="{ animationDelay: `${120 + i * 40}ms` }"
            @click="searchSuggested(topic.query)"
          >
            {{ topic.label }}
          </button>
        </div>
      </div>

      <!-- Recent searches -->
      <div
        v-if="recentSearches.length > 0"
        class="mb-8 home-recent"
      >
        <p class="text-center text-sm font-medium text-[var(--nc-text-muted)] mb-3">
          Recent searches
        </p>
        <div class="flex flex-wrap justify-center gap-2">
          <button
            v-for="(term, i) in recentSearches"
            :key="term"
            type="button"
            class="inline-flex items-center rounded-full border border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] px-4 py-2 text-sm text-[var(--nc-text-secondary)] hover:border-[var(--nc-primary)] hover:text-[var(--nc-primary)] focus:outline-none focus:ring-2 focus:ring-[var(--nc-primary)] focus:ring-offset-2 transition-colors duration-[var(--nc-duration)] home-recent-item"
            :style="{ animationDelay: `${80 + i * 30}ms` }"
            @click="searchSuggested(term)"
          >
            {{ term }}
          </button>
        </div>
      </div>

      <div class="text-center home-link">
        <router-link
          to="/advanced"
          class="text-sm font-medium text-[var(--nc-primary)] hover:text-[var(--nc-primary-hover)] underline underline-offset-2 transition-colors duration-[var(--nc-duration)]"
        >
          Advanced search
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import SearchBar from '@/components/search/SearchBar.vue'
import { getRecentSearches } from '@/composables/useRecentSearches'

const router = useRouter()
const query = ref('')

const suggestedTopics = [
  { query: 'crime', label: 'Crime' },
  { query: 'local news', label: 'Local News' },
  { query: 'mining', label: 'Mining' },
  { query: 'violent crime', label: 'Violent Crime' },
  { query: 'property crime', label: 'Property Crime' },
]

const recentSearches = ref<string[]>([])

onMounted(() => {
  recentSearches.value = getRecentSearches().slice(0, 6)
})

function handleSearch(searchQuery: string): void {
  if (searchQuery.trim()) {
    router.push({
      path: '/search',
      query: { q: searchQuery.trim() },
    })
  }
}

function searchSuggested(q: string): void {
  query.value = q
  router.push({
    path: '/search',
    query: { q: q.trim() },
  })
}
</script>

<style scoped>
.home-hero {
  animation: homeReveal var(--nc-duration-slow) var(--nc-ease-out) both;
}
.home-hero-sub {
  animation: homeReveal var(--nc-duration-slow) var(--nc-ease-out) 60ms both;
}
.home-search {
  animation: homeReveal var(--nc-duration-slow) var(--nc-ease-out) 120ms both;
}
.home-suggested {
  animation: homeReveal var(--nc-duration-slow) var(--nc-ease-out) 200ms both;
}
.home-recent {
  animation: homeReveal var(--nc-duration-slow) var(--nc-ease-out) 280ms both;
}
.home-link {
  animation: homeReveal var(--nc-duration-slow) var(--nc-ease-out) 340ms both;
}
.home-topic,
.home-recent-item {
  animation: homeReveal var(--nc-duration-slow) var(--nc-ease-out) both;
}
@keyframes homeReveal {
  from {
    opacity: 0;
    transform: translateY(12px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>
