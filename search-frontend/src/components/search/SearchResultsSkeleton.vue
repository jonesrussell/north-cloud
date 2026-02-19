<template>
  <div
    class="space-y-5"
    aria-busy="true"
    aria-label="Loading results"
  >
    <div
      v-for="i in count"
      :key="i"
      class="rounded-xl border border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] p-5 sm:p-6 animate-pulse"
    >
      <div class="h-5 bg-[var(--nc-bg-muted)] rounded-lg w-3/4 mb-2" />
      <div class="h-4 bg-[var(--nc-bg-muted)] rounded w-1/2 mb-3" />
      <div class="space-y-2 mb-3">
        <div class="h-4 bg-[var(--nc-bg-muted)] rounded w-full" />
        <div class="h-4 bg-[var(--nc-bg-muted)] rounded w-full" />
        <div class="h-4 bg-[var(--nc-bg-muted)] rounded w-2/3" />
      </div>
      <div class="flex gap-4">
        <div class="h-4 bg-[var(--nc-bg-muted)] rounded w-24" />
        <div class="h-4 bg-[var(--nc-bg-muted)] rounded w-20" />
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
