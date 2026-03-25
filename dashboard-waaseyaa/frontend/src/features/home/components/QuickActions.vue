<script setup lang="ts">
const grafanaUrl = import.meta.env.VITE_GRAFANA_URL || 'http://localhost:3000'
</script>

<template>
  <div class="flex flex-wrap gap-3">
    <RouterLink
      to="/sources/new"
      class="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white text-sm font-medium rounded-lg transition-colors"
    >
      Add Source
    </RouterLink>

    <button
      disabled
      class="px-4 py-2 bg-slate-700 text-slate-500 text-sm font-medium rounded-lg cursor-not-allowed"
    >
      Review Queue (Phase 3)
    </button>

    <a
      :href="grafanaUrl"
      target="_blank"
      rel="noopener noreferrer"
      class="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-slate-200 text-sm font-medium rounded-lg transition-colors"
    >
      Open Grafana
    </a>
  </div>
</template>
