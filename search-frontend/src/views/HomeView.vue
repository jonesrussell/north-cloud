<template>
  <div class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8">
    <div class="pt-20 pb-16">
      <!-- Hero Section -->
      <div class="text-center mb-12">
        <h1 class="text-5xl font-bold text-gray-900 mb-4">
          North Cloud
        </h1>
      </div>

      <!-- Search Bar -->
      <div class="mb-8">
        <SearchBar
          v-model="query"
          @search="handleSearch"
        />
      </div>

      <!-- Quick Links -->
      <div class="text-center text-sm text-gray-600">
        <router-link to="/advanced" class="text-blue-600 hover:text-blue-800 underline">
          Advanced Search
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import SearchBar from '@/components/search/SearchBar.vue'

const router = useRouter()
const query = ref('')

const handleSearch = (searchQuery) => {
  if (searchQuery.trim()) {
    router.push({
      path: '/search',
      query: { q: searchQuery.trim() }
    })
  }
}
</script>
