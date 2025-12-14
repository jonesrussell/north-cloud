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
    loading.value = true
    error.value = null
    health.value = await crawlerApi.getHealth()
  } catch (err) {
    error.value = 'Unable to fetch health status'
    console.error('Error loading health:', err)
  } finally {
    loading.value = false
  }
}

const loadStats = async () => {
  try {
    const data = await crawlerApi.getStats()
    stats.value = data
  } catch (err) {
    console.error('Error loading stats:', err)
    // Keep default values if API not available
  }
}

onMounted(() => {
  loadHealth()
  loadStats()
})
</script>
