<script setup lang="ts">
import { useRoute } from 'vue-router'

const route = useRoute()

const navGroups = [
  {
    label: 'Home',
    items: [{ name: 'Pipeline Overview', route: '/' }],
  },
  {
    label: 'Content Intake',
    items: [
      { name: 'Sources', route: '/sources' },
      { name: 'Crawl Jobs', route: '/crawl-jobs' },
    ],
  },
]

function isActive(path: string): boolean {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}
</script>

<template>
  <nav class="w-56 bg-slate-950 border-r border-slate-800 flex flex-col min-h-screen shrink-0">
    <div class="px-4 py-3 text-blue-500 font-bold text-lg border-b border-slate-800">
      North Cloud
    </div>
    <div v-for="group in navGroups" :key="group.label" class="mt-3">
      <div class="px-4 py-1 text-amber-500 text-xs uppercase tracking-wider">
        {{ group.label }}
      </div>
      <RouterLink
        v-for="item in group.items"
        :key="item.route"
        :to="item.route"
        class="block px-4 py-2 text-sm transition-colors"
        :class="isActive(item.route)
          ? 'text-slate-100 bg-slate-800 border-l-3 border-blue-500'
          : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900'"
      >
        {{ item.name }}
      </RouterLink>
    </div>
  </nav>
</template>
