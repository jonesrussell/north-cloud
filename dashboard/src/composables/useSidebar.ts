import { ref, computed } from 'vue'
import { useStorage, useMediaQuery } from '@vueuse/core'

const STORAGE_KEY = 'north-cloud-sidebar-collapsed'

// Shared state across all instances
const isCollapsed = useStorage(STORAGE_KEY, false)
const isMobileOpen = ref(false)

export function useSidebar() {
  const isMobile = useMediaQuery('(max-width: 768px)')

  // Sidebar width based on state
  const sidebarWidth = computed(() => {
    if (isMobile.value) return '0px'
    return isCollapsed.value ? '4rem' : '16rem'
  })

  // Toggle collapsed state
  const toggle = () => {
    if (isMobile.value) {
      isMobileOpen.value = !isMobileOpen.value
    } else {
      isCollapsed.value = !isCollapsed.value
    }
  }

  // Collapse sidebar
  const collapse = () => {
    isCollapsed.value = true
  }

  // Expand sidebar
  const expand = () => {
    isCollapsed.value = false
  }

  // Open mobile sidebar
  const openMobile = () => {
    isMobileOpen.value = true
  }

  // Close mobile sidebar
  const closeMobile = () => {
    isMobileOpen.value = false
  }

  return {
    isCollapsed,
    isMobileOpen,
    isMobile,
    sidebarWidth,
    toggle,
    collapse,
    expand,
    openMobile,
    closeMobile,
  }
}
