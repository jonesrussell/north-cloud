<template>
  <div>
    <PageHeader
      title="Source Reputation"
      subtitle="Source trustworthiness and quality metrics"
    />

    <!-- Loading State -->
    <LoadingSpinner
      v-if="loading"
      size="lg"
      text="Loading source reputation..."
      :full-page="true"
    />

    <!-- Error State -->
    <ErrorAlert
      v-else-if="error"
      :message="error"
      class="mb-6"
    />

    <!-- Sources Content -->
    <div v-else>
      <!-- Overview Stats -->
      <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
        <StatCard
          label="Total Sources"
          :value="sources.length"
          :icon="DocumentTextIcon"
          color="blue"
        />
        <StatCard
          label="Avg Reputation"
          :value="avgReputation"
          :icon="StarIcon"
          color="green"
          format="number"
        >
          <template #footer>
            <span class="text-xs text-gray-500">0-100 scale</span>
          </template>
        </StatCard>
        <StatCard
          label="High Quality Sources"
          :value="highQualityCount"
          :icon="CheckCircleIcon"
          color="green"
        >
          <template #footer>
            <span class="text-xs text-gray-500">Reputation ≥ 80</span>
          </template>
        </StatCard>
      </div>

      <!-- Sources Table -->
      <div class="bg-white shadow rounded-lg overflow-hidden">
        <div class="px-6 py-4 border-b border-gray-200">
          <h2 class="text-lg font-medium text-gray-900">
            Source Reputation Scores
          </h2>
        </div>
        <table class="min-w-full divide-y divide-gray-200">
          <thead class="bg-gray-50">
            <tr>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Source Name
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Reputation
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Category
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Total Classified
              </th>
              <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Avg Quality
              </th>
              <th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody class="bg-white divide-y divide-gray-200">
            <tr v-if="sources.length === 0">
              <td
                colspan="6"
                class="px-6 py-8 text-center text-sm text-gray-500"
              >
                No sources found.
              </td>
            </tr>
            <tr
              v-for="source in sortedSources"
              :key="source.name"
              class="hover:bg-gray-50"
            >
              <td class="px-6 py-4 whitespace-nowrap">
                <span class="text-sm font-medium text-gray-900">{{ source.name }}</span>
              </td>
              <td class="px-6 py-4 whitespace-nowrap">
                <div class="flex items-center">
                  <div
                    class="flex-1 bg-gray-200 rounded-full h-2 mr-3"
                    style="width: 100px"
                  >
                    <div
                      class="h-2 rounded-full"
                      :class="getReputationColorClass(source.reputation)"
                      :style="{ width: `${source.reputation}%` }"
                    />
                  </div>
                  <span
                    class="text-sm font-semibold"
                    :class="getReputationTextClass(source.reputation)"
                  >
                    {{ source.reputation }}/100
                  </span>
                </div>
              </td>
              <td class="px-6 py-4 whitespace-nowrap">
                <span class="text-sm text-gray-500 capitalize">{{ source.category || 'Unknown' }}</span>
              </td>
              <td class="px-6 py-4 whitespace-nowrap">
                <span class="text-sm text-gray-900">{{ source.total_classified || 0 }}</span>
              </td>
              <td class="px-6 py-4 whitespace-nowrap">
                <span class="text-sm text-gray-900">{{ source.avg_quality || 0 }}</span>
              </td>
              <td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                <button
                  class="text-blue-600 hover:text-blue-900"
                  @click="viewSourceDetails(source)"
                >
                  View Details
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Source Details Modal -->
    <div
      v-if="selectedSource"
      class="fixed inset-0 bg-gray-600 bg-opacity-50 overflow-y-auto h-full w-full z-50"
      @click.self="selectedSource = null"
    >
      <div class="relative top-20 mx-auto p-5 border w-96 shadow-lg rounded-md bg-white">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-lg font-medium text-gray-900">
            {{ selectedSource.name }}
          </h3>
          <button
            class="text-gray-400 hover:text-gray-500"
            @click="selectedSource = null"
          >
            <XMarkIcon class="h-6 w-6" />
          </button>
        </div>
        <div class="space-y-4">
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">Reputation Score</label>
            <div class="flex items-center">
              <div class="flex-1 bg-gray-200 rounded-full h-3 mr-3">
                <div
                  class="h-3 rounded-full"
                  :class="getReputationColorClass(selectedSource.reputation)"
                  :style="{ width: `${selectedSource.reputation}%` }"
                />
              </div>
              <span class="text-lg font-semibold">{{ selectedSource.reputation }}/100</span>
            </div>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">Category</label>
            <span class="text-sm text-gray-900 capitalize">{{ selectedSource.category || 'Unknown' }}</span>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">Total Classified</label>
            <span class="text-sm text-gray-900">{{ selectedSource.total_classified || 0 }} items</span>
          </div>
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-1">Average Quality Score</label>
            <span class="text-sm text-gray-900">{{ selectedSource.avg_quality || 0 }}/100</span>
          </div>
          <div v-if="selectedSource.last_updated">
            <label class="block text-sm font-medium text-gray-700 mb-1">Last Updated</label>
            <span class="text-sm text-gray-900">{{ formatDate(selectedSource.last_updated) }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import {
  DocumentTextIcon,
  StarIcon,
  CheckCircleIcon,
  XMarkIcon,
} from '@heroicons/vue/24/outline'
import { classifierApi } from '../../api/client'
import {
  PageHeader,
  LoadingSpinner,
  ErrorAlert,
  StatCard,
} from '../../components/common'

const loading = ref(true)
const error = ref(null)
const sources = ref([])
const selectedSource = ref(null)

const avgReputation = computed(() => {
  if (sources.value.length === 0) return 0
  const sum = sources.value.reduce((acc, s) => acc + (s.reputation || 0), 0)
  return Math.round(sum / sources.value.length)
})

const highQualityCount = computed(() => {
  return sources.value.filter((s) => (s.reputation || 0) >= 80).length
})

const sortedSources = computed(() => {
  return [...sources.value].sort((a, b) => (b.reputation || 0) - (a.reputation || 0))
})

const getReputationColorClass = (reputation) => {
  if (reputation >= 80) return 'bg-green-500'
  if (reputation >= 60) return 'bg-yellow-500'
  if (reputation >= 40) return 'bg-orange-500'
  return 'bg-red-500'
}

const getReputationTextClass = (reputation) => {
  if (reputation >= 80) return 'text-green-600'
  if (reputation >= 60) return 'text-yellow-600'
  if (reputation >= 40) return 'text-orange-600'
  return 'text-red-600'
}

const formatDate = (dateString) => {
  if (!dateString) return 'N/A'
  try {
    const date = new Date(dateString)
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
}

const viewSourceDetails = async (source) => {
  try {
    const res = await classifierApi.sources.get(source.name)
    selectedSource.value = res.data || source
  } catch (err) {
    // If detailed fetch fails, just show the basic info
    selectedSource.value = source
    console.error('[SourceReputationView] Error loading source details:', err)
  }
}

const loadSources = async () => {
  try {
    loading.value = true
    error.value = null

    const res = await classifierApi.sources.list()
    sources.value = res.data?.sources || res.data || []
  } catch (err) {
    error.value = 'Unable to load source reputation. Classifier API may not be available yet.'
    console.error('[SourceReputationView] Error loading sources:', err)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadSources()
})
</script>

