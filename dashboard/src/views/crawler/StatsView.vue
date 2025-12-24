<template>
  <div>
    <PageHeader
      title="Crawler Statistics"
      subtitle="Performance metrics and analytics"
    />

    <!-- Loading State -->
    <LoadingSpinner
      v-if="loading"
      size="lg"
      text="Loading statistics..."
      :full-page="true"
    />

    <!-- Error State -->
    <ErrorAlert
      v-else-if="error"
      :message="error"
      class="mb-6"
    />

    <!-- Stats Content -->
    <div v-else>
      <!-- Overview Stats -->
      <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
        <StatCard
          label="Total Articles"
          :value="stats.totalArticles"
          :icon="NewspaperIcon"
          color="blue"
        >
          <template #footer>
            <span class="text-xs text-gray-500">All time</span>
          </template>
        </StatCard>

        <StatCard
          label="Success Rate"
          :value="stats.successRate"
          :icon="CheckCircleIcon"
          color="green"
          format="percent"
        >
          <template #footer>
            <span class="text-xs text-gray-500">Last 24 hours</span>
          </template>
        </StatCard>

        <StatCard
          label="Avg Response Time"
          :value="`${stats.avgResponseTime}ms`"
          :icon="ClockIcon"
          color="gray"
          format="text"
        >
          <template #footer>
            <span class="text-xs text-gray-500">Last hour</span>
          </template>
        </StatCard>
      </div>

      <!-- Performance Metrics Grid -->
      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
        <!-- Articles by Status -->
        <div class="bg-white shadow rounded-lg p-6">
          <h2 class="text-lg font-medium text-gray-900 mb-4">
            Articles by Status
          </h2>
          <div class="space-y-4">
            <div class="flex items-center justify-between">
              <div class="flex items-center">
                <span class="h-3 w-3 rounded-full bg-green-500 mr-3" />
                <span class="text-sm text-gray-600">Crawled</span>
              </div>
              <span class="text-sm font-semibold text-green-600">{{ stats.crawled }}</span>
            </div>
            <div class="flex items-center justify-between">
              <div class="flex items-center">
                <span class="h-3 w-3 rounded-full bg-red-500 mr-3" />
                <span class="text-sm text-gray-600">Failed</span>
              </div>
              <span class="text-sm font-semibold text-red-600">{{ stats.failed }}</span>
            </div>
            <div class="flex items-center justify-between">
              <div class="flex items-center">
                <span class="h-3 w-3 rounded-full bg-yellow-500 mr-3" />
                <span class="text-sm text-gray-600">Pending</span>
              </div>
              <span class="text-sm font-semibold text-yellow-600">{{ stats.pending }}</span>
            </div>
          </div>

          <!-- Progress bar visualization -->
          <div class="mt-6">
            <div class="h-2 w-full bg-gray-200 rounded-full overflow-hidden flex">
              <div
                class="h-full bg-green-500"
                :style="{ width: `${getPercentage('crawled')}%` }"
              />
              <div
                class="h-full bg-red-500"
                :style="{ width: `${getPercentage('failed')}%` }"
              />
              <div
                class="h-full bg-yellow-500"
                :style="{ width: `${getPercentage('pending')}%` }"
              />
            </div>
          </div>
        </div>

        <!-- Sources Overview -->
        <div class="bg-white shadow rounded-lg p-6">
          <h2 class="text-lg font-medium text-gray-900 mb-4">
            Sources Overview
          </h2>
          <div class="space-y-4">
            <div class="flex items-center justify-between">
              <div class="flex items-center">
                <DocumentTextIcon class="h-5 w-5 text-gray-400 mr-3" />
                <span class="text-sm text-gray-600">Active Sources</span>
              </div>
              <span class="text-sm font-semibold text-gray-900">{{ stats.activeSources }}</span>
            </div>
            <div class="flex items-center justify-between">
              <div class="flex items-center">
                <DocumentTextIcon class="h-5 w-5 text-gray-400 mr-3" />
                <span class="text-sm text-gray-600">Total Sources</span>
              </div>
              <span class="text-sm font-semibold text-gray-900">{{ stats.totalSources }}</span>
            </div>
          </div>

          <div class="mt-6 pt-4 border-t border-gray-200">
            <router-link
              to="/sources"
              class="text-sm font-medium text-blue-600 hover:text-blue-500"
            >
              Manage sources &rarr;
            </router-link>
          </div>
        </div>
      </div>

      <!-- Chart Placeholder -->
      <div class="bg-white shadow rounded-lg p-6">
        <h2 class="text-lg font-medium text-gray-900 mb-4">
          Activity Over Time
        </h2>
        <div class="h-64 flex items-center justify-center border-2 border-dashed border-gray-300 rounded-lg bg-gray-50">
          <div class="text-center">
            <ChartBarIcon class="mx-auto h-12 w-12 text-gray-400" />
            <p class="mt-2 text-sm font-medium text-gray-500">
              Chart Visualization
            </p>
            <p class="mt-1 text-xs text-gray-400">
              Can be implemented with Chart.js or similar
            </p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import {
  NewspaperIcon,
  CheckCircleIcon,
  ClockIcon,
  DocumentTextIcon,
  ChartBarIcon,
} from '@heroicons/vue/24/outline'
import { crawlerApi, sourcesApi } from '../../api/client'
import { PageHeader, LoadingSpinner, ErrorAlert, StatCard } from '../../components/common'

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
  totalSources: 0,
})

const getPercentage = (type) => {
  const total = stats.value.crawled + stats.value.failed + stats.value.pending
  if (total === 0) return 0
  return Math.round((stats.value[type] / total) * 100)
}

const loadStats = async () => {
  try {
    loading.value = true
    error.value = null

    // Load stats and sources in parallel
    const [statsRes, sourcesRes] = await Promise.allSettled([
      crawlerApi.stats.get(),
      sourcesApi.list(),
    ])

    // Process stats
    if (statsRes.status === 'fulfilled' && statsRes.value.data) {
      const data = statsRes.value.data
      stats.value = {
        ...stats.value,
        totalArticles: data.total_articles || data.totalArticles || 0,
        successRate: data.success_rate || data.successRate || 0,
        avgResponseTime: data.avg_response_time || data.avgResponseTime || 0,
        crawled: data.crawled || 0,
        failed: data.failed || 0,
        pending: data.pending || 0,
      }
    }

    // Process sources
    if (sourcesRes.status === 'fulfilled') {
      const sources = sourcesRes.value.data?.sources || sourcesRes.value.data || []
      stats.value.totalSources = sources.length
      stats.value.activeSources = sources.filter((s) => s.enabled).length
    }
  } catch (err) {
    error.value = 'Unable to load statistics. Backend API may not be available yet.'
    console.error('[StatsView] Error loading stats:', err)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadStats()
})
</script>
