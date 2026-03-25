<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  panelId: string
  vars?: Record<string, string>
  height?: string
}>(), {
  height: '300px',
})

const grafanaUrl = import.meta.env.VITE_GRAFANA_URL || 'http://localhost:3000'

const src = computed(() => {
  const params = new URLSearchParams({ theme: 'dark' })
  if (props.vars) {
    for (const [key, value] of Object.entries(props.vars)) {
      params.set(`var-${key}`, value)
    }
  }
  return `${grafanaUrl}/d-solo/${props.panelId}?${params}`
})
</script>

<template>
  <iframe
    :src="src"
    :style="{ height }"
    class="w-full border border-slate-700 rounded-lg"
    frameborder="0"
  />
</template>
