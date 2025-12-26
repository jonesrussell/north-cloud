import { ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import searchApi from '@/api/search'

/**
 * Search state management composable
 * Handles search execution, state management, and URL synchronization
 */
export function useSearch() {
  const router = useRouter()
  const route = useRoute()

  // Core search state
  const query = ref('')
  const results = ref([])
  const facets = ref(null)
  const totalHits = ref(0)
  const currentPage = ref(1)
  const pageSize = ref(20)
  const loading = ref(false)
  const error = ref(null)

  // Filter state
  const filters = ref({
    topics: [],
    content_type: null,
    min_quality_score: 0,
    from_date: null,
    to_date: null,
    source_names: [],
  })

  // Sorting state
  const sortBy = ref('relevance')
  const sortOrder = ref('desc')

  // Computed properties
  const totalPages = computed(() => Math.ceil(totalHits.value / pageSize.value))
  const hasResults = computed(() => results.value.length > 0)
  const hasError = computed(() => error.value !== null)
  const isEmpty = computed(() => !loading.value && !hasResults.value && query.value !== '')

  /**
   * Execute search with current state
   */
  const search = async () => {
    if (!query.value.trim()) {
      results.value = []
      return
    }

    loading.value = true
    error.value = null

    try {
      const payload = {
        query: query.value,
        filters: {
          ...filters.value,
          // Remove empty filters
          topics: filters.value.topics.length > 0 ? filters.value.topics : undefined,
          content_type: filters.value.content_type || undefined,
          min_quality_score: filters.value.min_quality_score > 0 ? filters.value.min_quality_score : undefined,
          from_date: filters.value.from_date || undefined,
          to_date: filters.value.to_date || undefined,
          source_names: filters.value.source_names.length > 0 ? filters.value.source_names : undefined,
        },
        pagination: {
          page: currentPage.value,
          size: pageSize.value,
        },
        sort: {
          field: sortBy.value,
          order: sortOrder.value,
        },
        options: {
          include_highlights: true,
          include_facets: true,
        },
      }

      const response = await searchApi.search(payload)
      results.value = response.data.hits || []
      facets.value = response.data.facets || null
      totalHits.value = response.data.total_hits || 0

      // Update URL with current search params
      updateUrl()
    } catch (err) {
      console.error('Search error:', err)
      error.value = err.response?.data?.error || err.message || 'Search failed. Please try again.'
      results.value = []
      facets.value = null
      totalHits.value = 0
    } finally {
      loading.value = false
    }
  }

  /**
   * Parse URL parameters and update state
   */
  const syncFromUrl = () => {
    query.value = route.query.q || ''
    currentPage.value = parseInt(route.query.page) || 1
    sortBy.value = route.query.sort || 'relevance'
    sortOrder.value = route.query.order || 'desc'

    // Parse filters from URL
    if (route.query.topics) {
      filters.value.topics = typeof route.query.topics === 'string'
        ? route.query.topics.split(',')
        : route.query.topics
    }
    if (route.query.content_type) {
      filters.value.content_type = route.query.content_type
    }
    if (route.query.min_quality_score) {
      filters.value.min_quality_score = parseInt(route.query.min_quality_score) || 0
    }
    if (route.query.from_date) {
      filters.value.from_date = route.query.from_date
    }
    if (route.query.to_date) {
      filters.value.to_date = route.query.to_date
    }
    if (route.query.sources) {
      filters.value.source_names = typeof route.query.sources === 'string'
        ? route.query.sources.split(',')
        : route.query.sources
    }
  }

  /**
   * Update URL with current search state
   */
  const updateUrl = () => {
    const query_params = {
      q: query.value || undefined,
      page: currentPage.value > 1 ? currentPage.value : undefined,
      sort: sortBy.value !== 'relevance' ? sortBy.value : undefined,
      order: sortOrder.value !== 'desc' ? sortOrder.value : undefined,
      topics: filters.value.topics.length > 0 ? filters.value.topics.join(',') : undefined,
      content_type: filters.value.content_type || undefined,
      min_quality_score: filters.value.min_quality_score > 0 ? filters.value.min_quality_score : undefined,
      from_date: filters.value.from_date || undefined,
      to_date: filters.value.to_date || undefined,
      sources: filters.value.source_names.length > 0 ? filters.value.source_names.join(',') : undefined,
    }

    // Remove undefined values
    const cleanQuery = Object.fromEntries(
      Object.entries(query_params).filter(([_, v]) => v !== undefined)
    )

    router.push({ path: '/search', query: cleanQuery })
  }

  /**
   * Reset all filters
   */
  const clearFilters = () => {
    filters.value = {
      topics: [],
      content_type: null,
      min_quality_score: 0,
      from_date: null,
      to_date: null,
      source_names: [],
    }
    search()
  }

  /**
   * Change page
   */
  const changePage = (page) => {
    currentPage.value = page
    search()
    // Scroll to top
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  /**
   * Apply filters and search
   */
  const applyFilters = () => {
    currentPage.value = 1 // Reset to first page when filters change
    search()
  }

  return {
    // State
    query,
    results,
    facets,
    totalHits,
    currentPage,
    pageSize,
    loading,
    error,
    filters,
    sortBy,
    sortOrder,

    // Computed
    totalPages,
    hasResults,
    hasError,
    isEmpty,

    // Methods
    search,
    syncFromUrl,
    updateUrl,
    clearFilters,
    changePage,
    applyFilters,
  }
}

export default useSearch
