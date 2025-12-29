<template>
  <div class="mt-6">
    <!-- Section Header with Tooltip -->
    <div class="group relative px-3">
      <h3 class="text-xs font-semibold text-gray-400 uppercase tracking-wider flex items-center">
        <component
          :is="section.icon"
          class="h-4 w-4 mr-2"
        />
        {{ section.label }}
      </h3>
      <!-- Tooltip -->
      <div
        v-if="section.description"
        class="absolute left-full ml-2 top-0 z-50 hidden group-hover:block w-64 px-3 py-2 text-sm text-white bg-gray-800 rounded-md shadow-lg border border-gray-700"
      >
        {{ section.description }}
        <div class="absolute left-0 top-1/2 -translate-x-1 -translate-y-1/2 w-2 h-2 bg-gray-800 border-l border-b border-gray-700 rotate-45" />
      </div>
    </div>

    <!-- Navigation Items -->
    <div class="mt-2 space-y-1">
      <template
        v-for="item in section.items"
        :key="item.path"
      >
        <!-- External Link -->
        <a
          v-if="item.external"
          :href="item.path"
          target="_blank"
          rel="noopener noreferrer"
          :title="item.description"
          class="group flex items-center px-3 py-2 text-sm font-medium rounded-md transition-colors text-gray-300 hover:bg-gray-800 hover:text-white"
        >
          <component
            :is="item.icon"
            class="mr-3 h-5 w-5 flex-shrink-0"
          />
          {{ item.label }}
        </a>

        <!-- Internal Router Link -->
        <router-link
          v-else
          :to="item.path"
          :title="item.description"
          class="group flex items-center px-3 py-2 text-sm font-medium rounded-md transition-colors"
          :class="[
            isItemActive(item)
              ? 'bg-gray-800 text-white'
              : 'text-gray-300 hover:bg-gray-800 hover:text-white'
          ]"
        >
          <component
            :is="item.icon"
            class="mr-3 h-5 w-5 flex-shrink-0"
          />
          {{ item.label }}
        </router-link>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRoute } from 'vue-router'
import type { NavigationSection, NavigationItem } from '@/config/navigation'

defineProps<{
  section: NavigationSection
}>()

const route = useRoute()

const isItemActive = (item: NavigationItem): boolean => {
  if (item.exact) {
    return route.path === item.path
  }
  return route.path.startsWith(item.path)
}
</script>

<style scoped>
/* Smooth transitions for navigation */
a {
  transition: background-color 0.15s ease-in-out, color 0.15s ease-in-out;
}
</style>
