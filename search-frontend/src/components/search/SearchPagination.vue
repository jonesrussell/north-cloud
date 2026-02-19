<template>
  <div class="flex items-center justify-between border-t border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] px-4 py-3 sm:px-6 rounded-b-xl">
    <div class="flex flex-1 justify-between sm:hidden">
      <button
        :disabled="currentPage === 1"
        class="relative inline-flex items-center rounded-lg border border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] px-4 py-2 text-sm font-medium text-[var(--nc-text)] hover:bg-[var(--nc-bg-muted)] disabled:opacity-50 disabled:cursor-not-allowed transition-colors duration-[var(--nc-duration)]"
        @click="goToPrevious"
      >
        Previous
      </button>
      <button
        :disabled="currentPage === totalPages"
        class="relative ml-3 inline-flex items-center rounded-lg border border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] px-4 py-2 text-sm font-medium text-[var(--nc-text)] hover:bg-[var(--nc-bg-muted)] disabled:opacity-50 disabled:cursor-not-allowed transition-colors duration-[var(--nc-duration)]"
        @click="goToNext"
      >
        Next
      </button>
    </div>

    <div class="hidden sm:flex sm:flex-1 sm:items-center sm:justify-between">
      <p class="text-sm text-[var(--nc-text-secondary)]">
        Showing
        <span class="font-medium text-[var(--nc-text)]">{{ startResult }}</span>
        to
        <span class="font-medium text-[var(--nc-text)]">{{ endResult }}</span>
        of
        <span class="font-medium text-[var(--nc-text)]">{{ totalHits }}</span>
        results
      </p>
      <nav
        class="isolate inline-flex -space-x-px rounded-lg shadow-sm ring-1 ring-[var(--nc-border)] overflow-hidden"
        aria-label="Pagination"
      >
        <button
          :disabled="currentPage === 1"
          class="relative inline-flex items-center rounded-l-lg px-2 py-2 text-[var(--nc-text-muted)] bg-[var(--nc-bg-elevated)] hover:bg-[var(--nc-bg-muted)] focus:z-20 focus:outline-none focus:ring-2 focus:ring-[var(--nc-primary)] disabled:opacity-50 disabled:cursor-not-allowed transition-colors duration-[var(--nc-duration)]"
          @click="goToPrevious"
        >
          <span class="sr-only">Previous</span>
          <svg
            class="h-5 w-5"
            viewBox="0 0 20 20"
            fill="currentColor"
            aria-hidden="true"
          >
            <path
              fill-rule="evenodd"
              d="M12.79 5.23a.75.75 0 01-.02 1.06L8.832 10l3.938 3.71a.75.75 0 11-1.04 1.08l-4.5-4.25a.75.75 0 010-1.08l4.5-4.25a.75.75 0 011.06.02z"
              clip-rule="evenodd"
            />
          </svg>
        </button>

        <button
          v-for="page in visiblePages"
          :key="page"
          :class="[
            page === currentPage
              ? 'z-10 bg-[var(--nc-primary)] text-white focus-visible:outline focus-visible:ring-2 focus-visible:ring-[var(--nc-primary)]'
              : 'bg-[var(--nc-bg-elevated)] text-[var(--nc-text)] hover:bg-[var(--nc-bg-muted)] focus:outline-none focus:ring-2 focus:ring-[var(--nc-primary)]',
            'relative inline-flex items-center px-4 py-2 text-sm font-semibold border-l border-[var(--nc-border)] first:border-l-0 transition-colors duration-[var(--nc-duration)]'
          ]"
          @click="goToPage(page)"
        >
          {{ page }}
        </button>

        <button
          :disabled="currentPage === totalPages"
          class="relative inline-flex items-center rounded-r-lg px-2 py-2 text-[var(--nc-text-muted)] bg-[var(--nc-bg-elevated)] hover:bg-[var(--nc-bg-muted)] focus:z-20 focus:outline-none focus:ring-2 focus:ring-[var(--nc-primary)] disabled:opacity-50 disabled:cursor-not-allowed transition-colors duration-[var(--nc-duration)]"
          @click="goToNext"
        >
          <span class="sr-only">Next</span>
          <svg
            class="h-5 w-5"
            viewBox="0 0 20 20"
            fill="currentColor"
            aria-hidden="true"
          >
            <path
              fill-rule="evenodd"
              d="M7.21 14.77a.75.75 0 01.02-1.06L11.168 10 7.23 6.29a.75.75 0 111.04-1.08l4.5 4.25a.75.75 0 010 1.08l-4.5 4.25a.75.75 0 01-1.06-.02z"
              clip-rule="evenodd"
            />
          </svg>
        </button>
      </nav>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  currentPage: number
  totalPages: number
  totalHits: number
  pageSize?: number
}

const props = withDefaults(defineProps<Props>(), {
  pageSize: 20,
})

const emit = defineEmits<{
  'page-change': [page: number]
}>()

const startResult = computed((): number => {
  return (props.currentPage - 1) * props.pageSize + 1
})

const endResult = computed((): number => {
  return Math.min(props.currentPage * props.pageSize, props.totalHits)
})

const visiblePages = computed((): number[] => {
  const pages: number[] = []
  const maxVisible = 5
  let start = Math.max(1, props.currentPage - Math.floor(maxVisible / 2))
  let end = Math.min(props.totalPages, start + maxVisible - 1)
  if (end - start + 1 < maxVisible) {
    start = Math.max(1, end - maxVisible + 1)
  }
  for (let i = start; i <= end; i++) {
    pages.push(i)
  }
  return pages
})

const goToPage = (page: number): void => {
  emit('page-change', page)
}

const goToPrevious = (): void => {
  if (props.currentPage > 1) {
    emit('page-change', props.currentPage - 1)
  }
}

const goToNext = (): void => {
  if (props.currentPage < props.totalPages) {
    emit('page-change', props.currentPage + 1)
  }
}
</script>
