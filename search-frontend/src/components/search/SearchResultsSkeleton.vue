<template>
  <div class="space-y-4" aria-busy="true" aria-label="Loading results">
    <div
      v-for="i in count"
      :key="i"
      class="bg-white p-6 rounded-lg shadow-sm animate-pulse"
    >
      <div class="h-6 bg-gray-200 rounded w-3/4 mb-2" />
      <div class="h-4 bg-gray-100 rounded w-1/2 mb-3" />
      <div class="space-y-2 mb-3">
        <div class="h-4 bg-gray-100 rounded w-full" />
        <div class="h-4 bg-gray-100 rounded w-full" />
        <div class="h-4 bg-gray-100 rounded w-2/3" />
      </div>
      <div class="flex gap-4">
        <div class="h-4 bg-gray-100 rounded w-24" />
        <div class="h-4 bg-gray-100 rounded w-20" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    count?: number
  }>(),
  { count: 5 }
)
</script>
