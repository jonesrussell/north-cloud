<template>
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
    <!-- Search Bar -->
    <div class="mb-8">
      <SearchBar
        v-model="query"
        @search="handleSearch"
      />
    </div>

    <!-- Loading State -->
    <LoadingSpinner v-if="loading" />

    <!-- Error State -->
    <ErrorAlert
      v-else-if="hasError"
      title="Search Error"
      :message="error"
    />

    <!-- Empty State -->
    <EmptyState
      v-else-if="isEmpty"
      title="No results found"
      message="Try different keywords or adjust your search filters"
    />

    <!-- Results -->
    <div v-else-if="hasResults">
      <!-- Results Info -->
      <div class="mb-4 text-sm text-gray-600">
        About {{ totalHits.toLocaleString() }} results
      </div>

      <!-- Results List -->
      <div class="mb-8">
        <SearchResults :results="results" />
      </div>

      <!-- Pagination -->
      <SearchPagination
        :current-page="currentPage"
        :total-pages="totalPages"
        :total-hits="totalHits"
        :page-size="pageSize"
        @page-change="changePage"
      />
    </div>
  </div>
</template>

<script setup>
import { onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useSearch } from '@/composables/useSearch'
import SearchBar from '@/components/search/SearchBar.vue'
import SearchResults from '@/components/search/SearchResults.vue'
import SearchPagination from '@/components/search/SearchPagination.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import ErrorAlert from '@/components/common/ErrorAlert.vue'
import EmptyState from '@/components/common/EmptyState.vue'

const route = useRoute()
const {
  query,
  results,
  totalHits,
  currentPage,
  totalPages,
  pageSize,
  loading,
  error,
  hasResults,
  hasError,
  isEmpty,
  search,
  syncFromUrl,
  changePage,
} = useSearch()

// Handle search from search bar
const handleSearch = (searchQuery) => {
  query.value = searchQuery
  currentPage.value = 1
  search()
}

// Sync from URL on mount
onMounted(() => {
  syncFromUrl()
  if (query.value) {
    search()
  }
})

// Watch for URL changes (back/forward navigation)
watch(() => route.query, () => {
  syncFromUrl()
  if (query.value) {
    search()
  }
})
</script>
