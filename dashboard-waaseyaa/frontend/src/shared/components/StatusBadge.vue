<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  status: string
}>()

const colorMap: Record<string, string> = {
  active: 'bg-green-900/50 text-green-400',
  running: 'bg-green-900/50 text-green-400',
  healthy: 'bg-green-900/50 text-green-400',
  completed: 'bg-blue-900/50 text-blue-400',
  paused: 'bg-amber-900/50 text-amber-400',
  pending: 'bg-slate-700/50 text-slate-400',
  error: 'bg-red-900/50 text-red-400',
  failed: 'bg-red-900/50 text-red-400',
  disabled: 'bg-slate-700/50 text-slate-500',
}

const classes = computed(() => colorMap[props.status] || 'bg-slate-700/50 text-slate-400')
</script>

<template>
  <span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium" :class="classes">
    {{ status }}
  </span>
</template>
