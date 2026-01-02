<template>
  <div>
    <!-- Loading State -->
    <div
      v-if="loading"
      class="flex items-center justify-center min-h-64"
    >
      <LoadingSpinner
        size="lg"
        text="Loading dashboard..."
      />
    </div>

    <!-- Error State -->
    <ErrorAlert
      v-else-if="error"
      :message="error"
      class="mb-6"
    />

    <!-- Dashboard Content -->
    <div v-else>
      <!-- Overview Stats -->
      <div class="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-4 mb-8">
        <StatCard
          label="Active Jobs"
          :value="stats.activeJobs"
          :icon="BriefcaseIcon"
          color="blue"
        />
        <StatCard
          label="Total Sources"
          :value="stats.totalSources"
          :icon="DocumentTextIcon"
          color="green"
        />
        <StatCard
          label="Articles Posted"
          :value="publisherStats.totalPosted"
          :icon="CheckCircleIcon"
          color="green"
        />
        <StatCard
          label="Classified Content"
          :value="classifierStats.totalClassified"
          :icon="CheckCircleIcon"
          color="purple"
        />
      </div>

      <!-- Quick Actions & Status -->
      <div class="grid grid-cols-1 lg:grid-cols-4 gap-6">
        <!-- Crawler Status -->
        <div class="bg-white shadow rounded-lg">
          <div class="px-6 py-4 border-b border-gray-200">
            <h2 class="text-lg font-medium text-gray-900">
              Crawler Status
            </h2>
          </div>
          <div class="p-6">
            <div
              v-if="crawlerHealth"
              class="space-y-4"
            >
              <div class="flex items-center justify-between">
                <span class="text-sm text-gray-600">Service Status</span>
                <StatusBadge :status="crawlerHealth.status === 'healthy' ? 'active' : 'error'" />
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm text-gray-600">Version</span>
                <span class="text-sm font-medium text-gray-900">{{ crawlerHealth.version || 'N/A' }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm text-gray-600">Uptime</span>
                <span class="text-sm font-medium text-gray-900">{{ crawlerHealth.uptime || 'N/A' }}</span>
              </div>
            </div>
            <div
              v-else
              class="text-sm text-gray-500"
            >
              Unable to fetch crawler status
            </div>
            <div class="mt-4 pt-4 border-t border-gray-200">
              <router-link
                to="/crawler/jobs"
                class="text-sm font-medium text-blue-600 hover:text-blue-500"
              >
                View all jobs &rarr;
              </router-link>
            </div>
          </div>
        </div>

        <!-- Recent Jobs -->
        <div class="bg-white shadow rounded-lg">
          <div class="px-6 py-4 border-b border-gray-200">
            <h2 class="text-lg font-medium text-gray-900">
              Recent Jobs
            </h2>
          </div>
          <div class="p-6">
            <div
              v-if="recentJobs.length > 0"
              class="space-y-3"
            >
              <div
                v-for="job in recentJobs"
                :key="job.id"
                class="flex items-center justify-between py-2"
              >
                <div class="flex-1 min-w-0">
                  <p class="text-sm font-medium text-gray-900 truncate">
                    {{ job.source_name }}
                  </p>
                  <p class="text-xs text-gray-500">
                    {{ formatDate(job.created_at) }}
                  </p>
                </div>
                <StatusBadge
                  :status="job.status"
                  :show-dot="true"
                />
              </div>
            </div>
            <div
              v-else
              class="text-sm text-gray-500 text-center py-4"
            >
              No recent jobs
            </div>
            <div class="mt-4 pt-4 border-t border-gray-200">
              <router-link
                to="/crawler/jobs"
                class="text-sm font-medium text-blue-600 hover:text-blue-500"
              >
                Manage jobs &rarr;
              </router-link>
            </div>
          </div>
        </div>

        <!-- Publisher Status -->
        <div class="bg-white shadow rounded-lg">
          <div class="px-6 py-4 border-b border-gray-200">
            <h2 class="text-lg font-medium text-gray-900">
              Publisher Status
            </h2>
          </div>
          <div class="p-6">
            <div
              v-if="publisherHealth"
              class="space-y-4"
            >
              <div class="flex items-center justify-between">
                <span class="text-sm text-gray-600">Service Status</span>
                <StatusBadge :status="publisherHealth.status === 'healthy' ? 'active' : 'error'" />
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm text-gray-600">Version</span>
                <span class="text-sm font-medium text-gray-900">{{ publisherHealth.version || 'N/A' }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm text-gray-600">Articles Posted</span>
                <span class="text-sm font-medium text-gray-900">{{ publisherStats.totalPosted }}</span>
              </div>
            </div>
            <div
              v-else
              class="text-sm text-gray-500"
            >
              Unable to fetch publisher status
            </div>
            <div class="mt-4 pt-4 border-t border-gray-200">
              <router-link
                to="/publisher/stats"
                class="text-sm font-medium text-blue-600 hover:text-blue-500"
              >
                View statistics &rarr;
              </router-link>
            </div>
          </div>
        </div>

        <!-- Classifier Status -->
        <div class="bg-white shadow rounded-lg">
          <div class="px-6 py-4 border-b border-gray-200">
            <h2 class="text-lg font-medium text-gray-900">
              Classifier Status
            </h2>
          </div>
          <div class="p-6">
            <div
              v-if="classifierHealth"
              class="space-y-4"
            >
              <div class="flex items-center justify-between">
                <span class="text-sm text-gray-600">Service Status</span>
                <StatusBadge :status="classifierHealth.status === 'healthy' ? 'active' : 'error'" />
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm text-gray-600">Total Classified</span>
                <span class="text-sm font-medium text-gray-900">{{ classifierStats.totalClassified }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-sm text-gray-600">Avg Quality</span>
                <span class="text-sm font-medium text-gray-900">{{ classifierStats.avgQualityScore }}/100</span>
              </div>
            </div>
            <div
              v-else
              class="text-sm text-gray-500"
            >
              Unable to fetch classifier status
            </div>
            <div class="mt-4 pt-4 border-t border-gray-200">
              <router-link
                to="/classifier/stats"
                class="text-sm font-medium text-blue-600 hover:text-blue-500"
              >
                View statistics &rarr;
              </router-link>
            </div>
          </div>
        </div>

        <!-- Quick Actions -->
        <div class="bg-white shadow rounded-lg">
          <div class="px-6 py-4 border-b border-gray-200">
            <h2 class="text-lg font-medium text-gray-900">
              Quick Actions
            </h2>
          </div>
          <div class="p-6">
            <div class="space-y-3">
              <router-link
                to="/crawler/jobs"
                class="flex items-center p-3 rounded-lg border border-gray-200 hover:border-blue-300 hover:bg-blue-50 transition-colors"
              >
                <PlusIcon class="h-5 w-5 text-blue-600 mr-3" />
                <span class="text-sm font-medium text-gray-900">Create New Job</span>
              </router-link>
              <router-link
                to="/sources/new"
                class="flex items-center p-3 rounded-lg border border-gray-200 hover:border-green-300 hover:bg-green-50 transition-colors"
              >
                <PlusIcon class="h-5 w-5 text-green-600 mr-3" />
                <span class="text-sm font-medium text-gray-900">Add New Source</span>
              </router-link>
              <router-link
                to="/crawler/stats"
                class="flex items-center p-3 rounded-lg border border-gray-200 hover:border-purple-300 hover:bg-purple-50 transition-colors"
              >
                <ChartBarIcon class="h-5 w-5 text-purple-600 mr-3" />
                <span class="text-sm font-medium text-gray-900">View Statistics</span>
              </router-link>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import {
  BriefcaseIcon,
  DocumentTextIcon,
  CheckCircleIcon,
  PlusIcon,
  ChartBarIcon,
} from '@heroicons/vue/24/outline'
import { crawlerApi, sourcesApi, publisherApi, classifierApi } from '../api/client'
import { LoadingSpinner, ErrorAlert, StatCard, StatusBadge } from '../components/common'

const loading = ref(true)
const error = ref(null)

const crawlerHealth = ref(null)
const publisherHealth = ref(null)
const classifierHealth = ref(null)
const recentJobs = ref([])
const sources = ref([])
const publisherStats = ref({
  totalPosted: 0,
  totalSkipped: 0,
  totalErrors: 0,
})
const classifierStats = ref({
  totalClassified: 0,
  avgQualityScore: 0,
  crimeRelated: 0,
})
const stats = ref({
  activeJobs: 0,
  totalSources: 0,
  articlesCrawled: 0,
  successRate: 0,
})

const formatDate = (dateString) => {
  if (!dateString) return 'N/A'
  const date = new Date(dateString)
  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

const loadDashboard = async () => {
  loading.value = true
  error.value = null

  try {
    // Load data in parallel
    const [healthRes, publisherHealthRes, classifierHealthRes, jobsRes, sourcesRes, publisherStatsRes, classifierStatsRes] = await Promise.allSettled([
      crawlerApi.getHealth(),
      publisherApi.getHealth(),
      classifierApi.getHealth(),
      crawlerApi.jobs.list(),
      sourcesApi.list(),
      publisherApi.stats.get(),
      classifierApi.stats.get(),
    ])

    // Process health
    if (healthRes.status === 'fulfilled') {
      crawlerHealth.value = healthRes.value.data
    }

    // Process publisher health
    if (publisherHealthRes.status === 'fulfilled') {
      publisherHealth.value = publisherHealthRes.value.data
    }

    // Process classifier health
    if (classifierHealthRes.status === 'fulfilled') {
      classifierHealth.value = classifierHealthRes.value.data
    }

    // Process publisher stats
    // Note: /stats/overview returns different format, adapt the response
    if (publisherStatsRes.status === 'fulfilled' && publisherStatsRes.value.data) {
      const data = publisherStatsRes.value.data
      // /stats/overview returns: { total_articles, by_channel, period, channel_count, generated_at }
      // Adapt to expected format: { total_posted, total_skipped, total_errors }
      publisherStats.value = {
        totalPosted: data.total_articles || 0,
        totalSkipped: 0, // Not available in overview endpoint
        totalErrors: 0, // Not available in overview endpoint
      }
    }

    // Process jobs
    if (jobsRes.status === 'fulfilled') {
      const jobs = jobsRes.value.data?.jobs || jobsRes.value.data || []
      recentJobs.value = jobs.slice(0, 5)
      stats.value.activeJobs = jobs.filter(j => j.status === 'processing' || j.status === 'pending').length

      // Calculate success rate
      const completed = jobs.filter(j => j.status === 'completed').length
      const failed = jobs.filter(j => j.status === 'failed').length
      const total = completed + failed
      stats.value.successRate = total > 0 ? Math.round((completed / total) * 100) : 100
    }

    // Process sources
    if (sourcesRes.status === 'fulfilled') {
      sources.value = sourcesRes.value.data?.sources || sourcesRes.value.data || []
      stats.value.totalSources = sources.value.length
    }

    // Process classifier stats
    if (classifierStatsRes.status === 'fulfilled' && classifierStatsRes.value.data) {
      const data = classifierStatsRes.value.data
      classifierStats.value = {
        totalClassified: data.total_classified || 0,
        avgQualityScore: Math.round(data.avg_quality_score || 0),
        crimeRelated: data.crime_related || 0,
      }
    }

    // Try to get stats
    try {
      const statsRes = await crawlerApi.stats.get()
      if (statsRes.data) {
        stats.value.articlesCrawled = statsRes.data.total_articles || statsRes.data.articles_crawled || 0
      }
    } catch {
      // Stats endpoint may not exist, that's okay
    }
  } catch (err) {
    console.error('[Dashboard] Error loading data:', err)
    error.value = err.response?.data?.error || err.message || 'Failed to load dashboard data'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadDashboard()
})
</script>
