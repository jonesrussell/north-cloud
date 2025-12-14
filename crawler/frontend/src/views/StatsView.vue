<template>
  <div>
    <div class="mb-6">
      <h1 class="text-2xl font-bold text-gray-900">Statistics</h1>
      <p class="mt-1 text-sm text-gray-600">Crawler performance and metrics</p>
    </div>

    <div v-if="loading" class="text-center py-8 text-gray-500">
      Loading statistics...
    </div>
    <div v-else-if="error" class="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
      {{ error }}
    </div>
    <div v-else>
      <!-- Overview Stats -->
      <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
        <div class="bg-white shadow rounded-lg p-6">
          <div class="text-gray-500 text-sm font-medium">Total Articles</div>
          <div class="text-3xl font-bold text-gray-900 mt-2">
            {{ stats.totalArticles || 0 }}
          </div>
          <div class="text-sm text-gray-500 mt-1">All time</div>
        </div>
        <div class="bg-white shadow rounded-lg p-6">
          <div class="text-gray-500 text-sm font-medium">Success Rate</div>
          <div class="text-3xl font-bold text-gray-900 mt-2">
            {{ stats.successRate || 0 }}%
          </div>
          <div class="text-sm text-gray-500 mt-1">Last 24 hours</div>
        </div>
        <div class="bg-white shadow rounded-lg p-6">
          <div class="text-gray-500 text-sm font-medium">Avg Response Time</div>
          <div class="text-3xl font-bold text-gray-900 mt-2">
            {{ stats.avgResponseTime || 0 }}ms
          </div>
          <div class="text-sm text-gray-500 mt-1">Last hour</div>
        </div>
      </div>

      <!-- Performance Metrics -->
      <div class="bg-white shadow rounded-lg p-6 mb-6">
        <h2 class="text-lg font-medium text-gray-900 mb-4">Performance Metrics</h2>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div>
            <h3 class="text-sm font-medium text-gray-700 mb-2">Articles by Status</h3>
            <div class="space-y-2">
              <div class="flex justify-between items-center">
                <span class="text-sm text-gray-600">Crawled</span>
                <span class="text-sm font-medium text-green-600">
                  {{ stats.crawled || 0 }}
                </span>
              </div>
              <div class="flex justify-between items-center">
                <span class="text-sm text-gray-600">Failed</span>
                <span class="text-sm font-medium text-red-600">
                  {{ stats.failed || 0 }}
                </span>
              </div>
              <div class="flex justify-between items-center">
                <span class="text-sm text-gray-600">Pending</span>
                <span class="text-sm font-medium text-yellow-600">
                  {{ stats.pending || 0 }}
                </span>
              </div>
            </div>
          </div>
          <div>
            <h3 class="text-sm font-medium text-gray-700 mb-2">Sources</h3>
            <div class="space-y-2">
              <div class="flex justify-between items-center">
                <span class="text-sm text-gray-600">Active Sources</span>
                <span class="text-sm font-medium text-gray-900">
                  {{ stats.activeSources || 0 }}
                </span>
              </div>
              <div class="flex justify-between items-center">
                <span class="text-sm text-gray-600">Total Sources</span>
                <span class="text-sm font-medium text-gray-900">
                  {{ stats.totalSources || 0 }}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Chart Placeholder -->
      <div class="bg-white shadow rounded-lg p-6">
        <h2 class="text-lg font-medium text-gray-900 mb-4">Activity Over Time</h2>
        <div class="h-64 flex items-center justify-center border-2 border-dashed border-gray-300 rounded-lg">
          <div class="text-center text-gray-500">
            <div class="text-lg font-medium">Chart Visualization</div>
            <div class="text-sm mt-1">Can be implemented with Chart.js or similar</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { crawlerApi } from '../api/client'

const loading = ref(true)
const error = ref(null)
const stats = ref({
  totalArticles: 0,
  successRate: 0,
  avgResponseTime: 0,
  crawled: 0,
  failed: 0,
  pending: 0,
  activeSources: 0,
  totalSources: 0
})

const loadStats = async () => {
  try {
    loading.value = true
    error.value = null
    const data = await crawlerApi.getStats()
    stats.value = { ...stats.value, ...data }
  } catch (err) {
    error.value = 'Unable to load statistics. Backend API may not be available yet.'
    console.error('Error loading stats:', err)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadStats()
})
</script>
