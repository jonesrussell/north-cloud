import { ref, onMounted } from 'vue'
import type { Ref } from 'vue'
import type { FeedItem } from '@/types/search'
import { feedApi } from '@/api/search'
import axios from 'axios'

const DEBUG = import.meta.env.DEV

interface UseFeedReturn {
  items: Ref<FeedItem[]>
  loading: Ref<boolean>
  error: Ref<boolean>
  refresh: () => Promise<void>
}

/**
 * Composable for fetching articles from public feed endpoints.
 * @param slug - Optional topic slug. If provided, fetches topic-specific feed; otherwise fetches latest.
 */
export function useFeed(slug?: string): UseFeedReturn {
  const items: Ref<FeedItem[]> = ref([])
  const loading: Ref<boolean> = ref(false)
  const error: Ref<boolean> = ref(false)

  async function refresh(): Promise<void> {
    loading.value = true
    error.value = false

    try {
      const response = slug
        ? await feedApi.byTopic(slug)
        : await feedApi.latest()

      items.value = response.data.items
    } catch (err: unknown) {
      error.value = true
      items.value = []

      if (DEBUG) {
        if (axios.isAxiosError(err)) {
          console.error('[Feed] Error:', err.response?.status, err.response?.data || err.message)
        } else {
          console.error('[Feed] Error:', err)
        }
      }
    } finally {
      loading.value = false
    }
  }

  onMounted(refresh)

  return { items, loading, error, refresh }
}

export default useFeed
