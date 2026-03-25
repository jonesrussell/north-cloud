<script setup lang="ts">
const props = withDefaults(defineProps<{
  lines?: number
}>(), {
  lines: 3,
})
</script>

<template>
  <div class="space-y-3">
    <div
      v-for="i in lines"
      :key="i"
      class="h-4 bg-slate-800 rounded animate-pulse"
      :style="{ width: `${60 + Math.random() * 40}%` }"
    />
  </div>
</template>
