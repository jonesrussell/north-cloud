import { ref, watch, onMounted } from 'vue'
import { usePreferredDark, useStorage } from '@vueuse/core'

export type Theme = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'north-cloud-theme'

// Shared state across all instances
const theme = ref<Theme>('system')
const isDark = ref(false)

let initialized = false

export function useTheme() {
  const prefersDark = usePreferredDark()
  const storedTheme = useStorage<Theme>(STORAGE_KEY, 'system')

  // Initialize only once
  if (!initialized) {
    theme.value = storedTheme.value
    initialized = true
  }

  // Update isDark based on current theme and system preference
  const updateIsDark = () => {
    if (theme.value === 'system') {
      isDark.value = prefersDark.value
    } else {
      isDark.value = theme.value === 'dark'
    }
  }

  // Apply theme to document
  const applyTheme = () => {
    updateIsDark()
    if (isDark.value) {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
  }

  // Set theme
  const setTheme = (newTheme: Theme) => {
    theme.value = newTheme
    storedTheme.value = newTheme
    applyTheme()
  }

  // Toggle between light and dark (not system)
  const toggleTheme = () => {
    if (isDark.value) {
      setTheme('light')
    } else {
      setTheme('dark')
    }
  }

  // Cycle through all themes: light -> dark -> system
  const cycleTheme = () => {
    const themes: Theme[] = ['light', 'dark', 'system']
    const currentIndex = themes.indexOf(theme.value)
    const nextIndex = (currentIndex + 1) % themes.length
    setTheme(themes[nextIndex])
  }

  // Watch for system preference changes
  watch(prefersDark, () => {
    if (theme.value === 'system') {
      applyTheme()
    }
  })

  // Initialize on mount
  onMounted(() => {
    applyTheme()
  })

  return {
    theme,
    isDark,
    setTheme,
    toggleTheme,
    cycleTheme,
  }
}
