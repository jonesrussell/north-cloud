import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import type { Component } from 'vue'

export interface RecentPage {
  path: string
  title: string
  icon?: Component
  timestamp: number
}

const STORAGE_KEY = 'recent_pages'
const MAX_RECENT_PAGES = 5

// Shared reactive state across all instances
const recentPages = ref<RecentPage[]>([])

/**
 * Load recent pages from localStorage
 */
function loadRecentPages(): void {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      const parsed = JSON.parse(stored)
      recentPages.value = Array.isArray(parsed) ? parsed : []
    }
  } catch (error) {
    console.error('Failed to load recent pages:', error)
    recentPages.value = []
  }
}

/**
 * Save recent pages to localStorage
 */
function saveRecentPages(): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(recentPages.value))
  } catch (error) {
    console.error('Failed to save recent pages:', error)
  }
}

/**
 * Add a page to recent pages
 */
function addRecentPage(page: Omit<RecentPage, 'timestamp'>): void {
  // Don't add login page or empty paths
  if (!page.path || page.path === '/login') {
    return
  }

  // Remove existing entry for this path (if any)
  recentPages.value = recentPages.value.filter((p) => p.path !== page.path)

  // Add to beginning
  recentPages.value.unshift({
    ...page,
    timestamp: Date.now(),
  })

  // Keep only MAX_RECENT_PAGES
  if (recentPages.value.length > MAX_RECENT_PAGES) {
    recentPages.value = recentPages.value.slice(0, MAX_RECENT_PAGES)
  }

  saveRecentPages()
}

/**
 * Clear all recent pages
 */
function clearRecentPages(): void {
  recentPages.value = []
  saveRecentPages()
}

/**
 * Remove a specific recent page
 */
function removeRecentPage(path: string): void {
  recentPages.value = recentPages.value.filter((p) => p.path !== path)
  saveRecentPages()
}

/**
 * Composable for using recent pages
 */
export function useRecentPages() {
  const router = useRouter()

  // Load recent pages on first use
  if (recentPages.value.length === 0) {
    loadRecentPages()
  }

  /**
   * Navigate to a recent page
   */
  const navigateToRecentPage = (path: string) => {
    router.push(path)
  }

  /**
   * Get formatted time ago string
   */
  const getTimeAgo = (timestamp: number): string => {
    const now = Date.now()
    const diff = now - timestamp
    const seconds = Math.floor(diff / 1000)
    const minutes = Math.floor(seconds / 60)
    const hours = Math.floor(minutes / 60)
    const days = Math.floor(hours / 24)

    if (days > 0) {
      return `${days}d ago`
    }
    if (hours > 0) {
      return `${hours}h ago`
    }
    if (minutes > 0) {
      return `${minutes}m ago`
    }
    return 'just now'
  }

  /**
   * Recent pages with formatted time
   */
  const recentPagesWithTime = computed(() => {
    return recentPages.value.map((page) => ({
      ...page,
      timeAgo: getTimeAgo(page.timestamp),
    }))
  })

  return {
    recentPages: recentPagesWithTime,
    addRecentPage,
    clearRecentPages,
    removeRecentPage,
    navigateToRecentPage,
  }
}
