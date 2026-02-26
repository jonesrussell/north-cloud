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

    <div class="lg:flex lg:gap-6">
      <!-- Desktop: category sidebar (left rail) -->
      <aside
        v-if="query"
        class="hidden lg:block lg:w-48 lg:shrink-0"
        aria-label="Categories"
      >
        <div class="sticky top-24">
          <nav class="space-y-1">
            <button
              v-for="cat in categories"
              :key="cat.slug"
              type="button"
              class="w-full text-left px-3 py-2 rounded-lg text-sm font-medium transition-colors"
              :class="isCategoryActive(cat.slug)
                ? 'bg-[var(--nc-primary-muted)] text-[var(--nc-primary)] border-l-2 border-[var(--nc-primary)]'
                : 'text-[var(--nc-text-secondary)] hover:text-[var(--nc-text)] hover:bg-[var(--nc-bg-muted)]'"
              style="transition-duration: var(--nc-duration-fast)"
              @click="selectCategory(cat.slug)"
            >
              {{ cat.label }}
            </button>
          </nav>
        </div>
      </aside>

      <!-- Main content -->
      <main class="min-w-0 flex-1">
        <!-- Ordering pills row -->
        <div
          v-if="query"
          class="flex flex-wrap items-center gap-2 mb-4"
        >
          <span class="text-xs font-medium text-[var(--nc-text-muted)] uppercase tracking-wider mr-1">Order by</span>

          <!-- Best match pill -->
          <button
            type="button"
            class="rounded-full px-4 py-1.5 text-sm font-medium transition-colors"
            :class="sortBy === 'relevance'
              ? 'bg-[var(--nc-primary)] text-white'
              : 'bg-transparent border border-[var(--nc-border)] text-[var(--nc-text-secondary)] hover:border-[var(--nc-border-strong)]'"
            style="transition-duration: var(--nc-duration-fast)"
            @click="setSortBy('relevance')"
          >
            Best match
          </button>

          <!-- Most fresh pill -->
          <button
            type="button"
            class="rounded-full px-4 py-1.5 text-sm font-medium transition-colors"
            :class="sortBy === 'published_date'
              ? 'bg-[var(--nc-primary)] text-white'
              : 'bg-transparent border border-[var(--nc-border)] text-[var(--nc-text-secondary)] hover:border-[var(--nc-border-strong)]'"
            style="transition-duration: var(--nc-duration-fast)"
            @click="setSortBy('published_date')"
          >
            Most fresh
          </button>

          <!-- Time range dropdown -->
          <div class="relative">
            <button
              type="button"
              class="rounded-full px-4 py-1.5 text-sm font-medium bg-transparent border border-[var(--nc-border)] text-[var(--nc-text-secondary)] hover:border-[var(--nc-border-strong)] transition-colors"
              style="transition-duration: var(--nc-duration-fast)"
              @click="showTimeDropdown = !showTimeDropdown"
            >
              {{ activeTimeLabel }}
              <svg class="inline-block ml-1 h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7" />
              </svg>
            </button>
            <div
              v-if="showTimeDropdown"
              class="absolute z-40 mt-1 w-44 rounded-lg border border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] shadow-[var(--nc-shadow-lg)] py-1"
            >
              <button
                v-for="opt in timeOptions"
                :key="opt.label"
                type="button"
                class="w-full text-left px-4 py-2 text-sm text-[var(--nc-text-secondary)] hover:bg-[var(--nc-bg-muted)] hover:text-[var(--nc-text)] transition-colors"
                style="transition-duration: var(--nc-duration-fast)"
                @click="applyTimeRange(opt)"
              >
                {{ opt.label }}
              </button>
            </div>
          </div>

          <!-- Spacer -->
          <div class="flex-1" />

          <!-- Filters dropdown button (desktop) -->
          <div class="relative hidden lg:block">
            <button
              type="button"
              class="rounded-full px-4 py-1.5 text-sm font-medium bg-transparent border border-[var(--nc-border)] text-[var(--nc-text-secondary)] hover:border-[var(--nc-border-strong)] transition-colors"
              style="transition-duration: var(--nc-duration-fast)"
              @click="showFiltersPanel = !showFiltersPanel"
            >
              <svg class="inline-block mr-1 h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
              </svg>
              Filters
            </button>
            <div
              v-if="showFiltersPanel"
              class="absolute right-0 z-40 mt-2 w-80 rounded-xl border border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] shadow-[var(--nc-shadow-lg)] p-4"
            >
              <FilterSidebar
                :facets="facets"
                :filters="filters"
                @update:filters="onFiltersUpdate"
              />
            </div>
          </div>
        </div>

        <!-- Desktop: filter chips -->
        <div
          v-if="query && hasActiveFilters"
          class="mb-4 hidden lg:block"
        >
          <FilterChips
            :filters="filters"
            @remove-topic="removeTopic"
            @remove-source="removeSource"
            @clear-content-type="clearContentType"
            @clear-min-quality="clearMinQuality"
            @clear-dates="clearDates"
          />
        </div>

        <!-- Mobile: horizontal category scroll strip -->
        <div
          v-if="query"
          class="mb-4 lg:hidden overflow-x-auto flex gap-2"
        >
          <button
            v-for="cat in categories"
            :key="cat.slug"
            type="button"
            class="rounded-full px-3 py-1.5 text-sm font-medium whitespace-nowrap transition-colors"
            :class="isCategoryActive(cat.slug)
              ? 'bg-[var(--nc-primary)] text-white'
              : 'bg-[var(--nc-bg-surface)] text-[var(--nc-text-secondary)] border border-[var(--nc-border)]'"
            style="transition-duration: var(--nc-duration-fast)"
            @click="selectCategory(cat.slug)"
          >
            {{ cat.label }}
          </button>
        </div>

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
              class="inline-flex items-center rounded-lg border border-transparent bg-[var(--nc-primary)] px-3 py-1.5 text-sm font-medium text-white hover:bg-[var(--nc-primary-hover)] focus:outline-none focus:ring-2 focus:ring-[var(--nc-primary)] focus:ring-offset-2 transition-colors duration-[var(--nc-duration)]"
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
const showTimeDropdown = ref(false)
const showFiltersPanel = ref(false)

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
  sortBy,
  search,
  syncFromUrl,
  changePage,
  applyFilters,
  clearFilters,
} = useSearch()

const firstResult = computed(() => results.value[0] ?? null)

const hasActiveFilters = computed(() => {
  const f = filters.value
  return (f.topics && f.topics.length > 0)
    || (f.source_names && f.source_names.length > 0)
    || f.content_type
    || (f.min_quality_score && f.min_quality_score > 0)
    || f.from_date
    || f.to_date
})

interface Category {
  label: string
  slug: string
}

const categories: Category[] = [
  { label: 'All', slug: '' },
  { label: 'Top Stories', slug: 'top_stories' },
  { label: 'Crime', slug: 'crime' },
  { label: 'Mining', slug: 'mining' },
  { label: 'Entertainment', slug: 'entertainment' },
]

interface TimeOption {
  label: string
  hours: number | null
}

const timeOptions: TimeOption[] = [
  { label: 'Any time', hours: null },
  { label: 'Past hour', hours: 1 },
  { label: 'Past 24 hours', hours: 24 },
  { label: 'Past 7 days', hours: 168 },
  { label: 'Past 30 days', hours: 720 },
]

const activeTimeLabel = computed(() => {
  if (!filters.value.from_date) return 'Any time'
  const fromMs = new Date(filters.value.from_date).getTime()
  const nowMs = Date.now()
  const diffHours = Math.round((nowMs - fromMs) / 3600000)
  if (diffHours <= 2) return 'Past hour'
  if (diffHours <= 25) return 'Past 24 hours'
  if (diffHours <= 170) return 'Past 7 days'
  return 'Past 30 days'
})

function isCategoryActive(slug: string): boolean {
  const topics = filters.value.topics ?? []
  if (!slug) return topics.length === 0
  return topics.length === 1 && topics[0] === slug
}

function selectCategory(slug: string): void {
  filters.value.topics = slug ? [slug] : []
  currentPage.value = 1
  applyFilters()
}

function setSortBy(field: string): void {
  sortBy.value = field
  search()
}

function applyTimeRange(opt: TimeOption): void {
  if (opt.hours === null) {
    filters.value.from_date = null
  } else {
    const d = new Date(Date.now() - opt.hours * 3600000)
    filters.value.from_date = d.toISOString()
  }
  showTimeDropdown.value = false
  applyFilters()
}

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

// Close dropdowns on click outside
function handleGlobalClick(e: MouseEvent): void {
  const target = e.target as HTMLElement
  if (!target.closest('[class*="relative"]')) {
    showTimeDropdown.value = false
    showFiltersPanel.value = false
  }
}

onMounted(() => {
  syncFromUrl()
  if (query.value) {
    search()
  }
  document.addEventListener('click', handleGlobalClick)
})

watch(() => route.query, () => {
  syncFromUrl()
  if (query.value) {
    search()
  }
})
</script>
