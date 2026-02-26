<template>
  <div
    class="space-y-4"
    aria-busy="true"
    aria-label="Loading results"
  >
    <div
      v-for="i in count"
      :key="i"
      class="rounded-xl border border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] p-5 animate-pulse"
    >
      <div class="flex gap-4">
        <!-- Content skeleton -->
        <div class="flex-1 min-w-0">
          <div class="h-4 bg-[var(--nc-bg-muted)] rounded w-1/3 mb-3" />
          <div class="h-5 bg-[var(--nc-bg-muted)] rounded-lg w-3/4 mb-2" />
          <div class="space-y-2 mb-3">
            <div class="h-4 bg-[var(--nc-bg-muted)] rounded w-full" />
            <div class="h-4 bg-[var(--nc-bg-muted)] rounded w-2/3" />
          </div>
          <div class="flex gap-2">
            <div class="h-5 bg-[var(--nc-bg-muted)] rounded-md w-16" />
            <div class="h-5 bg-[var(--nc-bg-muted)] rounded-md w-20" />
          </div>
        </div>
        <!-- Thumbnail skeleton -->
        <div class="hidden sm:block flex-shrink-0 w-[120px] h-[90px] rounded-lg bg-[var(--nc-bg-muted)]" />
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
