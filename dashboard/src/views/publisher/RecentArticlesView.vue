<template>
  <div>
    <PageHeader
      title="Recent Published Articles"
      subtitle="Recently posted articles to Drupal"
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
      <!-- Articles List -->
      <div class="bg-white shadow rounded-lg overflow-hidden">
        <div class="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
          <h2 class="text-lg font-medium text-gray-900">
            Recent Articles
          </h2>
          <div class="flex items-center space-x-4">
            <label class="text-sm text-gray-600">
              Limit:
              <select
                v-model="limit"
                class="ml-2 border border-gray-300 rounded-md px-2 py-1 text-sm"
                @change="loadArticles"
              >
                <option :value="10">10</option>
                <option :value="25">25</option>
                <option :value="50">50</option>
                <option :value="100">100</option>
              </select>
            </label>
            <button
              type="button"
              @click="clearAllHistory"
              :disabled="clearing || articles.length === 0"
              class="inline-flex items-center px-3 py-2 border border-transparent text-sm leading-4 font-medium rounded-md text-white bg-red-600 hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500 disabled:bg-gray-400 disabled:cursor-not-allowed"
            >
              {{ clearing ? 'Clearing...' : 'Clear All' }}
            </button>
          </div>
        </div>

        <div
          v-if="articles.length === 0"
          class="text-center py-12 text-gray-500"
        >
          <NewspaperIcon class="mx-auto h-12 w-12 text-gray-400 mb-4" />
          <p>No articles published yet</p>
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
                <div class="flex items-center space-x-3 mb-2">
                  <h3 class="text-sm font-medium text-gray-900 truncate">
                    {{ article.title || 'Untitled Article' }}
                  </h3>
                  <StatusBadge
                    :status="'published'"
                    class="flex-shrink-0"
                  />
                </div>
                <div class="flex items-center space-x-4 text-xs text-gray-500">
                  <span class="flex items-center">
                    <MapPinIcon class="h-4 w-4 mr-1" />
                    {{ article.city }}
                  </span>
                  <span class="flex items-center">
                    <ClockIcon class="h-4 w-4 mr-1" />
                    {{ formatDate(article.posted_at) }}
                  </span>
                </div>
                <div
                  v-if="article.url"
                  class="mt-2"
                >
                  <a
                    :href="article.url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="text-xs text-blue-600 hover:text-blue-500 truncate block"
                  >
                    {{ article.url }}
                  </a>
                </div>
              </div>
            </div>
          </li>
        </ul>

        <div
          v-if="articles.length > 0"
          class="px-6 py-4 border-t border-gray-200 bg-gray-50"
        >
          <p class="text-sm text-gray-600">
            Showing {{ articles.length }} {{ articles.length === 1 ? 'article' : 'articles' }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  NewspaperIcon,
  ClockIcon,
  MapPinIcon,
} from '@heroicons/vue/24/outline'
import { publisherApi } from '../../api/client'
import { PageHeader, LoadingSpinner, ErrorAlert, StatusBadge } from '../../components/common'

const loading = ref(true)
const error = ref(null)
const articles = ref([])
const limit = ref(50)
const clearing = ref(false)

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

const loadArticles = async () => {
  try {
    loading.value = true
    error.value = null

    const response = await publisherApi.articles.recent({ limit: limit.value })

    if (response.data && response.data.articles) {
      articles.value = response.data.articles
    } else {
      articles.value = []
    }
  } catch (err) {
    error.value = 'Unable to load recent articles. Publisher API may not be available yet.'
    console.error('[RecentArticlesView] Error loading articles:', err)
    articles.value = []
  } finally {
    loading.value = false
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

onMounted(() => {
  loadArticles()
})
</script>
