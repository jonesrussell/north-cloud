import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { getAllNavigationItems } from '@/config/navigation'
import type { NavigationItem } from '@/config/navigation'

interface SearchResult extends NavigationItem {
  score: number
}

export function useCommandPalette() {
  const router = useRouter()
  const isOpen = ref(false)
  const searchQuery = ref('')

  /**
   * Fuzzy match search query against navigation items
   */
  const searchResults = computed<SearchResult[]>(() => {
    if (!searchQuery.value.trim()) {
      // Return all items when no query
      return getAllNavigationItems()
        .filter((item) => !item.external) // Exclude external links
        .map((item) => ({ ...item, score: 1 }))
    }

    const query = searchQuery.value.toLowerCase().trim()
    const items = getAllNavigationItems().filter((item) => !item.external)

    // Score each item based on fuzzy match
    const scored = items
      .map((item) => {
        let score = 0
        const labelLower = item.label.toLowerCase()
        const pathLower = item.path.toLowerCase()
        const descLower = item.description?.toLowerCase() || ''

        // Exact match in label (highest score)
        if (labelLower === query) {
          score += 100
        }
        // Starts with query in label
        else if (labelLower.startsWith(query)) {
          score += 50
        }
        // Contains query in label
        else if (labelLower.includes(query)) {
          score += 30
        }

        // Match in path
        if (pathLower.includes(query)) {
          score += 20
        }

        // Match in description
        if (descLower.includes(query)) {
          score += 10
        }

        // Fuzzy character matching (partial word match)
        const queryChars = query.split('')
        let charIndex = 0
        for (const char of labelLower) {
          if (char === queryChars[charIndex]) {
            charIndex++
            score += 1 // Small bonus for each matched character in order
          }
          if (charIndex === queryChars.length) {
            break
          }
        }

        return { ...item, score }
      })
      .filter((item) => item.score > 0)
      .sort((a, b) => b.score - a.score)
      .slice(0, 8) // Limit to top 8 results

    return scored
  })

  /**
   * Open command palette
   */
  const open = () => {
    isOpen.value = true
    searchQuery.value = ''
  }

  /**
   * Close command palette
   */
  const close = () => {
    isOpen.value = false
    searchQuery.value = ''
  }

  /**
   * Navigate to selected item and close palette
   */
  const navigateTo = (path: string) => {
    router.push(path)
    close()
  }

  /**
   * Handle keyboard shortcut (Cmd+K / Ctrl+K)
   */
  const handleKeydown = (event: KeyboardEvent) => {
    // Cmd+K (Mac) or Ctrl+K (Windows/Linux)
    if ((event.metaKey || event.ctrlKey) && event.key === 'k') {
      event.preventDefault()
      if (isOpen.value) {
        close()
      } else {
        open()
      }
    }

    // Escape to close
    if (event.key === 'Escape' && isOpen.value) {
      close()
    }
  }

  // Register keyboard listener
  onMounted(() => {
    window.addEventListener('keydown', handleKeydown)
  })

  onUnmounted(() => {
    window.removeEventListener('keydown', handleKeydown)
  })

  return {
    isOpen,
    searchQuery,
    searchResults,
    open,
    close,
    navigateTo,
  }
}
