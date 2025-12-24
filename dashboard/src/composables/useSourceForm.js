import { ref, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { sourcesApi } from '../api/client'
import { 
  stringToArray, 
  arrayToString, 
  extractErrorMessage, 
  normalizeSelectors, 
  mergeSelectors 
} from '../utils/formHelpers'

/**
 * Composable for managing source form state and operations
 */
export function useSourceForm() {
  const router = useRouter()
  const route = useRoute()
  
  const isEdit = computed(() => !!route.params.id)
  
  const form = ref({
    name: '',
    url: '',
    rate_limit: '1s',
    max_depth: 2,
    time: [],
    selectors: {
      article: {},
      list: {},
      page: {},
    },
    enabled: true,
  })
  
  const loading = ref(false)
  const submitting = ref(false)
  const error = ref(null)
  const fetchingMetadata = ref(false)
  const metadataFetched = ref(false)
  
  // Exclude input refs for comma-separated strings
  const articleExcludeInput = ref('')
  const listExcludeInput = ref('')
  const pageExcludeInput = ref('')
  
  // Collapsible section states
  const showArticleSelectors = ref(false)
  const showListSelectors = ref(false)
  const showPageSelectors = ref(false)
  
  /**
   * Initializes exclude input from form selectors
   */
  const initExcludeInputs = () => {
    if (form.value.selectors.article?.exclude) {
      articleExcludeInput.value = arrayToString(form.value.selectors.article.exclude)
    }
    if (form.value.selectors.list?.exclude_from_list) {
      listExcludeInput.value = arrayToString(form.value.selectors.list.exclude_from_list)
    }
    if (form.value.selectors.page?.exclude) {
      pageExcludeInput.value = arrayToString(form.value.selectors.page.exclude)
    }
  }
  
  /**
   * Fetches metadata from URL and prefills form
   */
  const fetchMetadata = async () => {
    if (!form.value.url) return
    
    fetchingMetadata.value = true
    error.value = null
    metadataFetched.value = false
    
    try {
      const response = await sourcesApi.fetchMetadata(form.value.url)
      const metadata = response.data
      
      if (metadata.name) {
        form.value.name = metadata.name
      }
      
      if (metadata.selectors) {
        form.value.selectors = mergeSelectors(form.value.selectors, metadata.selectors)
        
        // Initialize exclude inputs
        if (metadata.selectors.article?.exclude) {
          articleExcludeInput.value = arrayToString(metadata.selectors.article.exclude)
        }
        if (metadata.selectors.list?.exclude_from_list) {
          listExcludeInput.value = arrayToString(metadata.selectors.list.exclude_from_list)
        }
        if (metadata.selectors.page?.exclude) {
          pageExcludeInput.value = arrayToString(metadata.selectors.page.exclude)
        }
        
        // Auto-open sections with data
        if (metadata.selectors.article && Object.keys(metadata.selectors.article).length > 0) {
          showArticleSelectors.value = true
        }
        if (metadata.selectors.list && Object.keys(metadata.selectors.list).length > 0) {
          showListSelectors.value = true
        }
        if (metadata.selectors.page && Object.keys(metadata.selectors.page).length > 0) {
          showPageSelectors.value = true
        }
      }
      
      metadataFetched.value = true
    } catch (err) {
      error.value = extractErrorMessage(err, 'Failed to fetch metadata')
      metadataFetched.value = false
      console.error('[useSourceForm] Error fetching metadata:', err)
    } finally {
      fetchingMetadata.value = false
    }
  }
  
  /**
   * Loads source data for editing
   */
  const loadSource = async () => {
    if (!isEdit.value) return
    
    loading.value = true
    error.value = null
    
    try {
      const response = await sourcesApi.get(route.params.id)
      const source = response.data
      
      source.selectors = normalizeSelectors(source.selectors)
      form.value = { ...source }
      
      // Initialize exclude inputs
      initExcludeInputs()
    } catch (err) {
      error.value = extractErrorMessage(err, 'Failed to load source')
      console.error('[useSourceForm] Error loading source:', err)
    } finally {
      loading.value = false
    }
  }
  
  /**
   * Handles form submission
   */
  const handleSubmit = async () => {
    submitting.value = true
    error.value = null
    
    try {
      const data = { ...form.value }
      
      if (isEdit.value) {
        await sourcesApi.update(route.params.id, data)
      } else {
        await sourcesApi.create(data)
      }
      
      router.push('/sources')
    } catch (err) {
      error.value = extractErrorMessage(err, 'Failed to save source')
      console.error('[useSourceForm] Error saving source:', err)
    } finally {
      submitting.value = false
    }
  }
  
  return {
    // State
    form,
    isEdit,
    loading,
    submitting,
    error,
    fetchingMetadata,
    metadataFetched,
    articleExcludeInput,
    listExcludeInput,
    pageExcludeInput,
    showArticleSelectors,
    showListSelectors,
    showPageSelectors,
    
    // Methods
    fetchMetadata,
    loadSource,
    handleSubmit,
  }
}

