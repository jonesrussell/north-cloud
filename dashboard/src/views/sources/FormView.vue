<template>
  <div>
    <PageHeader
      :title="isEdit ? 'Edit Source' : 'New Source'"
      back-link="/sources"
      back-text="Back to Sources"
    />

    <!-- Loading State -->
    <LoadingSpinner
      v-if="loading"
      size="lg"
      :full-page="true"
    />

    <!-- Form -->
    <form
      v-else
      class="bg-white shadow-sm rounded-lg border border-gray-200 p-6"
      @submit.prevent="handleSubmit"
    >
      <div class="space-y-6">
        <!-- Basic Fields -->
        <div class="grid grid-cols-1 gap-6 sm:grid-cols-2">
          <div class="sm:col-span-2">
            <label
              for="url"
              class="block text-sm font-medium text-gray-700"
            >
              URL <span class="text-red-500">*</span>
            </label>
            <div class="mt-1 flex gap-2">
              <input
                id="url"
                v-model="form.url"
                type="url"
                required
                class="flex-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
                placeholder="https://example.com"
              >
              <button
                v-if="!isEdit"
                type="button"
                :disabled="!form.url || fetchingMetadata"
                class="px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                @click="fetchMetadata"
              >
                {{ fetchingMetadata ? 'Fetching...' : 'Prefill' }}
              </button>
            </div>
            <p
              v-if="metadataFetched"
              class="mt-1 text-xs text-green-600"
            >
              Form prefilled from URL metadata
            </p>
            <p
              v-else
              class="mt-1 text-xs text-gray-500"
            >
              Enter a URL and click "Prefill" to auto-fill form fields
            </p>
          </div>

          <div>
            <label
              for="name"
              class="block text-sm font-medium text-gray-700"
            >
              Name <span class="text-red-500">*</span>
            </label>
            <input
              id="name"
              v-model="form.name"
              type="text"
              required
              class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
            >
          </div>

          <div>
            <label
              for="rate_limit"
              class="block text-sm font-medium text-gray-700"
            >
              Rate Limit
            </label>
            <input
              id="rate_limit"
              v-model="form.rate_limit"
              type="text"
              placeholder="1s"
              class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
            >
          </div>

          <div>
            <label
              for="max_depth"
              class="block text-sm font-medium text-gray-700"
            >
              Max Depth
            </label>
            <input
              id="max_depth"
              v-model.number="form.max_depth"
              type="number"
              min="1"
              class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
            >
          </div>
        </div>

        <!-- Enabled Toggle -->
        <div>
          <label class="flex items-center">
            <input
              v-model="form.enabled"
              type="checkbox"
              class="rounded border-gray-300 text-blue-600 shadow-sm focus:border-blue-500 focus:ring-blue-500"
            >
            <span class="ml-2 text-sm text-gray-700">Enabled</span>
          </label>
        </div>

        <!-- Article Selectors -->
        <CollapsibleSection
          title="Article Selectors"
          :open="showArticleSelectors"
          @update:open="showArticleSelectors = $event"
        >
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <SelectorInput
              v-model="form.selectors.article.container"
              label="Container"
              placeholder="article, .article-container"
            />
            <SelectorInput
              v-model="form.selectors.article.title"
              label="Title"
              placeholder="h1, .article-title"
            />
            <SelectorInput
              v-model="form.selectors.article.body"
              label="Body"
              placeholder=".article-body, .content"
            />
            <SelectorInput
              v-model="form.selectors.article.intro"
              label="Intro"
              placeholder=".article-intro, .lead"
            />
            <SelectorInput
              v-model="form.selectors.article.link"
              label="Link"
              placeholder="a.article-link"
            />
            <SelectorInput
              v-model="form.selectors.article.image"
              label="Image"
              placeholder=".article-image img, figure img"
            />
            <SelectorInput
              v-model="form.selectors.article.byline"
              label="Byline"
              placeholder=".byline, .author"
            />
            <SelectorInput
              v-model="form.selectors.article.published_time"
              label="Published Time"
              placeholder="time, .published-date"
            />
            <SelectorInput
              v-model="form.selectors.article.time_ago"
              label="Time Ago"
              placeholder=".time-ago"
            />
            <SelectorInput
              v-model="form.selectors.article.section"
              label="Section"
              placeholder=".section, .category"
            />
            <SelectorInput
              v-model="form.selectors.article.category"
              label="Category"
              placeholder=".category"
            />
            <SelectorInput
              v-model="form.selectors.article.article_id"
              label="Article ID"
              placeholder="[data-article-id]"
            />
            <SelectorInput
              v-model="form.selectors.article.json_ld"
              label="JSON-LD"
              placeholder="script[type='application/ld+json']"
            />
            <SelectorInput
              v-model="form.selectors.article.keywords"
              label="Keywords"
              placeholder="meta[name='keywords']"
            />
            <SelectorInput
              v-model="form.selectors.article.description"
              label="Description"
              placeholder="meta[name='description']"
            />
            <SelectorInput
              v-model="form.selectors.article.og_title"
              label="OG Title"
              placeholder="meta[property='og:title']"
            />
            <SelectorInput
              v-model="form.selectors.article.og_description"
              label="OG Description"
              placeholder="meta[property='og:description']"
            />
            <SelectorInput
              v-model="form.selectors.article.og_image"
              label="OG Image"
              placeholder="meta[property='og:image']"
            />
            <SelectorInput
              v-model="form.selectors.article.og_url"
              label="OG URL"
              placeholder="meta[property='og:url']"
            />
            <SelectorInput
              v-model="form.selectors.article.og_type"
              label="OG Type"
              placeholder="meta[property='og:type']"
            />
            <SelectorInput
              v-model="form.selectors.article.og_site_name"
              label="OG Site Name"
              placeholder="meta[property='og:site_name']"
            />
            <SelectorInput
              v-model="form.selectors.article.canonical"
              label="Canonical"
              placeholder="link[rel='canonical']"
            />
            <SelectorInput
              v-model="form.selectors.article.author"
              label="Author"
              placeholder=".author-name, [rel='author']"
            />
            <div class="sm:col-span-2">
              <SelectorInput
                v-model="articleExcludeInput"
                label="Exclude (comma-separated)"
                placeholder=".ad, .social-share, .related-articles"
                hint="CSS selectors to exclude from article content"
              />
            </div>
          </div>
        </CollapsibleSection>

        <!-- List Selectors -->
        <CollapsibleSection
          title="List Selectors"
          :open="showListSelectors"
          @update:open="showListSelectors = $event"
        >
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <SelectorInput
              v-model="form.selectors.list.container"
              label="Container"
              placeholder=".article-list, .news-list"
            />
            <SelectorInput
              v-model="form.selectors.list.article_cards"
              label="Article Cards"
              placeholder=".article-card, .news-item"
            />
            <SelectorInput
              v-model="form.selectors.list.article_list"
              label="Article List"
              placeholder="ul.articles, .article-list"
            />
            <div class="sm:col-span-2">
              <SelectorInput
                v-model="listExcludeInput"
                label="Exclude From List (comma-separated)"
                placeholder=".sponsored, .ad-card"
                hint="CSS selectors to exclude from article lists"
              />
            </div>
          </div>
        </CollapsibleSection>

        <!-- Page Selectors -->
        <CollapsibleSection
          title="Page Selectors"
          :open="showPageSelectors"
          @update:open="showPageSelectors = $event"
        >
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <SelectorInput
              v-model="form.selectors.page.container"
              label="Container"
              placeholder="main, .main-content"
            />
            <SelectorInput
              v-model="form.selectors.page.title"
              label="Title"
              placeholder="h1, .page-title"
            />
            <SelectorInput
              v-model="form.selectors.page.content"
              label="Content"
              placeholder=".page-content, .content"
            />
            <SelectorInput
              v-model="form.selectors.page.description"
              label="Description"
              placeholder="meta[name='description']"
            />
            <SelectorInput
              v-model="form.selectors.page.keywords"
              label="Keywords"
              placeholder="meta[name='keywords']"
            />
            <SelectorInput
              v-model="form.selectors.page.og_title"
              label="OG Title"
              placeholder="meta[property='og:title']"
            />
            <SelectorInput
              v-model="form.selectors.page.og_description"
              label="OG Description"
              placeholder="meta[property='og:description']"
            />
            <SelectorInput
              v-model="form.selectors.page.og_image"
              label="OG Image"
              placeholder="meta[property='og:image']"
            />
            <SelectorInput
              v-model="form.selectors.page.og_url"
              label="OG URL"
              placeholder="meta[property='og:url']"
            />
            <SelectorInput
              v-model="form.selectors.page.canonical"
              label="Canonical"
              placeholder="link[rel='canonical']"
            />
            <div class="sm:col-span-2">
              <SelectorInput
                v-model="pageExcludeInput"
                label="Exclude (comma-separated)"
                placeholder=".sidebar, .footer, .ad"
                hint="CSS selectors to exclude from page content"
              />
            </div>
          </div>
        </CollapsibleSection>

        <!-- Error Display -->
        <ErrorAlert
          v-if="error"
          :message="error"
        />
      </div>

      <!-- Form Actions -->
      <div class="mt-6 flex justify-end space-x-3 pt-6 border-t border-gray-200">
        <router-link
          to="/sources"
          class="px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50"
        >
          Cancel
        </router-link>
        <button
          type="submit"
          :disabled="submitting"
          class="px-4 py-2 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {{ submitting ? 'Saving...' : (isEdit ? 'Update' : 'Create') + ' Source' }}
        </button>
      </div>
    </form>
  </div>
</template>

<script setup>
import { watch, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { PageHeader, LoadingSpinner, ErrorAlert, SelectorInput, CollapsibleSection } from '../../components/common'
import { useSourceForm } from '../../composables/useSourceForm'
import { stringToArray } from '../../utils/formHelpers'

// Use composable for form logic
const {
  form,
  isEdit,
  loading,
  submitting,
  error,
  fetchingMetadata,
  metadataFetched,
  articleExcludeInput,
  listExcludeInput,
  pageExcludeInput,
  showArticleSelectors,
  showListSelectors,
  showPageSelectors,
  fetchMetadata,
  loadSource,
  handleSubmit,
} = useSourceForm()

// Watch exclude inputs and sync with form selectors
watch(articleExcludeInput, (val) => {
  if (!form.value.selectors.article) form.value.selectors.article = {}
  form.value.selectors.article.exclude = stringToArray(val)
})

watch(listExcludeInput, (val) => {
  if (!form.value.selectors.list) form.value.selectors.list = {}
  form.value.selectors.list.exclude_from_list = stringToArray(val)
})

watch(pageExcludeInput, (val) => {
  if (!form.value.selectors.page) form.value.selectors.page = {}
  form.value.selectors.page.exclude = stringToArray(val)
})

const route = useRoute()

onMounted(() => {
  loadSource()
  
  // Prefill URL from query parameter (e.g., when coming from queued links)
  const urlParam = route.query.url
  if (urlParam && typeof urlParam === 'string' && !isEdit.value) {
    form.value.url = decodeURIComponent(urlParam)
    // Optionally auto-fetch metadata if URL is provided
    // Uncomment the line below if you want automatic metadata fetching
    // fetchMetadata()
  }
})
</script>
