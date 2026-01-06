<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useSidebar } from '@/composables/useSidebar'
import { useTheme } from '@/composables/useTheme'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

const { isCollapsed, isMobile, sidebarWidth } = useSidebar()

// Initialize theme on mount
useTheme()

const mainStyle = computed(() => ({
  marginLeft: isMobile.value ? '0' : sidebarWidth.value,
  transition: 'margin-left 0.2s ease-in-out',
}))

onMounted(() => {
  // Apply stored theme on mount
  const stored = localStorage.getItem('north-cloud-theme')
  if (stored === 'dark' || (!stored && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    document.documentElement.classList.add('dark')
  }
})
</script>

<template>
  <div class="min-h-screen bg-background">
    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main content -->
    <div
      :style="mainStyle"
      class="flex flex-col min-h-screen"
    >
      <!-- Header -->
      <AppHeader />

      <!-- Page content -->
      <main class="flex-1 p-6">
        <slot />
      </main>
    </div>
  </div>
</template>
