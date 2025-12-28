import { ref, watch, type Ref } from 'vue'

/**
 * Debounce a reactive value
 * @param value - Reactive value to debounce
 * @param delay - Delay in milliseconds (default: 300)
 * @returns Debounced reactive value
 */
export function useDebounce<T>(value: Ref<T>, delay = 300): Ref<T> {
  const debouncedValue = ref(value.value) as Ref<T>
  let timeout: ReturnType<typeof setTimeout> | null = null

  watch(value, (newValue: T) => {
    if (timeout) {
      clearTimeout(timeout)
    }
    timeout = setTimeout(() => {
      debouncedValue.value = newValue
    }, delay)
  })

  return debouncedValue
}

export default useDebounce

