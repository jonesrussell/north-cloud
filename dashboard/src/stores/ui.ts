import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export interface Toast {
  id: string
  type: 'success' | 'error' | 'warning' | 'info'
  title?: string
  message: string
  duration?: number
  action?: {
    label: string
    onClick: () => void
  }
}

const DEFAULT_TOAST_DURATION = 5000 // 5 seconds

export const useUIStore = defineStore('ui', () => {
  // State
  const modals = ref<Record<string, boolean>>({})
  const toasts = ref<Toast[]>([])
  const featureFlags = ref<Record<string, boolean>>({
    realtimeUpdates: import.meta.env.VITE_ENABLE_REALTIME === 'true',
    unifiedHealth: import.meta.env.VITE_ENABLE_UNIFIED_HEALTH !== 'false', // Default enabled
    virtualTables: import.meta.env.VITE_ENABLE_VIRTUAL_TABLES === 'true',
  })

  // Toast counter for unique IDs
  let toastCounter = 0

  // Track toast timers for cleanup
  const toastTimers = new Map<string, ReturnType<typeof setTimeout>>()

  // Getters
  const hasActiveModal = computed(() => Object.values(modals.value).some(Boolean))
  const activeToasts = computed(() => toasts.value)

  // Actions
  function showModal(id: string) {
    modals.value[id] = true
  }

  function hideModal(id: string) {
    modals.value[id] = false
  }

  function toggleModal(id: string) {
    modals.value[id] = !modals.value[id]
  }

  function isModalOpen(id: string): boolean {
    return modals.value[id] ?? false
  }

  function showToast(options: Omit<Toast, 'id'>): string {
    const id = `toast-${++toastCounter}`
    const toast: Toast = {
      id,
      ...options,
      duration: options.duration ?? DEFAULT_TOAST_DURATION,
    }

    toasts.value.push(toast)

    // Auto-dismiss after duration (track timer for cleanup)
    if (toast.duration && toast.duration > 0) {
      const timerId = setTimeout(() => {
        toastTimers.delete(id)
        dismissToast(id)
      }, toast.duration)
      toastTimers.set(id, timerId)
    }

    return id
  }

  function dismissToast(id: string) {
    // Clear auto-dismiss timer if exists
    const timerId = toastTimers.get(id)
    if (timerId) {
      clearTimeout(timerId)
      toastTimers.delete(id)
    }

    const index = toasts.value.findIndex((t) => t.id === id)
    if (index !== -1) {
      toasts.value.splice(index, 1)
    }
  }

  function clearAllToasts() {
    // Clear all pending timers
    for (const timerId of toastTimers.values()) {
      clearTimeout(timerId)
    }
    toastTimers.clear()
    toasts.value = []
  }

  // Convenience methods for different toast types
  function success(message: string, title?: string) {
    return showToast({ type: 'success', message, title })
  }

  function showError(message: string, title?: string) {
    return showToast({ type: 'error', message, title, duration: 8000 }) // Longer duration for errors
  }

  function warning(message: string, title?: string) {
    return showToast({ type: 'warning', message, title })
  }

  function info(message: string, title?: string) {
    return showToast({ type: 'info', message, title })
  }

  // Feature flags
  function isFeatureEnabled(feature: string): boolean {
    // Check localStorage override first
    const storedFlags = localStorage.getItem('north-cloud-features')
    if (storedFlags) {
      try {
        const parsed = JSON.parse(storedFlags)
        if (feature in parsed) return parsed[feature]
      } catch {
        // Ignore parse errors
      }
    }
    return featureFlags.value[feature] ?? false
  }

  function setFeatureFlag(feature: string, enabled: boolean) {
    featureFlags.value[feature] = enabled
    // Persist to localStorage
    const storedFlags = localStorage.getItem('north-cloud-features')
    let flags: Record<string, boolean> = {}
    if (storedFlags) {
      try {
        flags = JSON.parse(storedFlags)
      } catch {
        // Reset corrupted storage
        flags = {}
      }
    }
    flags[feature] = enabled
    localStorage.setItem('north-cloud-features', JSON.stringify(flags))
  }

  function $reset() {
    modals.value = {}
    // Clear all timers before resetting toasts
    for (const timerId of toastTimers.values()) {
      clearTimeout(timerId)
    }
    toastTimers.clear()
    toasts.value = []
  }

  return {
    // State
    modals,
    toasts,
    featureFlags,

    // Getters
    hasActiveModal,
    activeToasts,

    // Modal actions
    showModal,
    hideModal,
    toggleModal,
    isModalOpen,

    // Toast actions
    showToast,
    dismissToast,
    clearAllToasts,
    success,
    error: showError,
    warning,
    info,

    // Feature flags
    isFeatureEnabled,
    setFeatureFlag,

    $reset,
  }
})
