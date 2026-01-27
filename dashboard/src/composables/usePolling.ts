import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useDocumentVisibility } from '@vueuse/core'

export interface PollingOptions {
  /** Start polling immediately on mount (default: true) */
  immediate?: boolean
  /** Pause polling when document is hidden (default: true) */
  pauseWhenHidden?: boolean
  /** Custom error handler */
  onError?: (error: Error) => void
}

/**
 * Smart polling composable with pause/resume on visibility change
 *
 * @param callback - Async function to call on each poll
 * @param interval - Polling interval in milliseconds (default: 30000)
 * @param options - Polling options
 */
export function usePolling(
  callback: () => Promise<void>,
  interval: number = 30000,
  options: PollingOptions = {}
) {
  const {
    immediate = true,
    pauseWhenHidden = true,
    onError,
  } = options

  const isPolling = ref(false)
  const lastUpdate = ref<Date | null>(null)
  const error = ref<Error | null>(null)
  const isPaused = ref(false)

  let timerId: ReturnType<typeof setInterval> | null = null
  const visibility = useDocumentVisibility()

  async function poll() {
    if (isPaused.value) return

    try {
      error.value = null
      await callback()
      lastUpdate.value = new Date()
    } catch (err) {
      const errorObj = err instanceof Error ? err : new Error(String(err))
      error.value = errorObj
      onError?.(errorObj)
    }
  }

  function start() {
    if (isPolling.value) return

    isPolling.value = true
    isPaused.value = false

    // Immediate first call
    poll()

    // Set up interval
    timerId = setInterval(poll, interval)
  }

  function stop() {
    if (timerId) {
      clearInterval(timerId)
      timerId = null
    }
    isPolling.value = false
  }

  function pause() {
    isPaused.value = true
  }

  function resume() {
    isPaused.value = false
    // Immediately poll on resume
    poll()
  }

  async function refresh() {
    await poll()
  }

  // Auto-pause when document is hidden
  if (pauseWhenHidden) {
    watch(visibility, (visible) => {
      if (visible === 'visible' && isPolling.value) {
        resume()
      } else if (visible === 'hidden' && isPolling.value) {
        pause()
      }
    })
  }

  // Auto-start if immediate
  onMounted(() => {
    if (immediate) {
      start()
    }
  })

  // Cleanup on unmount
  onUnmounted(() => {
    stop()
  })

  return {
    isPolling,
    isPaused,
    lastUpdate,
    error,
    start,
    stop,
    pause,
    resume,
    refresh,
  }
}
