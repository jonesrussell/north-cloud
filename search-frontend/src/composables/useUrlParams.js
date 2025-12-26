import { watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'

/**
 * Sync state with URL query parameters
 * @param {Object} state - Reactive state object
 * @param {Array} keys - Keys to sync with URL
 * @returns {Object} - Helper functions
 */
export function useUrlParams(state, keys = []) {
  const router = useRouter()
  const route = useRoute()

  /**
   * Parse URL params to state
   */
  const syncFromUrl = () => {
    keys.forEach((key) => {
      const value = route.query[key]
      if (value !== undefined) {
        // Handle arrays (e.g., topics=crime,news)
        if (Array.isArray(state[key])) {
          state[key] = typeof value === 'string' ? value.split(',') : value
        }
        // Handle numbers
        else if (typeof state[key] === 'number') {
          state[key] = parseInt(value) || state[key]
        }
        // Handle booleans
        else if (typeof state[key] === 'boolean') {
          state[key] = value === 'true'
        }
        // Handle strings
        else {
          state[key] = value
        }
      }
    })
  }

  /**
   * Update URL with current state
   */
  const updateUrl = () => {
    const query = {}
    keys.forEach((key) => {
      const value = state[key]
      if (value !== undefined && value !== null && value !== '' && (!Array.isArray(value) || value.length > 0)) {
        // Arrays to comma-separated strings
        query[key] = Array.isArray(value) ? value.join(',') : value
      }
    })
    router.push({ query })
  }

  /**
   * Watch state changes and update URL
   */
  const watchState = () => {
    keys.forEach((key) => {
      watch(() => state[key], () => {
        updateUrl()
      }, { deep: true })
    })
  }

  return {
    syncFromUrl,
    updateUrl,
    watchState,
  }
}

export default useUrlParams
