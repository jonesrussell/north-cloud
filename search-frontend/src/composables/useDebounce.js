import { ref, watch } from 'vue'

/**
 * Debounce a reactive value
 * @param {Ref} value - Reactive value to debounce
 * @param {Number} delay - Delay in milliseconds (default: 300)
 * @returns {Ref} - Debounced reactive value
 */
export function useDebounce(value, delay = 300) {
  const debouncedValue = ref(value.value)
  let timeout = null

  watch(value, (newValue) => {
    clearTimeout(timeout)
    timeout = setTimeout(() => {
      debouncedValue.value = newValue
    }, delay)
  })

  return debouncedValue
}

export default useDebounce
