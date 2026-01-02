<template>
  <div>
    <PageHeader
      title="Recent Published Articles"
      subtitle="Recently posted articles to PubSub"
    />

    <!-- Loading State -->
    <LoadingSpinner
      v-if="loading"
      size="lg"
      text="Loading articles..."
      :full-page="true"
    />

    <!-- Error State -->
    <ErrorAlert
      v-else-if="error"
      :message="error"
      class="mb-6"
    />

    <!-- Articles Content -->
    <div v-else>
      <!-- Filters and Search -->
      <div class="bg-white shadow rounded-lg p-4 mb-6">
        <div class="grid grid-cols-1 gap-4 md:grid-cols-4">
          <!-- Search -->
          <div class="md:col-span-2">
            <label
              for="search"
              class="block text-sm font-medium text-gray-700 mb-1"
            >
              Search Title/URL
            </label>
            <input
              id="search"
              v-model="filters.search"
              type="text"
              placeholder="Search by title or URL..."
              class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
              @input="debouncedLoadArticles"
            >
          </div>

          <!-- Channel Filter -->
          <div>
            <label
              for="channel"
              class="block text-sm font-medium text-gray-700 mb-1"
            >
              Channel
            </label>
            <select
              id="channel"
              v-model="filters.channel_name"
              class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
              @change="loadArticles"
            >
              <option value="">
                All Channels
              </option>
              <option
                v-for="channel in channels"
                :key="channel.id"
                :value="channel.name"
              >
                {{ channel.name }}
              </option>
            </select>
          </div>

          <!-- Date Range Filter -->
          <div>
            <label
              for="date-range"
              class="block text-sm font-medium text-gray-700 mb-1"
            >
              Date Range
            </label>
            <select
              id="date-range"
              v-model="filters.dateRange"
              class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
              @change="onDateRangeChange"
            >
              <option value="">
                All Time
              </option>
              <option value="today">
                Today
              </option>
              <option value="week">
                Last 7 Days
              </option>
              <option value="month">
                Last 30 Days
              </option>
              <option value="custom">
                Custom Range
              </option>
            </select>
          </div>
        </div>

        <!-- Custom Date Range -->
        <div
          v-if="filters.dateRange === 'custom'"
          class="mt-4 grid grid-cols-1 gap-4 md:grid-cols-2"
        >
          <div>
            <label
              for="start-date"
              class="block text-sm font-medium text-gray-700 mb-1"
            >
              Start Date
            </label>
            <input
              id="start-date"
              v-model="filters.start_date"
              type="date"
              class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
              @change="loadArticles"
            >
          </div>
          <div>
            <label
              for="end-date"
              class="block text-sm font-medium text-gray-700 mb-1"
            >
              End Date
            </label>
            <input
              id="end-date"
              v-model="filters.end_date"
              type="date"
              class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
              @change="loadArticles"
            >
          </div>
        </div>

        <!-- Sort and Limit Controls -->
        <div class="mt-4 flex items-center gap-4">
          <div class="flex items-center gap-2">
            <label
              for="limit"
              class="text-sm font-medium text-gray-700"
            >
              Per Page:
            </label>
            <select
              id="limit"
              v-model="filters.limit"
              class="px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
              @change="loadArticles"
            >
              <option :value="25">
                25
              </option>
              <option :value="50">
                50
              </option>
              <option :value="100">
                100
              </option>
              <option :value="200">
                200
              </option>
            </select>
          </div>
        </div>
      </div>

      <!-- Articles List -->
      <div class="bg-white shadow rounded-lg overflow-hidden">
        <div class="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
          <h2 class="text-lg font-medium text-gray-900">
            Recent Articles
          </h2>
          <button
            type="button"
            :disabled="clearing || articles.length === 0"
            class="inline-flex items-center px-3 py-2 border border-transparent text-sm leading-4 font-medium rounded-md text-white bg-red-600 hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500 disabled:bg-gray-400 disabled:cursor-not-allowed"
            @click="clearAllHistory"
          >
            {{ clearing ? 'Clearing...' : 'Clear All' }}
          </button>
        </div>

        <div
          v-if="articles.length === 0"
          class="text-center py-12 text-gray-500"
        >
          <NewspaperIcon class="mx-auto h-12 w-12 text-gray-400 mb-4" />
          <p>No articles found</p>
          <p
            v-if="hasActiveFilters"
            class="text-sm mt-2"
          >
            Try adjusting your filters
          </p>
        </div>

        <ul
          v-else
          class="divide-y divide-gray-200"
        >
          <li
            v-for="article in articles"
            :key="article.id"
            class="px-6 py-4 hover:bg-gray-50 transition-colors"
          >
            <div class="flex items-start justify-between">
              <div class="flex-1 min-w-0">
                <div class="mb-2">
                  <h3 class="text-sm font-medium text-gray-900 truncate">
                    {{ article.article_title || 'Untitled Article' }}
                  </h3>
                </div>
                <div class="flex items-center space-x-4 text-xs text-gray-500">
                  <span class="flex items-center">
                    <NewspaperIcon class="h-4 w-4 mr-1" />
                    {{ article.source_name || 'Unknown' }}
                  </span>
                  <span
                    v-if="article.channel_name"
                    class="flex items-center"
                  >
                    <TagIcon class="h-4 w-4 mr-1" />
                    {{ article.channel_name }}
                  </span>
                  <span class="flex items-center">
                    <ClockIcon class="h-4 w-4 mr-1" />
                    {{ formatDate(article.published_at) }}
                  </span>
                  <span
                    v-if="article.quality_score"
                    class="flex items-center"
                  >
                    <StarIcon class="h-4 w-4 mr-1" />
                    Quality: {{ article.quality_score }}
                  </span>
                </div>
                <div
                  v-if="article.article_url"
                  class="mt-2"
                >
                  <a
                    :href="article.article_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="text-xs text-blue-600 hover:text-blue-500 truncate block"
                  >
                    {{ article.article_url }}
                  </a>
                </div>
                <div
                  v-if="article.topics && article.topics.length > 0"
                  class="mt-2 flex flex-wrap gap-1"
                >
                  <span
                    v-for="topic in article.topics"
                    :key="topic"
                    class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-blue-100 text-blue-800"
                  >
                    {{ topic }}
                  </span>
                </div>
              </div>
            </div>
          </li>
        </ul>

        <!-- Pagination -->
        <div
          v-if="total > filters.limit"
          class="bg-white px-4 py-3 border-t border-gray-200 sm:px-6 flex items-center justify-between"
        >
          <div class="flex-1 flex justify-between sm:hidden">
            <button
              :disabled="filters.offset === 0"
              class="relative inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
              @click="previousPage"
            >
              Previous
            </button>
            <button
              :disabled="filters.offset + filters.limit >= total"
              class="ml-3 relative inline-flex items-center px-4 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
              @click="nextPage"
            >
              Next
            </button>
          </div>
          <div class="hidden sm:flex-1 sm:flex sm:items-center sm:justify-between">
            <div>
              <p class="text-sm text-gray-700">
                Showing
                <span class="font-medium">{{ filters.offset + 1 }}</span>
                to
                <span class="font-medium">{{ Math.min(filters.offset + filters.limit, total) }}</span>
                of
                <span class="font-medium">{{ total }}</span>
                results
              </p>
            </div>
            <div>
              <nav
                class="relative z-0 inline-flex rounded-md shadow-sm -space-x-px"
                aria-label="Pagination"
              >
                <button
                  :disabled="filters.offset === 0"
                  class="relative inline-flex items-center px-2 py-2 rounded-l-md border border-gray-300 bg-white text-sm font-medium text-gray-500 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                  @click="previousPage"
                >
                  Previous
                </button>
                <button
                  :disabled="filters.offset + filters.limit >= total"
                  class="relative inline-flex items-center px-2 py-2 rounded-r-md border border-gray-300 bg-white text-sm font-medium text-gray-500 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                  @click="nextPage"
                >
                  Next
                </button>
              </nav>
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
  NewspaperIcon,
  ClockIcon,
  TagIcon,
  StarIcon,
} from '@heroicons/vue/24/outline'
import { publisherApi } from '../../api/client'
import { PageHeader, LoadingSpinner, ErrorAlert } from '../../components/common'
import type { PublishHistoryItem, Channel } from '../../types/publisher'

const loading = ref(true)
const error = ref<string | null>(null)
const articles = ref<PublishHistoryItem[]>([])
const channels = ref<Channel[]>([])
const total = ref(0)
const clearing = ref(false)

const filters = ref({
  search: '',
  channel_name: '',
  dateRange: '',
  start_date: '',
  end_date: '',
  limit: 50,
  offset: 0,
})

const hasActiveFilters = computed(() => {
  return !!(
    filters.value.search ||
    filters.value.channel_name ||
    filters.value.dateRange ||
    filters.value.start_date ||
    filters.value.end_date
  )
})

let debounceTimer: ReturnType<typeof setTimeout> | null = null

const debouncedLoadArticles = () => {
  if (debounceTimer) {
    clearTimeout(debounceTimer)
  }
  debounceTimer = setTimeout(() => {
    filters.value.offset = 0 // Reset to first page on search
    loadArticles()
  }, 500)
}

const onDateRangeChange = () => {
  const today = new Date()
  const formatDate = (date: Date) => date.toISOString().split('T')[0]

  switch (filters.value.dateRange) {
    case 'today':
      filters.value.start_date = formatDate(today)
      filters.value.end_date = formatDate(today)
      break
    case 'week': {
      const weekAgo = new Date(today)
      weekAgo.setDate(today.getDate() - 7)
      filters.value.start_date = formatDate(weekAgo)
      filters.value.end_date = formatDate(today)
      break
    }
    case 'month': {
      const monthAgo = new Date(today)
      monthAgo.setDate(today.getDate() - 30)
      filters.value.start_date = formatDate(monthAgo)
      filters.value.end_date = formatDate(today)
      break
    }
    case 'custom':
      // Keep existing dates or clear them
      if (!filters.value.start_date) filters.value.start_date = ''
      if (!filters.value.end_date) filters.value.end_date = ''
      break
    default:
      filters.value.start_date = ''
      filters.value.end_date = ''
  }

  loadArticles()
}

const loadArticles = async () => {
  try {
    loading.value = true
    error.value = null

    const params: {
      limit: number
      offset: number
      channel_name?: string
      start_date?: string
      end_date?: string
    } = {
      limit: filters.value.limit,
      offset: filters.value.offset,
    }

    if (filters.value.channel_name) {
      params.channel_name = filters.value.channel_name
    }

    if (filters.value.start_date) {
      params.start_date = filters.value.start_date
    }

    if (filters.value.end_date) {
      params.end_date = filters.value.end_date
    }

    const response = await publisherApi.history.list(params)

    // Filter by search term on client side (since backend doesn't support title/URL search yet)
    let filteredArticles = response.data?.history || []
    if (filters.value.search) {
      const searchLower = filters.value.search.toLowerCase()
      filteredArticles = filteredArticles.filter(
        (article) =>
          article.article_title?.toLowerCase().includes(searchLower) ||
          article.article_url?.toLowerCase().includes(searchLower)
      )
    }

    articles.value = filteredArticles
    // Use count from response, or calculate from filtered results if search is active
    total.value = filters.value.search
      ? filteredArticles.length
      : response.data?.count || response.data?.total || filteredArticles.length
  } catch (err) {
    error.value = 'Unable to load recent articles. Publisher API may not be available yet.'
    console.error('[RecentArticlesView] Error loading articles:', err)
    articles.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

const loadChannels = async () => {
  try {
    const response = await publisherApi.channels.list()
    channels.value = response.data?.channels || []
  } catch (err) {
    console.error('[RecentArticlesView] Error loading channels:', err)
    // Non-fatal - just won't have channel filter
  }
}

const previousPage = () => {
  if (filters.value.offset > 0) {
    filters.value.offset -= filters.value.limit
    loadArticles()
  }
}

const nextPage = () => {
  if (filters.value.offset + filters.value.limit < total.value) {
    filters.value.offset += filters.value.limit
    loadArticles()
  }
}

const clearAllHistory = async () => {
  if (!confirm('Are you sure you want to clear all publish history? This action cannot be undone.')) {
    return
  }

  try {
    clearing.value = true
    error.value = null

    await publisherApi.history.clearAll()

    // Reload articles after clearing
    await loadArticles()
  } catch (err) {
    error.value = 'Failed to clear publish history'
    console.error('[RecentArticlesView] Error clearing history:', err)
  } finally {
    clearing.value = false
  }
}

const formatDate = (dateString: string | undefined): string => {
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

onMounted(() => {
  loadChannels()
  loadArticles()
})
</script>
