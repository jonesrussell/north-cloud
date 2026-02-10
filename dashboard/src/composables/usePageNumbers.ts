import { computed, type Ref } from 'vue'

/**
 * Generates a page number sequence for pagination UI.
 * Returns an array of page numbers and ellipsis strings.
 *
 * @example
 * // totalPages=10, page=5 → [1, '...', 4, 5, 6, '...', 10]
 */
export function usePageNumbers(
  page: Ref<number>,
  totalPages: Ref<number>
): { pageNumbers: ReturnType<typeof computed> } {
  const pageNumbers = computed(() => {
    const current = page.value
    const total = totalPages.value
    const pages: (number | string)[] = []

    if (total <= 7) {
      for (let i = 1; i <= total; i++) pages.push(i)
    } else {
      pages.push(1)
      if (current > 3) pages.push('...')
      for (let i = Math.max(2, current - 1); i <= Math.min(total - 1, current + 1); i++) {
        pages.push(i)
      }
      if (current < total - 2) pages.push('...')
      pages.push(total)
    }

    return pages
  })

  return { pageNumbers }
}
