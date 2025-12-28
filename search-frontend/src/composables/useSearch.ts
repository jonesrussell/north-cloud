import { ref, computed, type Ref, type ComputedRef } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import searchApi from '@/api/search'
import type { SearchResult, SearchFilters, Facet } from '@/types/search'

/**
 * Search state management composable
 * Handles search execution, state management, and URL synchronization
 */
export function useSearch() {
  const router = useRouter()
  const route = useRoute()

  // Core search state
  const query: Ref<string> = ref('')
  const results: Ref<SearchResult[]> = ref([])
  const facets: Ref<Facet | null> = ref(null)
  const totalHits: Ref<number> = ref(0)
  const currentPage: Ref<number> = ref(1)
  const pageSize: Ref<number> = ref(20)
  const loading: Ref<boolean> = ref(false)
  const error: Ref<string | null> = ref(null)

  // Filter state
  const filters: Ref<SearchFilters> = ref({
    topics: [],
    content_type: null,
    min_quality_score: 0,
    from_date: null,
    to_date: null,
    source_names: [],
  })

  // Sorting state
  const sortBy: Ref<string> = ref('relevance')
  const sortOrder: Ref<'asc' | 'desc'> = ref('desc')

  // Computed properties
  const totalPages: ComputedRef<number> = computed(() => Math.ceil(totalHits.value / pageSize.value))
  const hasResults: ComputedRef<boolean> = computed(() => results.value.length > 0)
  const hasError: ComputedRef<boolean> = computed(() => error.value !== null)
  const isEmpty: ComputedRef<boolean> = computed(() => !loading.value && !hasResults.value && query.value !== '')

  /**
   * Execute search with current state
   */
  const search = async (): Promise<void> => {
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
          topics: filters.value.topics && filters.value.topics.length > 0 ? filters.value.topics : undefined,
          content_type: filters.value.content_type || undefined,
          min_quality_score: filters.value.min_quality_score && filters.value.min_quality_score > 0 ? filters.value.min_quality_score : undefined,
          from_date: filters.value.from_date || undefined,
          to_date: filters.value.to_date || undefined,
          source_names: filters.value.source_names && filters.value.source_names.length > 0 ? filters.value.source_names : undefined,
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
      if (err && typeof err === 'object' && 'response' in err) {
        const axiosError = err as { response?: { data?: { error?: string } }; message?: string }
        error.value = axiosError.response?.data?.error || axiosError.message || 'Search failed. Please try again.'
      } else if (err instanceof Error) {
        error.value = err.message || 'Search failed. Please try again.'
      } else {
        error.value = 'Search failed. Please try again.'
      }
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
  const syncFromUrl = (): void => {
    query.value = (route.query.q as string) || ''
    currentPage.value = parseInt(route.query.page as string) || 1
    sortBy.value = (route.query.sort as string) || 'relevance'
    sortOrder.value = (route.query.order as 'asc' | 'desc') || 'desc'

    // Parse filters from URL
    if (route.query.topics) {
      filters.value.topics = typeof route.query.topics === 'string'
        ? route.query.topics.split(',')
        : (route.query.topics as string[])
    }
    if (route.query.content_type) {
      filters.value.content_type = route.query.content_type as string
    }
    if (route.query.min_quality_score) {
      filters.value.min_quality_score = parseInt(route.query.min_quality_score as string) || 0
    }
    if (route.query.from_date) {
      filters.value.from_date = route.query.from_date as string
    }
    if (route.query.to_date) {
      filters.value.to_date = route.query.to_date as string
    }
    if (route.query.sources) {
      filters.value.source_names = typeof route.query.sources === 'string'
        ? route.query.sources.split(',')
        : (route.query.sources as string[])
    }
  }

  /**
   * Update URL with current search state
   */
  const updateUrl = (): void => {
    const query_params: Record<string, string | number | undefined> = {
      q: query.value || undefined,
      page: currentPage.value > 1 ? currentPage.value : undefined,
      sort: sortBy.value !== 'relevance' ? sortBy.value : undefined,
      order: sortOrder.value !== 'desc' ? sortOrder.value : undefined,
      topics: filters.value.topics && filters.value.topics.length > 0 ? filters.value.topics.join(',') : undefined,
      content_type: filters.value.content_type || undefined,
      min_quality_score: filters.value.min_quality_score && filters.value.min_quality_score > 0 ? filters.value.min_quality_score : undefined,
      from_date: filters.value.from_date || undefined,
      to_date: filters.value.to_date || undefined,
      sources: filters.value.source_names && filters.value.source_names.length > 0 ? filters.value.source_names.join(',') : undefined,
    }

    // Remove undefined values
    const cleanQuery = Object.fromEntries(
      Object.entries(query_params).filter(([, v]) => v !== undefined)
    )

    router.push({ path: '/search', query: cleanQuery })
  }

  /**
   * Reset all filters
   */
  const clearFilters = (): void => {
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
  const changePage = (page: number): void => {
    currentPage.value = page
    search()
    // Scroll to top
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  /**
   * Apply filters and search
   */
  const applyFilters = (): void => {
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

