<template>
  <div class="min-h-screen bg-gray-50 flex flex-col">
    <!-- Header -->
    <header class="bg-white shadow-sm">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div class="flex justify-between items-center h-16">
          <router-link
            v-if="showHeaderLogo"
            to="/"
            class="flex items-center space-x-3 text-2xl font-bold text-blue-600"
          >
            North Cloud
          </router-link>
          <span
            v-else
            class="h-4"
          />
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

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const currentYear = computed(() => new Date().getFullYear())

const route = useRoute()
const showHeaderLogo = computed(() => route.name !== 'home')
</script>
