<template>
  <div>
    <PageHeader
      title="Publisher Dashboard"
      subtitle="Overview of publishing activity and system status"
    />

    <!-- Stats Overview -->
    <div class="bg-white shadow rounded-lg p-6 mb-6">
      <h2 class="text-lg font-medium text-gray-900 mb-4">
        Publishing Statistics
      </h2>

      <div class="mb-4">
        <label class="block text-sm font-medium text-gray-700 mb-2">
          Time Period:
        </label>
        <select
          v-model="selectedPeriod"
          @change="loadStats"
          class="max-w-xs px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-blue-500 focus:border-blue-500"
        >
          <option value="today">Today</option>
          <option value="week">Last 7 Days</option>
          <option value="month">Last 30 Days</option>
          <option value="all">All Time</option>
        </select>
      </div>

      <LoadingSpinner
        v-if="loadingStats"
        text="Loading statistics..."
      />

      <ErrorAlert
        v-else-if="statsError"
        :message="statsError"
        class="mb-4"
      />

      <div v-else>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
          <StatCard
            label="Total Articles Published"
            :value="stats.total_articles || 0"
            color="blue"
          />
          <StatCard
            label="Active Channels"
            :value="stats.channel_count || 0"
            color="green"
          />
        </div>

        <div v-if="stats.by_channel && Object.keys(stats.by_channel).length > 0">
          <h3 class="text-md font-medium text-gray-900 mb-3">
            Articles by Channel
          </h3>
          <div class="overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-200">
              <thead class="bg-gray-50">
                <tr>
                  <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Channel
                  </th>
                  <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Articles Published
                  </th>
                </tr>
              </thead>
              <tbody class="bg-white divide-y divide-gray-200">
                <tr
                  v-for="(count, channel) in stats.by_channel"
                  :key="channel"
                  class="hover:bg-gray-50"
                >
                  <td class="px-6 py-4 whitespace-nowrap">
                    <code class="text-sm text-gray-900">{{ channel }}</code>
                  </td>
                  <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-gray-900">
                    {{ count }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>

    <!-- Recent Publish History -->
    <div class="bg-white shadow rounded-lg p-6 mb-6">
      <h2 class="text-lg font-medium text-gray-900 mb-4">
        Recent Publish History
      </h2>

      <LoadingSpinner
        v-if="loadingHistory"
        text="Loading history..."
      />

      <ErrorAlert
        v-else-if="historyError"
        :message="historyError"
        class="mb-4"
      />

      <div v-else>
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200">
            <thead class="bg-gray-50">
              <tr>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Article
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Channel
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Quality Score
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Topics
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Published At
                </th>
              </tr>
            </thead>
            <tbody class="bg-white divide-y divide-gray-200">
              <tr
                v-for="item in history"
                :key="item.id"
                class="hover:bg-gray-50"
              >
                <td class="px-6 py-4">
                  <div class="text-sm font-medium text-gray-900">
                    {{ item.article_title }}
                  </div>
                  <a
                    :href="item.article_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="text-xs text-blue-600 hover:text-blue-500 truncate block max-w-xs"
                  >
                    {{ truncateUrl(item.article_url) }}
                  </a>
                </td>
                <td class="px-6 py-4 whitespace-nowrap">
                  <code class="text-sm text-gray-900">{{ item.channel_name }}</code>
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  {{ item.quality_score }}
                </td>
                <td class="px-6 py-4">
                  <div class="flex flex-wrap gap-1">
                    <span
                      v-for="topic in (item.topics || [])"
                      :key="topic"
                      class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-100 text-green-800"
                    >
                      {{ topic }}
                    </span>
                    <span
                      v-if="!item.topics || item.topics.length === 0"
                      class="text-xs text-gray-400"
                    >
                      -
                    </span>
                  </div>
                </td>
                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  {{ formatDate(item.published_at) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div
          v-if="history.length === 0"
          class="text-center py-12 text-gray-500"
        >
          No publish history found.
        </div>

        <div
          v-if="history.length > 0"
          class="mt-4 text-center"
        >
          <button
            class="px-4 py-2 border border-gray-300 rounded-md text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50"
            @click="loadMoreHistory"
            :disabled="loadingMore"
          >
            {{ loadingMore ? 'Loading...' : 'Load More' }}
          </button>
        </div>
      </div>
    </div>

    <!-- System Info -->
    <div class="bg-white shadow rounded-lg p-6">
      <h2 class="text-lg font-medium text-gray-900 mb-4">
        System Information
      </h2>
      <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div>
          <h3 class="text-sm font-medium text-gray-500 mb-1">
            Sources
          </h3>
          <p class="text-2xl font-semibold text-gray-900">
            {{ systemInfo.sources_count }}
          </p>
          <router-link
            to="/publisher/sources"
            class="mt-2 inline-block text-sm text-blue-600 hover:text-blue-500"
          >
            Manage Sources →
          </router-link>
        </div>
        <div>
          <h3 class="text-sm font-medium text-gray-500 mb-1">
            Channels
          </h3>
          <p class="text-2xl font-semibold text-gray-900">
            {{ systemInfo.channels_count }}
          </p>
          <router-link
            to="/publisher/channels"
            class="mt-2 inline-block text-sm text-blue-600 hover:text-blue-500"
          >
            Manage Channels →
          </router-link>
        </div>
        <div>
          <h3 class="text-sm font-medium text-gray-500 mb-1">
            Active Routes
          </h3>
          <p class="text-2xl font-semibold text-gray-900">
            {{ systemInfo.routes_count }}
          </p>
          <router-link
            to="/publisher/routes"
            class="mt-2 inline-block text-sm text-blue-600 hover:text-blue-500"
          >
            Manage Routes →
          </router-link>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { publisherApi } from '../../api/client'
import type { StatsOverviewResponse, PublishHistoryItem, StatsPeriod } from '../../types/publisher'
import { PageHeader, LoadingSpinner, ErrorAlert, StatCard } from '../../components/common'

const selectedPeriod = ref<StatsPeriod>('today')
const stats = ref<StatsOverviewResponse>({
  total_articles: 0,
  channel_count: 0,
  by_channel: {},
})
const loadingStats = ref(false)
const statsError = ref<string | null>(null)

const history = ref<PublishHistoryItem[]>([])
const loadingHistory = ref(false)
const historyError = ref<string | null>(null)
const historyOffset = ref(0)
const loadingMore = ref(false)

const systemInfo = ref({
  sources_count: 0,
  channels_count: 0,
  routes_count: 0,
})

const loadStats = async (): Promise<void> => {
  loadingStats.value = true
  statsError.value = null
  try {
    const response = await publisherApi.stats.overview(selectedPeriod.value)
    stats.value = response.data
  } catch (err) {
    const axiosError = err as { response?: { data?: { error?: string } } }
    statsError.value = axiosError.response?.data?.error || 'Failed to load statistics'
  } finally {
    loadingStats.value = false
  }
}

const loadHistory = async (offset = 0): Promise<void> => {
  if (offset === 0) {
    loadingHistory.value = true
  } else {
    loadingMore.value = true
  }
  historyError.value = null

  try {
    const response = await publisherApi.history.list({ limit: 20, offset })
    if (offset === 0) {
      history.value = response.data.history || []
    } else {
      history.value = [...history.value, ...(response.data.history || [])]
    }
    historyOffset.value = offset
  } catch (err) {
    const axiosError = err as { response?: { data?: { error?: string } } }
    historyError.value = axiosError.response?.data?.error || 'Failed to load history'
  } finally {
    loadingHistory.value = false
    loadingMore.value = false
  }
}

const loadMoreHistory = (): void => {
  loadHistory(historyOffset.value + 20)
}

const loadSystemInfo = async (): Promise<void> => {
  try {
    const [sourcesRes, channelsRes, routesRes] = await Promise.all([
      publisherApi.sources.list(),
      publisherApi.channels.list(),
      publisherApi.routes.list(),
    ])

    systemInfo.value = {
      sources_count: sourcesRes.data.count || sourcesRes.data.sources?.length || 0,
      channels_count: channelsRes.data.count || channelsRes.data.channels?.length || 0,
      routes_count: routesRes.data.count || routesRes.data.routes?.length || 0,
    }
  } catch (err) {
    console.error('Failed to load system info:', err)
  }
}

const formatDate = (dateString: string): string => {
  return new Date(dateString).toLocaleString()
}

const truncateUrl = (url: string): string => {
  if (url.length > 60) {
    return url.substring(0, 57) + '...'
  }
  return url
}

onMounted(() => {
  loadStats()
  loadHistory()
  loadSystemInfo()
})
</script>

