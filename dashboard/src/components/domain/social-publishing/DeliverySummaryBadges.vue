<script setup lang="ts">
import type { DeliverySummary } from '@/types/socialPublisher'

defineProps<{
  summary: DeliverySummary
}>()
</script>

<template>
  <div class="flex items-center gap-1.5">
    <span
      v-if="summary.delivered > 0"
      class="inline-flex items-center rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700 dark:bg-green-900/30 dark:text-green-400"
    >
      {{ summary.delivered }} delivered
    </span>
    <span
      v-if="summary.failed > 0"
      class="inline-flex items-center rounded-full bg-red-100 px-2 py-0.5 text-xs font-medium text-red-700 dark:bg-red-900/30 dark:text-red-400"
    >
      {{ summary.failed }} failed
    </span>
    <span
      v-if="summary.pending > 0"
      class="inline-flex items-center rounded-full bg-yellow-100 px-2 py-0.5 text-xs font-medium text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400"
    >
      {{ summary.pending }} pending
    </span>
    <span
      v-if="summary.retrying > 0"
      class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
    >
      {{ summary.retrying }} retrying
    </span>
    <span
      v-if="summary.total === 0"
      class="text-xs text-muted-foreground"
    >
      No deliveries
    </span>
  </div>
</template>
