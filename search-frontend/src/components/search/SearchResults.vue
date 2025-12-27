<template>
  <div class="space-y-4">
    <SearchResultItem
      v-for="result in results"
      :key="result.id"
      :result="result"
    />
  </div>
</template>

<script setup>
import SearchResultItem from './SearchResultItem.vue'

defineProps({
  results: {
    type: Array,
    required: true,
  },
})
</script>
