<template>
  <div class="min-h-screen bg-gray-100">
    <!-- Sidebar -->
    <div class="fixed inset-y-0 left-0 z-50 w-64 bg-gray-900">
      <!-- Logo -->
      <div class="flex h-16 items-center justify-center border-b border-gray-800">
        <CloudIcon class="h-8 w-8 text-blue-500" />
        <span class="ml-2 text-xl font-bold text-white">North Cloud</span>
      </div>

      <!-- Navigation -->
      <nav class="mt-6 px-3">
        <!-- Dashboard -->
        <router-link
          to="/dashboard"
          class="group flex items-center px-3 py-2 text-sm font-medium rounded-md transition-colors"
          :class="[
            isActive('/dashboard')
              ? 'bg-gray-800 text-white'
              : 'text-gray-300 hover:bg-gray-800 hover:text-white'
          ]"
        >
          <HomeIcon class="mr-3 h-5 w-5 flex-shrink-0" />
          Dashboard
        </router-link>

        <!-- Crawler Section -->
        <div class="mt-6">
          <h3 class="px-3 text-xs font-semibold text-gray-400 uppercase tracking-wider">
            Crawler
          </h3>
          <div class="mt-2 space-y-1">
            <router-link
              to="/crawler/jobs"
              class="group flex items-center px-3 py-2 text-sm font-medium rounded-md transition-colors"
              :class="[
                isActive('/crawler/jobs')
                  ? 'bg-gray-800 text-white'
                  : 'text-gray-300 hover:bg-gray-800 hover:text-white'
              ]"
            >
              <BriefcaseIcon class="mr-3 h-5 w-5 flex-shrink-0" />
              Jobs
            </router-link>
            <router-link
              to="/crawler/stats"
              class="group flex items-center px-3 py-2 text-sm font-medium rounded-md transition-colors"
              :class="[
                isActive('/crawler/stats')
                  ? 'bg-gray-800 text-white'
                  : 'text-gray-300 hover:bg-gray-800 hover:text-white'
              ]"
            >
              <ChartBarIcon class="mr-3 h-5 w-5 flex-shrink-0" />
              Statistics
            </router-link>
          </div>
        </div>

        <!-- Publisher Section -->
        <div class="mt-6">
          <h3 class="px-3 text-xs font-semibold text-gray-400 uppercase tracking-wider">
            Publisher
          </h3>
          <div class="mt-2 space-y-1">
            <router-link
              to="/publisher/stats"
              class="group flex items-center px-3 py-2 text-sm font-medium rounded-md transition-colors"
              :class="[
                isActive('/publisher/stats')
                  ? 'bg-gray-800 text-white'
                  : 'text-gray-300 hover:bg-gray-800 hover:text-white'
              ]"
            >
              <ChartBarIcon class="mr-3 h-5 w-5 flex-shrink-0" />
              Statistics
            </router-link>
            <router-link
              to="/publisher/articles"
              class="group flex items-center px-3 py-2 text-sm font-medium rounded-md transition-colors"
              :class="[
                isActive('/publisher/articles')
                  ? 'bg-gray-800 text-white'
                  : 'text-gray-300 hover:bg-gray-800 hover:text-white'
              ]"
            >
              <NewspaperIcon class="mr-3 h-5 w-5 flex-shrink-0" />
              Recent Articles
            </router-link>
          </div>
        </div>

        <!-- Sources Section -->
        <div class="mt-6">
          <h3 class="px-3 text-xs font-semibold text-gray-400 uppercase tracking-wider">
            Sources
          </h3>
          <div class="mt-2 space-y-1">
            <router-link
              to="/sources"
              class="group flex items-center px-3 py-2 text-sm font-medium rounded-md transition-colors"
              :class="[
                isActiveExact('/sources')
                  ? 'bg-gray-800 text-white'
                  : 'text-gray-300 hover:bg-gray-800 hover:text-white'
              ]"
            >
              <DocumentTextIcon class="mr-3 h-5 w-5 flex-shrink-0" />
              Manage Sources
            </router-link>
            <router-link
              to="/sources/cities"
              class="group flex items-center px-3 py-2 text-sm font-medium rounded-md transition-colors"
              :class="[
                isActive('/sources/cities')
                  ? 'bg-gray-800 text-white'
                  : 'text-gray-300 hover:bg-gray-800 hover:text-white'
              ]"
            >
              <MapPinIcon class="mr-3 h-5 w-5 flex-shrink-0" />
              Cities
            </router-link>
          </div>
        </div>
      </nav>

      <!-- Footer -->
      <div class="absolute bottom-0 left-0 right-0 p-4 border-t border-gray-800">
        <div class="flex items-center text-xs text-gray-500">
          <span>North Cloud Platform</span>
        </div>
      </div>
    </div>

    <!-- Main Content -->
    <div class="pl-64">
      <!-- Top bar -->
      <header class="sticky top-0 z-40 bg-white border-b border-gray-200">
        <div class="flex h-16 items-center justify-between px-6">
          <h1 class="text-lg font-semibold text-gray-900">{{ pageTitle }}</h1>
          <div class="flex items-center space-x-4">
            <!-- Health indicator -->
            <div class="flex items-center text-sm">
              <span
                class="h-2 w-2 rounded-full mr-2"
                :class="healthStatus === 'healthy' ? 'bg-green-500' : 'bg-red-500'"
              ></span>
              <span class="text-gray-600">
                {{ healthStatus === 'healthy' ? 'System Healthy' : 'System Issues' }}
              </span>
            </div>
          </div>
        </div>
      </header>

      <!-- Page content -->
      <main class="p-6">
        <router-view />
      </main>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import {
  CloudIcon,
  HomeIcon,
  BriefcaseIcon,
  ChartBarIcon,
  DocumentTextIcon,
  MapPinIcon,
  NewspaperIcon,
} from '@heroicons/vue/24/outline'
import { crawlerApi, publisherApi } from './api/client'

const route = useRoute()
const healthStatus = ref('healthy')

const pageTitle = computed(() => {
  return route.meta?.title || 'Dashboard'
})

const isActive = (path) => {
  return route.path.startsWith(path)
}

const isActiveExact = (path) => {
  return route.path === path
}

// Check system health on mount
onMounted(async () => {
  try {
    // Check both crawler and publisher health
    const [crawlerHealth, publisherHealth] = await Promise.allSettled([
      crawlerApi.getHealth(),
      publisherApi.getHealth(),
    ])
    // Consider healthy if at least one service is healthy
    if (crawlerHealth.status === 'fulfilled' || publisherHealth.status === 'fulfilled') {
      healthStatus.value = 'healthy'
    } else {
      healthStatus.value = 'unhealthy'
    }
  } catch {
    healthStatus.value = 'unhealthy'
  }
})
</script>

<style scoped>
/* Smooth transitions for navigation */
nav a {
  transition: background-color 0.15s ease-in-out, color 0.15s ease-in-out;
}
</style>
