<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">Publisher Dashboard</h1>
      <p class="page-description">Overview of publishing activity and system status</p>
    </div>

    <!-- Stats Overview -->
    <div class="card">
      <h2 style="margin-bottom: 1rem;">Publishing Statistics</h2>

      <div style="margin-bottom: 1rem;">
        <label class="form-label">Time Period:</label>
        <select class="form-select" v-model="selectedPeriod" @change="loadStats" style="max-width: 200px;">
          <option value="today">Today</option>
          <option value="week">Last 7 Days</option>
          <option value="month">Last 30 Days</option>
          <option value="all">All Time</option>
        </select>
      </div>

      <div v-if="loadingStats" class="loading">Loading statistics...</div>

      <div v-else-if="statsError" class="alert alert-error">{{ statsError }}</div>

      <div v-else>
        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 1rem; margin-bottom: 2rem;">
          <div class="stat-card">
            <div class="stat-value">{{ stats.total_articles || 0 }}</div>
            <div class="stat-label">Total Articles Published</div>
          </div>
          <div class="stat-card">
            <div class="stat-value">{{ stats.channel_count || 0 }}</div>
            <div class="stat-label">Active Channels</div>
          </div>
        </div>

        <h3 style="margin-bottom: 1rem;">Articles by Channel</h3>
        <table class="table">
          <thead>
            <tr>
              <th>Channel</th>
              <th>Articles Published</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(count, channel) in stats.by_channel" :key="channel">
              <td><code>{{ channel }}</code></td>
              <td><strong>{{ count }}</strong></td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Recent Publish History -->
    <div class="card">
      <h2 style="margin-bottom: 1rem;">Recent Publish History</h2>

      <div v-if="loadingHistory" class="loading">Loading history...</div>

      <div v-else-if="historyError" class="alert alert-error">{{ historyError }}</div>

      <div v-else>
        <table class="table">
          <thead>
            <tr>
              <th>Article</th>
              <th>Channel</th>
              <th>Quality Score</th>
              <th>Topics</th>
              <th>Published At</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in history" :key="item.id">
              <td>
                <strong>{{ item.article_title }}</strong><br>
                <a :href="item.article_url" target="_blank" style="font-size: 0.875rem; color: #3498db;">
                  {{ truncateUrl(item.article_url) }}
                </a>
              </td>
              <td><code>{{ item.channel_name }}</code></td>
              <td>{{ item.quality_score }}</td>
              <td>
                <span v-if="item.topics && item.topics.length > 0">
                  <span v-for="topic in item.topics" :key="topic" class="badge badge-success" style="margin-right: 0.25rem;">
                    {{ topic }}
                  </span>
                </span>
                <span v-else style="color: #999;">-</span>
              </td>
              <td>{{ formatDate(item.published_at) }}</td>
            </tr>
          </tbody>
        </table>

        <div v-if="history.length === 0" style="text-align: center; padding: 2rem; color: #666;">
          No publish history found.
        </div>

        <div v-if="history.length > 0" style="margin-top: 1rem; text-align: center;">
          <button class="btn btn-secondary" @click="loadMoreHistory" :disabled="loadingMore">
            {{ loadingMore ? 'Loading...' : 'Load More' }}
          </button>
        </div>
      </div>
    </div>

    <!-- System Info -->
    <div class="card">
      <h2 style="margin-bottom: 1rem;">System Information</h2>
      <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 1.5rem;">
        <div>
          <h3 style="font-size: 1rem; margin-bottom: 0.5rem; color: #666;">Sources</h3>
          <p style="font-size: 1.5rem; font-weight: 600;">{{ systemInfo.sources_count }}</p>
          <router-link to="/sources" class="btn btn-sm btn-primary" style="display: inline-block; margin-top: 0.5rem;">
            Manage Sources
          </router-link>
        </div>
        <div>
          <h3 style="font-size: 1rem; margin-bottom: 0.5rem; color: #666;">Channels</h3>
          <p style="font-size: 1.5rem; font-weight: 600;">{{ systemInfo.channels_count }}</p>
          <router-link to="/channels" class="btn btn-sm btn-primary" style="display: inline-block; margin-top: 0.5rem;">
            Manage Channels
          </router-link>
        </div>
        <div>
          <h3 style="font-size: 1rem; margin-bottom: 0.5rem; color: #666;">Active Routes</h3>
          <p style="font-size: 1.5rem; font-weight: 600;">{{ systemInfo.routes_count }}</p>
          <router-link to="/routes" class="btn btn-sm btn-primary" style="display: inline-block; margin-top: 0.5rem;">
            Manage Routes
          </router-link>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { statsAPI, historyAPI, sourcesAPI, channelsAPI, routesAPI } from '../api/publisher'

const selectedPeriod = ref('today')
const stats = ref({})
const loadingStats = ref(false)
const statsError = ref(null)

const history = ref([])
const loadingHistory = ref(false)
const historyError = ref(null)
const historyOffset = ref(0)
const loadingMore = ref(false)

const systemInfo = ref({
  sources_count: 0,
  channels_count: 0,
  routes_count: 0
})

const loadStats = async () => {
  loadingStats.value = true
  statsError.value = null
  try {
    const response = await statsAPI.overview(selectedPeriod.value)
    stats.value = response.data
  } catch (err) {
    statsError.value = err.response?.data?.error || 'Failed to load statistics'
  } finally {
    loadingStats.value = false
  }
}

const loadHistory = async (offset = 0) => {
  if (offset === 0) {
    loadingHistory.value = true
  } else {
    loadingMore.value = true
  }
  historyError.value = null

  try {
    const response = await historyAPI.list({ limit: 20, offset })
    if (offset === 0) {
      history.value = response.data.history || []
    } else {
      history.value = [...history.value, ...(response.data.history || [])]
    }
    historyOffset.value = offset
  } catch (err) {
    historyError.value = err.response?.data?.error || 'Failed to load history'
  } finally {
    loadingHistory.value = false
    loadingMore.value = false
  }
}

const loadMoreHistory = () => {
  loadHistory(historyOffset.value + 20)
}

const loadSystemInfo = async () => {
  try {
    const [sourcesRes, channelsRes, routesRes] = await Promise.all([
      sourcesAPI.list(),
      channelsAPI.list(),
      routesAPI.list()
    ])

    systemInfo.value = {
      sources_count: sourcesRes.data.count || 0,
      channels_count: channelsRes.data.count || 0,
      routes_count: routesRes.data.count || 0
    }
  } catch (err) {
    console.error('Failed to load system info:', err)
  }
}

const formatDate = (dateString) => {
  return new Date(dateString).toLocaleString()
}

const truncateUrl = (url) => {
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

<style scoped>
.stat-card {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: white;
  padding: 1.5rem;
  border-radius: 8px;
  text-align: center;
}

.stat-value {
  font-size: 2.5rem;
  font-weight: 700;
  margin-bottom: 0.5rem;
}

.stat-label {
  font-size: 0.875rem;
  opacity: 0.9;
}
</style>
