<template>
  <div>
    <div class="mb-6">
      <router-link
        to="/sources"
        class="text-sm text-gray-500 hover:text-gray-700 inline-flex items-center mb-4"
      >
        <ArrowLeftIcon class="h-4 w-4 mr-1" />
        Back to Sources
      </router-link>
      <h2 class="text-2xl font-bold text-gray-900">
        {{ isEdit ? 'Edit Source' : 'New Source' }}
      </h2>
    </div>

    <div v-if="loading" class="text-center py-12">
      <div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
    </div>

    <form v-else @submit.prevent="handleSubmit" class="bg-white shadow-sm rounded-lg border border-gray-200 p-6">
      <div class="space-y-6">
        <div class="grid grid-cols-1 gap-6 sm:grid-cols-2">
          <div>
            <label for="name" class="block text-sm font-medium text-gray-700">
              Name <span class="text-red-500">*</span>
            </label>
            <input
              id="name"
              v-model="form.name"
              type="text"
              required
              class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
            />
          </div>

          <div>
            <label for="url" class="block text-sm font-medium text-gray-700">
              URL <span class="text-red-500">*</span>
            </label>
            <input
              id="url"
              v-model="form.url"
              type="url"
              required
              class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
            />
          </div>

          <div>
            <label for="article_index" class="block text-sm font-medium text-gray-700">
              Article Index <span class="text-red-500">*</span>
            </label>
            <input
              id="article_index"
              v-model="form.article_index"
              type="text"
              required
              class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
            />
          </div>

          <div>
            <label for="page_index" class="block text-sm font-medium text-gray-700">
              Page Index <span class="text-red-500">*</span>
            </label>
            <input
              id="page_index"
              v-model="form.page_index"
              type="text"
              required
              class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
            />
          </div>

          <div>
            <label for="rate_limit" class="block text-sm font-medium text-gray-700">
              Rate Limit
            </label>
            <input
              id="rate_limit"
              v-model="form.rate_limit"
              type="text"
              placeholder="1s"
              class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
            />
          </div>

          <div>
            <label for="max_depth" class="block text-sm font-medium text-gray-700">
              Max Depth
            </label>
            <input
              id="max_depth"
              v-model.number="form.max_depth"
              type="number"
              min="1"
              class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
            />
          </div>

          <div>
            <label for="city_name" class="block text-sm font-medium text-gray-700">
              City Name
            </label>
            <input
              id="city_name"
              v-model="form.city_name"
              type="text"
              placeholder="sudbury_com"
              class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
            />
          </div>

          <div>
            <label for="group_id" class="block text-sm font-medium text-gray-700">
              Group ID (Drupal UUID)
            </label>
            <input
              id="group_id"
              v-model="form.group_id"
              type="text"
              placeholder="550e8400-e29b-41d4-a716-446655440000"
              class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
            />
          </div>
        </div>

        <div>
          <label class="flex items-center">
            <input
              v-model="form.enabled"
              type="checkbox"
              class="rounded border-gray-300 text-blue-600 shadow-sm focus:border-blue-500 focus:ring-blue-500"
            />
            <span class="ml-2 text-sm text-gray-700">Enabled</span>
          </label>
        </div>

        <!-- Article Selectors -->
        <div class="border-t border-gray-200 pt-6">
          <button
            type="button"
            @click="showArticleSelectors = !showArticleSelectors"
            class="flex w-full items-center justify-between text-left"
          >
            <h3 class="text-lg font-medium text-gray-900">Article Selectors</h3>
            <svg
              :class="['h-5 w-5 text-gray-500 transition-transform', showArticleSelectors ? 'rotate-180' : '']"
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 20 20"
              fill="currentColor"
            >
              <path fill-rule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z" clip-rule="evenodd" />
            </svg>
          </button>
          <div v-show="showArticleSelectors" class="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <label class="block text-sm font-medium text-gray-700">Container</label>
              <input
                v-model="form.selectors.article.container"
                type="text"
                placeholder="article, .article-container"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Title</label>
              <input
                v-model="form.selectors.article.title"
                type="text"
                placeholder="h1, .article-title"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Body</label>
              <input
                v-model="form.selectors.article.body"
                type="text"
                placeholder=".article-body, .content"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Intro</label>
              <input
                v-model="form.selectors.article.intro"
                type="text"
                placeholder=".article-intro, .lead"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Link</label>
              <input
                v-model="form.selectors.article.link"
                type="text"
                placeholder="a.article-link"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Image</label>
              <input
                v-model="form.selectors.article.image"
                type="text"
                placeholder=".article-image img, figure img"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Byline</label>
              <input
                v-model="form.selectors.article.byline"
                type="text"
                placeholder=".byline, .author"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Published Time</label>
              <input
                v-model="form.selectors.article.published_time"
                type="text"
                placeholder="time, .published-date"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Time Ago</label>
              <input
                v-model="form.selectors.article.time_ago"
                type="text"
                placeholder=".time-ago"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Section</label>
              <input
                v-model="form.selectors.article.section"
                type="text"
                placeholder=".section, .category"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Category</label>
              <input
                v-model="form.selectors.article.category"
                type="text"
                placeholder=".category"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Article ID</label>
              <input
                v-model="form.selectors.article.article_id"
                type="text"
                placeholder="[data-article-id]"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">JSON-LD</label>
              <input
                v-model="form.selectors.article.json_ld"
                type="text"
                placeholder="script[type='application/ld+json']"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Keywords</label>
              <input
                v-model="form.selectors.article.keywords"
                type="text"
                placeholder="meta[name='keywords']"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Description</label>
              <input
                v-model="form.selectors.article.description"
                type="text"
                placeholder="meta[name='description']"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">OG Title</label>
              <input
                v-model="form.selectors.article.og_title"
                type="text"
                placeholder="meta[property='og:title']"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">OG Description</label>
              <input
                v-model="form.selectors.article.og_description"
                type="text"
                placeholder="meta[property='og:description']"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">OG Image</label>
              <input
                v-model="form.selectors.article.og_image"
                type="text"
                placeholder="meta[property='og:image']"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">OG URL</label>
              <input
                v-model="form.selectors.article.og_url"
                type="text"
                placeholder="meta[property='og:url']"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">OG Type</label>
              <input
                v-model="form.selectors.article.og_type"
                type="text"
                placeholder="meta[property='og:type']"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">OG Site Name</label>
              <input
                v-model="form.selectors.article.og_site_name"
                type="text"
                placeholder="meta[property='og:site_name']"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Canonical</label>
              <input
                v-model="form.selectors.article.canonical"
                type="text"
                placeholder="link[rel='canonical']"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Author</label>
              <input
                v-model="form.selectors.article.author"
                type="text"
                placeholder=".author-name, [rel='author']"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div class="sm:col-span-2">
              <label class="block text-sm font-medium text-gray-700">Exclude (comma-separated)</label>
              <input
                v-model="articleExcludeInput"
                type="text"
                placeholder=".ad, .social-share, .related-articles"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
              <p class="mt-1 text-xs text-gray-500">CSS selectors to exclude from article content</p>
            </div>
          </div>
        </div>

        <!-- List Selectors -->
        <div class="border-t border-gray-200 pt-6">
          <button
            type="button"
            @click="showListSelectors = !showListSelectors"
            class="flex w-full items-center justify-between text-left"
          >
            <h3 class="text-lg font-medium text-gray-900">List Selectors</h3>
            <svg
              :class="['h-5 w-5 text-gray-500 transition-transform', showListSelectors ? 'rotate-180' : '']"
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 20 20"
              fill="currentColor"
            >
              <path fill-rule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z" clip-rule="evenodd" />
            </svg>
          </button>
          <div v-show="showListSelectors" class="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <label class="block text-sm font-medium text-gray-700">Container</label>
              <input
                v-model="form.selectors.list.container"
                type="text"
                placeholder=".article-list, .news-list"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Article Cards</label>
              <input
                v-model="form.selectors.list.article_cards"
                type="text"
                placeholder=".article-card, .news-item"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Article List</label>
              <input
                v-model="form.selectors.list.article_list"
                type="text"
                placeholder="ul.articles, .article-list"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div class="sm:col-span-2">
              <label class="block text-sm font-medium text-gray-700">Exclude From List (comma-separated)</label>
              <input
                v-model="listExcludeInput"
                type="text"
                placeholder=".sponsored, .ad-card"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
              <p class="mt-1 text-xs text-gray-500">CSS selectors to exclude from article lists</p>
            </div>
          </div>
        </div>

        <!-- Page Selectors -->
        <div class="border-t border-gray-200 pt-6">
          <button
            type="button"
            @click="showPageSelectors = !showPageSelectors"
            class="flex w-full items-center justify-between text-left"
          >
            <h3 class="text-lg font-medium text-gray-900">Page Selectors</h3>
            <svg
              :class="['h-5 w-5 text-gray-500 transition-transform', showPageSelectors ? 'rotate-180' : '']"
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 20 20"
              fill="currentColor"
            >
              <path fill-rule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z" clip-rule="evenodd" />
            </svg>
          </button>
          <div v-show="showPageSelectors" class="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <label class="block text-sm font-medium text-gray-700">Container</label>
              <input
                v-model="form.selectors.page.container"
                type="text"
                placeholder="main, .main-content"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Title</label>
              <input
                v-model="form.selectors.page.title"
                type="text"
                placeholder="h1, .page-title"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Content</label>
              <input
                v-model="form.selectors.page.content"
                type="text"
                placeholder=".page-content, .content"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Description</label>
              <input
                v-model="form.selectors.page.description"
                type="text"
                placeholder="meta[name='description']"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Keywords</label>
              <input
                v-model="form.selectors.page.keywords"
                type="text"
                placeholder="meta[name='keywords']"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">OG Title</label>
              <input
                v-model="form.selectors.page.og_title"
                type="text"
                placeholder="meta[property='og:title']"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">OG Description</label>
              <input
                v-model="form.selectors.page.og_description"
                type="text"
                placeholder="meta[property='og:description']"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">OG Image</label>
              <input
                v-model="form.selectors.page.og_image"
                type="text"
                placeholder="meta[property='og:image']"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">OG URL</label>
              <input
                v-model="form.selectors.page.og_url"
                type="text"
                placeholder="meta[property='og:url']"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-gray-700">Canonical</label>
              <input
                v-model="form.selectors.page.canonical"
                type="text"
                placeholder="link[rel='canonical']"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
            </div>
            <div class="sm:col-span-2">
              <label class="block text-sm font-medium text-gray-700">Exclude (comma-separated)</label>
              <input
                v-model="pageExcludeInput"
                type="text"
                placeholder=".sidebar, .footer, .ad"
                class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
              />
              <p class="mt-1 text-xs text-gray-500">CSS selectors to exclude from page content</p>
            </div>
          </div>
        </div>

        <div v-if="error" class="rounded-md bg-red-50 p-4">
          <div class="flex">
            <ExclamationCircleIcon class="h-5 w-5 text-red-400" />
            <div class="ml-3">
              <h3 class="text-sm font-medium text-red-800">Error</h3>
              <div class="mt-2 text-sm text-red-700">{{ error }}</div>
            </div>
          </div>
        </div>
      </div>

      <div class="mt-6 flex justify-end space-x-3">
        <router-link
          to="/sources"
          class="px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500"
        >
          Cancel
        </router-link>
        <button
          type="submit"
          :disabled="submitting"
          class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          <span v-if="submitting">Saving...</span>
          <span v-else>{{ isEdit ? 'Update' : 'Create' }} Source</span>
        </button>
      </div>
    </form>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { sourcesApi } from '../api/client'
import { ArrowLeftIcon, ExclamationCircleIcon } from '@heroicons/vue/24/outline'

const router = useRouter()
const route = useRoute()

const isEdit = computed(() => !!route.params.id)

const form = ref({
  name: '',
  url: '',
  article_index: '',
  page_index: '',
  rate_limit: '1s',
  max_depth: 2,
  time: [],
  selectors: {
    article: {},
    list: {},
    page: {},
  },
  city_name: null,
  group_id: null,
  enabled: true,
})

const loading = ref(false)
const submitting = ref(false)
const error = ref(null)

// Collapsible sections state
const showArticleSelectors = ref(false)
const showListSelectors = ref(false)
const showPageSelectors = ref(false)

// Exclude fields as comma-separated strings
const articleExcludeInput = ref('')
const listExcludeInput = ref('')
const pageExcludeInput = ref('')

// Watch exclude inputs and update form data
watch(articleExcludeInput, (val) => {
  if (!form.value.selectors.article) form.value.selectors.article = {}
  form.value.selectors.article.exclude = val ? val.split(',').map(s => s.trim()).filter(Boolean) : []
})

watch(listExcludeInput, (val) => {
  if (!form.value.selectors.list) form.value.selectors.list = {}
  form.value.selectors.list.exclude_from_list = val ? val.split(',').map(s => s.trim()).filter(Boolean) : []
})

watch(pageExcludeInput, (val) => {
  if (!form.value.selectors.page) form.value.selectors.page = {}
  form.value.selectors.page.exclude = val ? val.split(',').map(s => s.trim()).filter(Boolean) : []
})

const loadSource = async () => {
  if (!isEdit.value) return

  loading.value = true
  error.value = null
  try {
    const source = await sourcesApi.get(route.params.id)

    // Ensure selectors exist
    if (!source.selectors) {
      source.selectors = { article: {}, list: {}, page: {} }
    }
    if (!source.selectors.article) source.selectors.article = {}
    if (!source.selectors.list) source.selectors.list = {}
    if (!source.selectors.page) source.selectors.page = {}

    form.value = {
      ...source,
      city_name: source.city_name || null,
      group_id: source.group_id || null,
    }

    // Populate exclude input fields from arrays
    if (source.selectors.article?.exclude) {
      articleExcludeInput.value = source.selectors.article.exclude.join(', ')
    }
    if (source.selectors.list?.exclude_from_list) {
      listExcludeInput.value = source.selectors.list.exclude_from_list.join(', ')
    }
    if (source.selectors.page?.exclude) {
      pageExcludeInput.value = source.selectors.page.exclude.join(', ')
    }
  } catch (err) {
    error.value = err.response?.data?.error || err.message || 'Failed to load source'
  } finally {
    loading.value = false
  }
}

const handleSubmit = async () => {
  submitting.value = true
  error.value = null
  
  try {
    const data = {
      ...form.value,
      city_name: form.value.city_name || null,
      group_id: form.value.group_id || null,
    }
    
    if (isEdit.value) {
      await sourcesApi.update(route.params.id, data)
    } else {
      await sourcesApi.create(data)
    }
    
    router.push('/sources')
  } catch (err) {
    error.value = err.response?.data?.error || err.response?.data?.details || err.message || 'Failed to save source'
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  loadSource()
})
</script>

