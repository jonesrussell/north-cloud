import { ref, computed } from 'vue'

export function usePagination(defaultPerPage = 20) {
  const page = ref(1)
  const perPage = ref(defaultPerPage)

  const offset = computed(() => (page.value - 1) * perPage.value)

  function goToPage(p: number) {
    page.value = Math.max(1, p)
  }

  function nextPage() {
    page.value++
  }

  function prevPage() {
    if (page.value > 1) page.value--
  }

  return { page, perPage, offset, goToPage, nextPage, prevPage }
}
