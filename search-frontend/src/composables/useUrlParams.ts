import { watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'

/**
 * Sync state with URL query parameters
 * @param state - Reactive state object
 * @param keys - Keys to sync with URL
 * @returns Helper functions
 */
export function useUrlParams<T extends Record<string, unknown>>(
  state: T,
  keys: (keyof T)[] = []
) {
  const router = useRouter()
  const route = useRoute()

  /**
   * Parse URL params to state
   */
  const syncFromUrl = (): void => {
    keys.forEach((key) => {
      const value = route.query[key as string]
      if (value !== undefined) {
        // Handle arrays (e.g., topics=crime,news)
        if (Array.isArray(state[key])) {
          state[key] = (typeof value === 'string' ? value.split(',') : value) as T[keyof T]
        }
        // Handle numbers
        else if (typeof state[key] === 'number') {
          state[key] = (parseInt(value as string) || state[key]) as T[keyof T]
        }
        // Handle booleans
        else if (typeof state[key] === 'boolean') {
          state[key] = (value === 'true') as T[keyof T]
        }
        // Handle strings
        else {
          state[key] = value as T[keyof T]
        }
      }
    })
  }

  /**
   * Update URL with current state
   */
  const updateUrl = (): void => {
    const query: Record<string, string | number | boolean> = {}
    keys.forEach((key) => {
      const value = state[key]
      if (value !== undefined && value !== null && value !== '' && (!Array.isArray(value) || value.length > 0)) {
        // Arrays to comma-separated strings
        query[key as string] = Array.isArray(value) ? value.join(',') : value as string | number | boolean
      }
    })
    router.push({ query })
  }

  /**
   * Watch state changes and update URL
   */
  const watchState = (): void => {
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

