<template>
  <div class="min-h-screen bg-gray-100">
    <!-- Command Palette (Cmd+K) -->
    <CommandPalette />

    <!-- Show sidebar only when authenticated (not on login page) -->
    <template v-if="isAuthenticated">
      <!-- Sidebar -->
      <AppSidebar />

      <!-- Main Content -->
      <div class="pl-64">
        <!-- Top bar -->
        <header class="sticky top-0 z-40 bg-white border-b border-gray-200">
          <div class="flex h-16 items-center justify-between px-6">
            <h1 class="text-lg font-semibold text-gray-900">
              {{ pageTitle }}
            </h1>
            <div class="flex items-center space-x-4">
              <!-- Recent Pages -->
              <RecentPages />

              <!-- Health indicator -->
              <div class="flex items-center text-sm">
                <span
                  class="h-2 w-2 rounded-full mr-2"
                  :class="healthStatus === 'healthy' ? 'bg-green-500' : 'bg-red-500'"
                />
                <span class="text-gray-600">
                  {{ healthStatus === 'healthy' ? 'System Healthy' : 'System Issues' }}
                </span>
              </div>
              <!-- Logout button -->
              <button
                class="rounded-md bg-gray-200 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500"
                @click="handleLogout"
              >
                Logout
              </button>
            </div>
          </div>
        </header>

        <!-- Breadcrumbs -->
        <div class="bg-white px-6 py-3 border-b border-gray-200">
          <BreadcrumbsNav />
        </div>

        <!-- Page content -->
        <main class="p-6">
          <router-view />
        </main>
      </div>
    </template>
    
    <!-- Show full-width router-view for login page -->
    <template v-else>
      <router-view />
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { AppSidebar, BreadcrumbsNav, CommandPalette, RecentPages } from './components/navigation'
import { crawlerApi, publisherApi, classifierApi } from './api/client'
import { useAuth } from './composables/useAuth'

const route = useRoute()
const { isAuthenticated, logout } = useAuth()
const healthStatus = ref('healthy')

const pageTitle = computed(() => {
  return route.meta?.title || 'Dashboard'
})

// Handle logout
const handleLogout = () => {
  logout()
}

// Check system health on mount (only when authenticated)
onMounted(async () => {
  if (!isAuthenticated.value) {
    return
  }

  try {
    // Check all services health
    const [crawlerHealth, publisherHealth, classifierHealth] = await Promise.allSettled([
      crawlerApi.getHealth(),
      publisherApi.getHealth(),
      classifierApi.getHealth(),
    ])
    // Consider healthy if at least one service is healthy
    if (
      crawlerHealth.status === 'fulfilled' ||
      publisherHealth.status === 'fulfilled' ||
      classifierHealth.status === 'fulfilled'
    ) {
      healthStatus.value = 'healthy'
    } else {
      healthStatus.value = 'unhealthy'
    }
  } catch {
    healthStatus.value = 'unhealthy'
  }
})
</script>
