<script setup lang="ts">
withDefaults(defineProps<{
  label: string
  value: number | string
  subtitle?: string
  subtitleColor?: 'green' | 'amber' | 'red'
}>(), {
  subtitleColor: 'green',
})

const colorMap: Record<string, string> = {
  green: 'text-green-400',
  amber: 'text-amber-400',
  red: 'text-red-400',
}
</script>

<template>
  <div class="bg-slate-800 border border-slate-700 rounded-lg p-5">
    <p class="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-2">
      {{ label }}
    </p>
    <p class="text-3xl font-bold text-slate-100">
      {{ value }}
    </p>
    <p
      v-if="subtitle"
      class="text-sm mt-1"
      :class="colorMap[subtitleColor ?? 'green']"
    >
      {{ subtitle }}
    </p>
  </div>
</template>
