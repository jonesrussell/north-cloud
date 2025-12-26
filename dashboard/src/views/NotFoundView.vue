<template>
  <div class="flex flex-col items-center justify-center min-h-96">
    <div class="text-center">
      <!-- 404 Icon/Number -->
      <div class="mb-6">
        <h1 class="text-9xl font-bold text-gray-300">404</h1>
      </div>

      <!-- Error Message -->
      <h2 class="text-3xl font-semibold text-gray-900 mb-4">
        Page Not Found
      </h2>
      <p class="text-lg text-gray-600 mb-8 max-w-md">
        The page you're looking for doesn't exist or has been moved.
      </p>

      <!-- Action Buttons -->
      <div class="flex items-center justify-center gap-4">
        <router-link
          to="/dashboard"
          class="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 transition-colors"
        >
          <HomeIcon class="mr-2 h-5 w-5" />
          Go to Dashboard
        </router-link>
        <button
          @click="$router.go(-1)"
          class="inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 transition-colors"
        >
          <ArrowLeftIcon class="mr-2 h-5 w-5" />
          Go Back
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { HomeIcon, ArrowLeftIcon } from '@heroicons/vue/24/outline'
</script>

