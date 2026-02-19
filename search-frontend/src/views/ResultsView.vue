<template>
  <div class="max-w-6xl mx-auto px-4 sm:px-6 lg:px-8 py-6 sm:py-8">
    <!-- Search bar -->
    <div class="mb-6">
      <SearchBar
        v-model="query"
        @search="handleSearch"
      />
    </div>

    <!-- Mobile: filter chips + drawer trigger -->
    <div
      v-if="query"
      class="mb-4 lg:hidden"
    >
      <FilterChips
        :filters="filters"
        drawer-id="filter-drawer"
        @open-drawer="showFilterDrawer = true"
        @remove-topic="removeTopic"
        @remove-source="removeSource"
        @clear-content-type="clearContentType"
        @clear-min-quality="clearMinQuality"
        @clear-dates="clearDates"
      />
    </div>

    <div class="lg:flex lg:gap-8">
      <!-- Desktop: left sidebar -->
      <aside
        v-if="query"
        class="hidden lg:block lg:w-64 lg:shrink-0"
        aria-label="Refine search"
      >
        <div class="sticky top-24 space-y-6">
          <FilterSidebar
            :facets="facets"
            :filters="filters"
            @update:filters="onFiltersUpdate"
          />
          <RelatedContent
            v-if="hasResults"
            :facets="facets"
            :first-result="firstResult"
            @topic-click="applyRelatedTopic"
            @source-click="applyRelatedSource"
          />
        </div>
      </aside>

      <!-- Main content -->
      <main class="min-w-0 flex-1">
        <SearchResultsSkeleton
          v-if="loading"
          :count="pageSize"
        />

        <ErrorAlert
          v-else-if="hasError"
          title="Search error"
          :message="error ?? 'Something went wrong.'"
        >
          <template #actions>
            <button
              type="button"
              class="inline-flex items-center rounded-lg border border-transparent bg-[var(--nc-accent)] px-3 py-1.5 text-sm font-medium text-white hover:bg-[var(--nc-accent-hover)] focus:outline-none focus:ring-2 focus:ring-[var(--nc-accent)] focus:ring-offset-2 transition-colors duration-[var(--nc-duration)]"
              @click="search"
            >
              Try again
            </button>
          </template>
        </ErrorAlert>

        <EmptySearchState
          v-else-if="isEmpty"
          :query="query"
          :filters="filters"
          :facets="facets"
          :clear-filters="clearFilters"
          @suggested-topic="applySuggestedTopic"
        />

        <div v-else-if="hasResults">
          <div class="mb-4 text-sm text-[var(--nc-text-secondary)]">
            About {{ totalHits.toLocaleString() }} results
          </div>

          <div class="mb-8">
            <SearchResults :results="results" />
          </div>

          <SearchPagination
            :current-page="currentPage"
            :total-pages="totalPages"
            :total-hits="totalHits"
            :page-size="pageSize"
            @page-change="changePage"
          />

          <RelatedContent
            v-if="hasResults"
            class="mt-8 lg:hidden"
            :facets="facets"
            :first-result="firstResult"
            @topic-click="applyRelatedTopic"
            @source-click="applyRelatedSource"
          />
        </div>
      </main>
    </div>

    <FilterDrawer
      :open="showFilterDrawer"
      @close="showFilterDrawer = false"
    >
      <FilterSidebar
        :facets="facets"
        :filters="filters"
        @update:filters="onFiltersUpdateAndClose"
      />
    </FilterDrawer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useSearch } from '@/composables/useSearch'
import SearchBar from '@/components/search/SearchBar.vue'
import SearchResults from '@/components/search/SearchResults.vue'
import SearchResultsSkeleton from '@/components/search/SearchResultsSkeleton.vue'
import SearchPagination from '@/components/search/SearchPagination.vue'
import FilterSidebar from '@/components/search/FilterSidebar.vue'
import FilterChips from '@/components/search/FilterChips.vue'
import FilterDrawer from '@/components/search/FilterDrawer.vue'
import RelatedContent from '@/components/search/RelatedContent.vue'
import EmptySearchState from '@/components/search/EmptySearchState.vue'
import ErrorAlert from '@/components/common/ErrorAlert.vue'
import { trackEvent } from '@/utils/analytics'
import type { SearchFilters } from '@/types/search'

const route = useRoute()
const showFilterDrawer = ref(false)

const {
  query,
  results,
  facets,
  filters,
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
  applyFilters,
  clearFilters,
} = useSearch()

const firstResult = computed(() => results.value[0] ?? null)

function applyRelatedTopic(topic: string): void {
  const current = filters.value.topics ?? []
  if (current.includes(topic)) return
  filters.value.topics = [...current, topic]
  currentPage.value = 1
  applyFilters()
}

function applyRelatedSource(source: string): void {
  filters.value.source_names = [source]
  currentPage.value = 1
  applyFilters()
}

function applySuggestedTopic(topic: string): void {
  filters.value.topics = [topic]
  currentPage.value = 1
  applyFilters()
}

function onFiltersUpdate(newFilters: SearchFilters): void {
  Object.assign(filters.value, newFilters)
  trackEvent('filter_change', {
    filter_type: 'full',
    filter_value: newFilters,
  })
  applyFilters()
}

function onFiltersUpdateAndClose(newFilters: SearchFilters): void {
  onFiltersUpdate(newFilters)
  showFilterDrawer.value = false
}

function removeTopic(topic: string): void {
  filters.value.topics = (filters.value.topics ?? []).filter((t) => t !== topic)
  applyFilters()
}

function removeSource(source: string): void {
  filters.value.source_names = (filters.value.source_names ?? []).filter((s) => s !== source)
  applyFilters()
}

function clearContentType(): void {
  filters.value.content_type = null
  applyFilters()
}

function clearMinQuality(): void {
  filters.value.min_quality_score = 0
  applyFilters()
}

function clearDates(): void {
  filters.value.from_date = null
  filters.value.to_date = null
  applyFilters()
}

function handleSearch(searchQuery: string): void {
  query.value = searchQuery
  currentPage.value = 1
  search()
}

onMounted(() => {
  syncFromUrl()
  if (query.value) {
    search()
  }
})

watch(() => route.query, () => {
  syncFromUrl()
  if (query.value) {
    search()
  }
})
</script>
