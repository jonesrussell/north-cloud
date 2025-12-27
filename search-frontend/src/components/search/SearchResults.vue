<template>
  <div class="space-y-4">
    <SearchResultItem
      v-for="result in results"
      :key="result.id"
      :result="result"
    />
  </div>
</template>

<script setup lang="ts">
import SearchResultItem from './SearchResultItem.vue'
import type { SearchResult } from '@/types/search'

interface Props {
  results: SearchResult[]
}

defineProps<Props>()
</script>
