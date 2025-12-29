<template>
  <div class="fixed inset-y-0 left-0 z-50 w-64 bg-gray-900">
    <!-- Logo -->
    <div class="flex h-16 items-center justify-center border-b border-gray-800">
      <CloudIcon class="h-8 w-8 text-blue-500" />
      <span class="ml-2 text-xl font-bold text-white">North Cloud</span>
    </div>

    <!-- Navigation -->
    <nav
      class="mt-6 px-3 overflow-y-auto"
      style="max-height: calc(100vh - 8rem);"
    >
      <!-- Render navigation sections from config -->
      <NavigationSection
        v-for="section in sections"
        :key="section.id"
        :section="section"
      />
    </nav>

    <!-- Footer -->
    <div class="absolute bottom-0 left-0 right-0 p-4 border-t border-gray-800">
      <div class="flex items-center text-xs text-gray-500">
        <span>North Cloud Platform</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { CloudIcon } from '@heroicons/vue/24/outline'
import { getNavigationSections } from '@/config/navigation'
import NavigationSection from './NavigationSection.vue'

// Get navigation sections sorted by order (pipeline-based)
const sections = getNavigationSections()
</script>
