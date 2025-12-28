<template>
  <div>
    <PageHeader
      title="Publisher Statistics"
      subtitle="Article publishing metrics and analytics"
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
          label="Total Posted"
          :value="stats.totalPosted"
          :icon="CheckCircleIcon"
          color="green"
        >
          <template #footer>
            <span class="text-xs text-gray-500">All time</span>
          </template>
        </StatCard>

        <StatCard
          label="Total Skipped"
          :value="stats.totalSkipped"
          :icon="XCircleIcon"
          color="yellow"
        >
          <template #footer>
            <span class="text-xs text-gray-500">Duplicates or filtered</span>
          </template>
        </StatCard>

        <StatCard
          label="Total Errors"
          :value="stats.totalErrors"
          :icon="ExclamationTriangleIcon"
          color="red"
        >
          <template #footer>
            <span class="text-xs text-gray-500">Publishing failures</span>
          </template>
        </StatCard>
      </div>

      <!-- Last Sync -->
      <div class="bg-white shadow rounded-lg p-6 mb-8">
        <div class="flex items-center justify-between">
          <div>
            <h2 class="text-lg font-medium text-gray-900 mb-1">
              Last Sync
            </h2>
            <p class="text-sm text-gray-500">
              {{ lastSyncText }}
            </p>
          </div>
          <ClockIcon class="h-8 w-8 text-gray-400" />
        </div>
      </div>

      <!-- Cities Breakdown -->
      <div class="bg-white shadow rounded-lg p-6 mb-8">
        <h2 class="text-lg font-medium text-gray-900 mb-4">
          Statistics by City
        </h2>
        <div
          v-if="stats.cities && stats.cities.length > 0"
          class="space-y-4"
        >
          <div
            v-for="city in stats.cities"
            :key="city.name"
            class="border-b border-gray-200 pb-4 last:border-b-0 last:pb-0"
          >
            <div class="flex items-center justify-between mb-2">
              <h3 class="text-sm font-semibold text-gray-900">
                {{ city.name }}
              </h3>
            </div>
            <div class="grid grid-cols-3 gap-4 mt-2">
              <div>
                <span class="text-xs text-gray-500">Posted</span>
                <p class="text-lg font-semibold text-green-600">
                  {{ city.posted }}
                </p>
              </div>
              <div>
                <span class="text-xs text-gray-500">Skipped</span>
                <p class="text-lg font-semibold text-yellow-600">
                  {{ city.skipped }}
                </p>
              </div>
              <div>
                <span class="text-xs text-gray-500">Errors</span>
                <p class="text-lg font-semibold text-red-600">
                  {{ city.errors }}
                </p>
              </div>
            </div>
          </div>
        </div>
        <div
          v-else
          class="text-center py-8 text-gray-500"
        >
          <p>No city statistics available</p>
        </div>
      </div>

      <!-- Success Rate Visualization -->
      <div class="bg-white shadow rounded-lg p-6">
        <h2 class="text-lg font-medium text-gray-900 mb-4">
          Success Rate
        </h2>
        <div class="space-y-4">
          <div class="flex items-center justify-between">
            <div class="flex items-center">
              <span class="h-3 w-3 rounded-full bg-green-500 mr-3" />
              <span class="text-sm text-gray-600">Posted Successfully</span>
            </div>
            <span class="text-sm font-semibold text-green-600">{{ stats.totalPosted }}</span>
          </div>
          <div class="flex items-center justify-between">
            <div class="flex items-center">
              <span class="h-3 w-3 rounded-full bg-yellow-500 mr-3" />
              <span class="text-sm text-gray-600">Skipped</span>
            </div>
            <span class="text-sm font-semibold text-yellow-600">{{ stats.totalSkipped }}</span>
          </div>
          <div class="flex items-center justify-between">
            <div class="flex items-center">
              <span class="h-3 w-3 rounded-full bg-red-500 mr-3" />
              <span class="text-sm text-gray-600">Errors</span>
            </div>
            <span class="text-sm font-semibold text-red-600">{{ stats.totalErrors }}</span>
          </div>

          <!-- Progress bar visualization -->
          <div class="mt-6">
            <div class="h-2 w-full bg-gray-200 rounded-full overflow-hidden flex">
              <div
                class="h-full bg-green-500"
                :style="{ width: `${getPercentage('posted')}%` }"
              />
              <div
                class="h-full bg-yellow-500"
                :style="{ width: `${getPercentage('skipped')}%` }"
              />
              <div
                class="h-full bg-red-500"
                :style="{ width: `${getPercentage('errors')}%` }"
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  CheckCircleIcon,
  XCircleIcon,
  ExclamationTriangleIcon,
  ClockIcon,
} from '@heroicons/vue/24/outline'
import { publisherApi } from '../../api/client'
import { PageHeader, LoadingSpinner, ErrorAlert, StatCard } from '../../components/common'

const loading = ref(true)
const error = ref(null)
const stats = ref({
  totalPosted: 0,
  totalSkipped: 0,
  totalErrors: 0,
  cities: [],
  lastSync: null,
})

const lastSyncText = computed(() => {
  if (!stats.value.lastSync) return 'Never'
  try {
    const date = new Date(stats.value.lastSync)
    return date.toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  } catch {
    return 'Invalid date'
  }
})

const getPercentage = (type) => {
  const total = stats.value.totalPosted + stats.value.totalSkipped + stats.value.totalErrors
  if (total === 0) return 0
  const value = stats.value[`total${type.charAt(0).toUpperCase() + type.slice(1)}`] || 0
  return Math.round((value / total) * 100)
}

const loadStats = async () => {
  try {
    loading.value = true
    error.value = null

    const statsRes = await publisherApi.stats.get()

    if (statsRes.data) {
      stats.value = {
        totalPosted: statsRes.data.total_posted || 0,
        totalSkipped: statsRes.data.total_skipped || 0,
        totalErrors: statsRes.data.total_errors || 0,
        cities: statsRes.data.cities || [],
        lastSync: statsRes.data.last_sync || null,
      }
    }
  } catch (err) {
    error.value = 'Unable to load statistics. Publisher API may not be available yet.'
    console.error('[PublisherStatsView] Error loading stats:', err)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadStats()
})
</script>
