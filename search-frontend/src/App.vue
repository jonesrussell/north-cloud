<template>
  <div class="min-h-screen bg-gray-50 flex flex-col">
    <!-- Header -->
    <header class="bg-white shadow-sm">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div class="flex justify-between items-center h-16">
          <!-- Logo -->
          <router-link to="/" class="flex items-center space-x-3">
            <div class="text-2xl font-bold text-blue-600">
              North Cloud
            </div>
          </router-link>

          <!-- Navigation -->
          <nav class="flex items-center space-x-6">
            <router-link
              to="/advanced"
              class="text-gray-600 hover:text-gray-900 text-sm font-medium transition-colors"
            >
              Advanced Search
            </router-link>
            <a
              href="/dashboard"
              class="text-gray-600 hover:text-gray-900 text-sm font-medium transition-colors"
            >
              Admin Dashboard
            </a>
          </nav>
        </div>
      </div>
    </header>

    <!-- Main Content -->
    <main class="flex-1">
      <router-view />
    </main>

    <!-- Footer -->
    <footer class="bg-white border-t border-gray-200 mt-auto">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-6">
        <div class="text-center text-sm text-gray-500">
          &copy; {{ currentYear }} North Cloud. All rights reserved.
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const currentYear = computed(() => new Date().getFullYear())
</script>
