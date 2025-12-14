<template>
  <div>
    <div class="mb-6">
      <h1 class="text-2xl font-bold text-gray-900">Crawler Dashboard</h1>
      <p class="mt-1 text-sm text-gray-600">Monitor crawler status and activity</p>
    </div>

    <!-- Health Status Card -->
    <div class="bg-white shadow rounded-lg p-6 mb-6">
      <h2 class="text-lg font-medium text-gray-900 mb-4">System Health</h2>
      <div v-if="loading" class="text-gray-500">Loading...</div>
      <div v-else-if="error" class="text-red-600">{{ error }}</div>
      <div v-else class="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div class="bg-green-50 border border-green-200 rounded-lg p-4">
          <div class="text-green-800 text-sm font-medium">Status</div>
          <div class="text-2xl font-bold text-green-900 mt-1">
            {{ health?.status || 'Healthy' }}
          </div>
        </div>
        <div class="bg-blue-50 border border-blue-200 rounded-lg p-4">
          <div class="text-blue-800 text-sm font-medium">Service</div>
          <div class="text-2xl font-bold text-blue-900 mt-1">Crawler</div>
        </div>
        <div class="bg-purple-50 border border-purple-200 rounded-lg p-4">
          <div class="text-purple-800 text-sm font-medium">Version</div>
          <div class="text-2xl font-bold text-purple-900 mt-1">
            {{ health?.version || '1.0.0' }}
          </div>
        </div>
      </div>
    </div>

    <!-- Quick Stats -->
    <div class="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
      <div class="bg-white shadow rounded-lg p-6">
        <div class="text-gray-500 text-sm font-medium">Active Jobs</div>
        <div class="text-3xl font-bold text-gray-900 mt-2">
          {{ stats.activeJobs || 0 }}
        </div>
      </div>
      <div class="bg-white shadow rounded-lg p-6">
        <div class="text-gray-500 text-sm font-medium">Articles Crawled</div>
        <div class="text-3xl font-bold text-gray-900 mt-2">
          {{ stats.articlesCrawled || 0 }}
        </div>
      </div>
      <div class="bg-white shadow rounded-lg p-6">
        <div class="text-gray-500 text-sm font-medium">Success Rate</div>
        <div class="text-3xl font-bold text-gray-900 mt-2">
          {{ stats.successRate || '0' }}%
        </div>
      </div>
      <div class="bg-white shadow rounded-lg p-6">
        <div class="text-gray-500 text-sm font-medium">Errors</div>
        <div class="text-3xl font-bold text-gray-900 mt-2">
          {{ stats.errors || 0 }}
        </div>
      </div>
    </div>

    <!-- Recent Activity -->
    <div class="bg-white shadow rounded-lg p-6">
      <h2 class="text-lg font-medium text-gray-900 mb-4">Recent Activity</h2>
      <div class="text-gray-500 text-sm">
        Activity logs will appear here when crawler backend is integrated.
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { crawlerApi } from '../api/client'

const loading = ref(true)
const error = ref(null)
const health = ref(null)
const stats = ref({
  activeJobs: 0,
  articlesCrawled: 0,
  successRate: 0,
  errors: 0
})

const loadHealth = async () => {
  try {
    console.log('[Dashboard] Loading health...')
    loading.value = true
    error.value = null
    health.value = await crawlerApi.getHealth()
    console.log('[Dashboard] Health loaded successfully:', health.value)
  } catch (err) {
    const errorDetails = {
      message: err.message,
      code: err.code,
      response: err.response?.status,
      data: err.response?.data,
    }
    console.error('[Dashboard] Error loading health:', err, errorDetails)
    error.value = `Unable to fetch health status: ${err.message} (${err.code || 'unknown'})`

    // Show detailed error in console for debugging
    if (err.code === 'ERR_NETWORK') {
      console.error('[Dashboard] Network error - backend may not be running on port 8060')
      console.error('[Dashboard] Check: 1) Is crawler container running? 2) Is it listening on :8060? 3) Is port mapped correctly?')
    }
  } finally {
    loading.value = false
  }
}

const loadStats = async () => {
  try {
    console.log('[Dashboard] Loading stats...')
    const data = await crawlerApi.getStats()
    stats.value = data
    console.log('[Dashboard] Stats loaded successfully:', stats.value)
  } catch (err) {
    console.error('[Dashboard] Error loading stats:', err)
    console.error('[Dashboard] Stats endpoint may not be implemented yet - using default values')
    // Keep default values if API not available
  }
}

onMounted(() => {
  loadHealth()
  loadStats()
})
</script>
