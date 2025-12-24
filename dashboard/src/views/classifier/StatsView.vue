<template>
  <div>
    <PageHeader
      title="Classifier Statistics"
      subtitle="Content classification metrics and analytics"
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
      <div class="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
        <StatCard
          label="Total Classified"
          :value="stats.totalClassified"
          :icon="CheckCircleIcon"
          color="green"
        >
          <template #footer>
            <span class="text-xs text-gray-500">All time</span>
          </template>
        </StatCard>

        <StatCard
          label="Avg Quality Score"
          :value="stats.avgQualityScore"
          :icon="StarIcon"
          color="blue"
          format="number"
        >
          <template #footer>
            <span class="text-xs text-gray-500">0-100 scale</span>
          </template>
        </StatCard>

        <StatCard
          label="Crime Related"
          :value="stats.crimeRelated"
          :icon="ExclamationTriangleIcon"
          color="red"
        >
          <template #footer>
            <span class="text-xs text-gray-500">{{ getCrimePercentage }}% of total</span>
          </template>
        </StatCard>

        <StatCard
          label="Avg Processing Time"
          :value="stats.avgProcessingTime"
          :icon="ClockIcon"
          color="purple"
          format="ms"
        >
          <template #footer>
            <span class="text-xs text-gray-500">Per item</span>
          </template>
        </StatCard>
      </div>

      <!-- Topic Distribution -->
      <div class="bg-white shadow rounded-lg p-6 mb-8">
        <h2 class="text-lg font-medium text-gray-900 mb-4">
          Topic Distribution
        </h2>
        <div
          v-if="topicStats && topicStats.length > 0"
          class="space-y-3"
        >
          <div
            v-for="topic in topicStats"
            :key="topic.topic"
            class="flex items-center justify-between"
          >
            <div class="flex items-center flex-1">
              <span class="text-sm font-medium text-gray-900 capitalize">{{ topic.topic }}</span>
            </div>
            <div class="flex items-center space-x-4">
              <span class="text-sm text-gray-600">{{ topic.count }} items</span>
              <div class="w-32 bg-gray-200 rounded-full h-2">
                <div
                  class="bg-blue-500 h-2 rounded-full"
                  :style="{ width: `${getTopicPercentage(topic.count)}%` }"
                />
              </div>
              <span class="text-sm font-semibold text-gray-900 w-12 text-right">
                {{ getTopicPercentage(topic.count) }}%
              </span>
            </div>
          </div>
        </div>
        <div
          v-else
          class="text-center py-8 text-gray-500"
        >
          <p>No topic statistics available</p>
        </div>
      </div>

      <!-- Source Reputation Distribution -->
      <div class="bg-white shadow rounded-lg p-6 mb-8">
        <h2 class="text-lg font-medium text-gray-900 mb-4">
          Source Reputation Distribution
        </h2>
        <div
          v-if="sourceStats && sourceStats.length > 0"
          class="space-y-4"
        >
          <div
            v-for="source in sourceStats"
            :key="source.name"
            class="border-b border-gray-200 pb-4 last:border-b-0 last:pb-0"
          >
            <div class="flex items-center justify-between mb-2">
              <h3 class="text-sm font-semibold text-gray-900">
                {{ source.name }}
              </h3>
              <span
                class="px-2 py-1 text-xs font-semibold rounded"
                :class="getReputationClass(source.reputation)"
              >
                {{ source.reputation }}/100
              </span>
            </div>
            <div class="flex items-center space-x-2">
              <div class="flex-1 bg-gray-200 rounded-full h-2">
                <div
                  class="h-2 rounded-full"
                  :class="getReputationColorClass(source.reputation)"
                  :style="{ width: `${source.reputation}%` }"
                />
              </div>
              <span class="text-xs text-gray-500">{{ source.category || 'Unknown' }}</span>
            </div>
          </div>
        </div>
        <div
          v-else
          class="text-center py-8 text-gray-500"
        >
          <p>No source reputation statistics available</p>
        </div>
      </div>

      <!-- Content Type Breakdown -->
      <div class="bg-white shadow rounded-lg p-6">
        <h2 class="text-lg font-medium text-gray-900 mb-4">
          Content Type Breakdown
        </h2>
        <div
          v-if="contentTypeStats && Object.keys(contentTypeStats).length > 0"
          class="space-y-3"
        >
          <div
            v-for="(count, type) in contentTypeStats"
            :key="type"
            class="flex items-center justify-between"
          >
            <span class="text-sm font-medium text-gray-900 capitalize">{{ type }}</span>
            <div class="flex items-center space-x-4">
              <span class="text-sm text-gray-600">{{ count }} items</span>
              <div class="w-32 bg-gray-200 rounded-full h-2">
                <div
                  class="bg-green-500 h-2 rounded-full"
                  :style="{ width: `${getContentTypePercentage(count)}%` }"
                />
              </div>
              <span class="text-sm font-semibold text-gray-900 w-12 text-right">
                {{ getContentTypePercentage(count) }}%
              </span>
            </div>
          </div>
        </div>
        <div
          v-else
          class="text-center py-8 text-gray-500"
        >
          <p>No content type statistics available</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import {
  CheckCircleIcon,
  StarIcon,
  ExclamationTriangleIcon,
  ClockIcon,
} from '@heroicons/vue/24/outline'
import { classifierApi } from '../../api/client'
import { PageHeader, LoadingSpinner, ErrorAlert, StatCard } from '../../components/common'

const loading = ref(true)
const error = ref(null)
const stats = ref({
  totalClassified: 0,
  avgQualityScore: 0,
  crimeRelated: 0,
  avgProcessingTime: 0,
})
const topicStats = ref([])
const sourceStats = ref([])
const contentTypeStats = ref({})

const getCrimePercentage = computed(() => {
  if (stats.value.totalClassified === 0) return 0
  return Math.round((stats.value.crimeRelated / stats.value.totalClassified) * 100)
})

const getTopicPercentage = (count) => {
  if (stats.value.totalClassified === 0) return 0
  return Math.round((count / stats.value.totalClassified) * 100)
}

const getContentTypePercentage = (count) => {
  if (stats.value.totalClassified === 0) return 0
  return Math.round((count / stats.value.totalClassified) * 100)
}

const getReputationClass = (reputation) => {
  if (reputation >= 80) return 'bg-green-100 text-green-800'
  if (reputation >= 60) return 'bg-yellow-100 text-yellow-800'
  if (reputation >= 40) return 'bg-orange-100 text-orange-800'
  return 'bg-red-100 text-red-800'
}

const getReputationColorClass = (reputation) => {
  if (reputation >= 80) return 'bg-green-500'
  if (reputation >= 60) return 'bg-yellow-500'
  if (reputation >= 40) return 'bg-orange-500'
  return 'bg-red-500'
}

const loadStats = async () => {
  try {
    loading.value = true
    error.value = null

    const [statsRes, topicsRes, sourcesRes] = await Promise.allSettled([
      classifierApi.stats.get(),
      classifierApi.stats.topics(),
      classifierApi.stats.sources(),
    ])

    if (statsRes.status === 'fulfilled' && statsRes.value.data) {
      const data = statsRes.value.data
      stats.value = {
        totalClassified: data.total_classified || 0,
        avgQualityScore: Math.round(data.avg_quality_score || 0),
        crimeRelated: data.crime_related || 0,
        avgProcessingTime: Math.round(data.avg_processing_time_ms || 0),
      }
      contentTypeStats.value = data.content_types || {}
    }

    if (topicsRes.status === 'fulfilled' && topicsRes.value.data) {
      topicStats.value = topicsRes.value.data.topics || topicsRes.value.data || []
    }

    if (sourcesRes.status === 'fulfilled' && sourcesRes.value.data) {
      sourceStats.value = sourcesRes.value.data.sources || sourcesRes.value.data || []
    }
  } catch (err) {
    error.value = 'Unable to load statistics. Classifier API may not be available yet.'
    console.error('[ClassifierStatsView] Error loading stats:', err)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadStats()
})
</script>

