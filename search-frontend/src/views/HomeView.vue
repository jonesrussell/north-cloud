<template>
  <div class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8">
    <div class="pt-20 pb-16">
      <!-- Hero Section -->
      <div class="text-center mb-10">
        <h1 class="text-5xl font-bold text-gray-900 mb-3">
          North Cloud
        </h1>
        <p class="text-lg text-gray-600 max-w-xl mx-auto">
          Search across North Cloud's knowledge and content.
        </p>
      </div>

      <!-- Search Bar -->
      <div class="mb-10">
        <SearchBar
          v-model="query"
          @search="handleSearch"
        />
      </div>

      <!-- Suggested topics -->
      <div
        v-if="suggestedTopics.length > 0"
        class="mb-8"
      >
        <p class="text-center text-sm font-medium text-gray-500 mb-3">
          Suggested topics
        </p>
        <div class="flex flex-wrap justify-center gap-2">
          <button
            v-for="topic in suggestedTopics"
            :key="topic.query"
            type="button"
            class="inline-flex items-center rounded-full bg-gray-100 px-4 py-2 text-sm text-gray-700 hover:bg-gray-200 focus:outline-none focus:ring-2 focus:ring-blue-500"
            @click="searchSuggested(topic.query)"
          >
            {{ topic.label }}
          </button>
        </div>
      </div>

      <!-- Recent searches -->
      <div
        v-if="recentSearches.length > 0"
        class="mb-8"
      >
        <p class="text-center text-sm font-medium text-gray-500 mb-3">
          Recent searches
        </p>
        <div class="flex flex-wrap justify-center gap-2">
          <button
            v-for="term in recentSearches"
            :key="term"
            type="button"
            class="inline-flex items-center rounded-full border border-gray-200 bg-white px-4 py-2 text-sm text-gray-600 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500"
            @click="searchSuggested(term)"
          >
            {{ term }}
          </button>
        </div>
      </div>

      <!-- Quick Links -->
      <div class="text-center text-sm text-gray-600">
        <router-link
          to="/advanced"
          class="text-blue-600 hover:text-blue-800 underline"
        >
          Advanced Search
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
