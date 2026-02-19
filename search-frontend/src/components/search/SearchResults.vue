<template>
  <div
    ref="listRef"
    class="space-y-5"
    role="list"
    aria-label="Search results"
    tabindex="-1"
    @keydown="onKeydown"
  >
    <SearchResultItem
      v-for="(result, index) in results"
      :key="result.id"
      :result="result"
      :featured="index === 0"
      :aria-posinset="index + 1"
      :aria-setsize="results.length"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import SearchResultItem from './SearchResultItem.vue'
import type { SearchResult } from '@/types/search'

interface Props {
  results: SearchResult[]
}

defineProps<Props>()

const listRef = ref<HTMLElement | null>(null)

function onKeydown(e: KeyboardEvent): void {
  if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return
  const list = listRef.value
  if (!list) return
  const links = Array.from(list.querySelectorAll<HTMLAnchorElement>('a[href]'))
  const current = document.activeElement as HTMLAnchorElement | null
  const i = links.indexOf(current ?? undefined)
  if (i < 0) return
  e.preventDefault()
  if (e.key === 'ArrowDown' && i < links.length - 1) {
    links[i + 1].focus()
  }
  if (e.key === 'ArrowUp' && i > 0) {
    links[i - 1].focus()
  }
}
</script>
