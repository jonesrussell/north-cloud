<template>
  <div>
    <PageHeader
      :title="isEdit ? 'Edit Source' : 'New Source'"
      back-link="/sources"
      back-text="Back to Sources"
    />

    <!-- Loading State -->
    <LoadingSpinner v-if="loading" size="lg" :full-page="true" />

    <!-- Form -->
    <form v-else @submit.prevent="handleSubmit" class="bg-white shadow-sm rounded-lg border border-gray-200 p-6">
      <div class="space-y-6">
        <!-- Basic Fields -->
        <div class="grid grid-cols-1 gap-6 sm:grid-cols-2">
          <div class="sm:col-span-2">
            <label for="url" class="block text-sm font-medium text-gray-700">
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
              />
              <button
                v-if="!isEdit"
                type="button"
                @click="fetchMetadata"
                :disabled="!form.url || fetchingMetadata"
                class="px-4 py-2 border border-gray-300 rounded-md shadow-sm text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {{ fetchingMetadata ? 'Fetching...' : 'Prefill' }}
              </button>
            </div>
            <p v-if="metadataFetched" class="mt-1 text-xs text-green-600">
              Form prefilled from URL metadata
            </p>
            <p v-else class="mt-1 text-xs text-gray-500">
              Enter a URL and click "Prefill" to auto-fill form fields
            </p>
          </div>

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

        <!-- Enabled Toggle -->
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
        <CollapsibleSection
          title="Article Selectors"
          :open="showArticleSelectors"
          @update:open="showArticleSelectors = $event"
        >
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <SelectorInput v-model="form.selectors.article.container" label="Container" placeholder="article, .article-container" />
            <SelectorInput v-model="form.selectors.article.title" label="Title" placeholder="h1, .article-title" />
            <SelectorInput v-model="form.selectors.article.body" label="Body" placeholder=".article-body, .content" />
            <SelectorInput v-model="form.selectors.article.intro" label="Intro" placeholder=".article-intro, .lead" />
            <SelectorInput v-model="form.selectors.article.link" label="Link" placeholder="a.article-link" />
            <SelectorInput v-model="form.selectors.article.image" label="Image" placeholder=".article-image img, figure img" />
            <SelectorInput v-model="form.selectors.article.byline" label="Byline" placeholder=".byline, .author" />
            <SelectorInput v-model="form.selectors.article.published_time" label="Published Time" placeholder="time, .published-date" />
            <SelectorInput v-model="form.selectors.article.time_ago" label="Time Ago" placeholder=".time-ago" />
            <SelectorInput v-model="form.selectors.article.section" label="Section" placeholder=".section, .category" />
            <SelectorInput v-model="form.selectors.article.category" label="Category" placeholder=".category" />
            <SelectorInput v-model="form.selectors.article.article_id" label="Article ID" placeholder="[data-article-id]" />
            <SelectorInput v-model="form.selectors.article.json_ld" label="JSON-LD" placeholder="script[type='application/ld+json']" />
            <SelectorInput v-model="form.selectors.article.keywords" label="Keywords" placeholder="meta[name='keywords']" />
            <SelectorInput v-model="form.selectors.article.description" label="Description" placeholder="meta[name='description']" />
            <SelectorInput v-model="form.selectors.article.og_title" label="OG Title" placeholder="meta[property='og:title']" />
            <SelectorInput v-model="form.selectors.article.og_description" label="OG Description" placeholder="meta[property='og:description']" />
            <SelectorInput v-model="form.selectors.article.og_image" label="OG Image" placeholder="meta[property='og:image']" />
            <SelectorInput v-model="form.selectors.article.og_url" label="OG URL" placeholder="meta[property='og:url']" />
            <SelectorInput v-model="form.selectors.article.og_type" label="OG Type" placeholder="meta[property='og:type']" />
            <SelectorInput v-model="form.selectors.article.og_site_name" label="OG Site Name" placeholder="meta[property='og:site_name']" />
            <SelectorInput v-model="form.selectors.article.canonical" label="Canonical" placeholder="link[rel='canonical']" />
            <SelectorInput v-model="form.selectors.article.author" label="Author" placeholder=".author-name, [rel='author']" />
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
            <SelectorInput v-model="form.selectors.list.container" label="Container" placeholder=".article-list, .news-list" />
            <SelectorInput v-model="form.selectors.list.article_cards" label="Article Cards" placeholder=".article-card, .news-item" />
            <SelectorInput v-model="form.selectors.list.article_list" label="Article List" placeholder="ul.articles, .article-list" />
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
            <SelectorInput v-model="form.selectors.page.container" label="Container" placeholder="main, .main-content" />
            <SelectorInput v-model="form.selectors.page.title" label="Title" placeholder="h1, .page-title" />
            <SelectorInput v-model="form.selectors.page.content" label="Content" placeholder=".page-content, .content" />
            <SelectorInput v-model="form.selectors.page.description" label="Description" placeholder="meta[name='description']" />
            <SelectorInput v-model="form.selectors.page.keywords" label="Keywords" placeholder="meta[name='keywords']" />
            <SelectorInput v-model="form.selectors.page.og_title" label="OG Title" placeholder="meta[property='og:title']" />
            <SelectorInput v-model="form.selectors.page.og_description" label="OG Description" placeholder="meta[property='og:description']" />
            <SelectorInput v-model="form.selectors.page.og_image" label="OG Image" placeholder="meta[property='og:image']" />
            <SelectorInput v-model="form.selectors.page.og_url" label="OG URL" placeholder="meta[property='og:url']" />
            <SelectorInput v-model="form.selectors.page.canonical" label="Canonical" placeholder="link[rel='canonical']" />
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
        <ErrorAlert v-if="error" :message="error" />
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
import { ref, computed, onMounted, watch, defineComponent, h } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ChevronDownIcon } from '@heroicons/vue/24/outline'
import { sourcesApi } from '../../api/client'
import { PageHeader, LoadingSpinner, ErrorAlert } from '../../components/common'

// Inline components for cleaner code
const SelectorInput = defineComponent({
  props: {
    modelValue: String,
    label: String,
    placeholder: String,
    hint: String,
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    return () => h('div', [
      h('label', { class: 'block text-sm font-medium text-gray-700' }, props.label),
      h('input', {
        type: 'text',
        value: props.modelValue,
        placeholder: props.placeholder,
        class: 'mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm',
        onInput: (e) => emit('update:modelValue', e.target.value),
      }),
      props.hint && h('p', { class: 'mt-1 text-xs text-gray-500' }, props.hint),
    ])
  },
})

const CollapsibleSection = defineComponent({
  props: {
    title: String,
    open: Boolean,
  },
  emits: ['update:open'],
  setup(props, { emit, slots }) {
    return () => h('div', { class: 'border-t border-gray-200 pt-6' }, [
      h('button', {
        type: 'button',
        class: 'flex w-full items-center justify-between text-left',
        onClick: () => emit('update:open', !props.open),
      }, [
        h('h3', { class: 'text-lg font-medium text-gray-900' }, props.title),
        h(ChevronDownIcon, {
          class: ['h-5 w-5 text-gray-500 transition-transform', props.open ? 'rotate-180' : ''],
        }),
      ]),
      props.open && h('div', { class: 'mt-4' }, slots.default?.()),
    ])
  },
})

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
const fetchingMetadata = ref(false)
const metadataFetched = ref(false)

const showArticleSelectors = ref(false)
const showListSelectors = ref(false)
const showPageSelectors = ref(false)

const articleExcludeInput = ref('')
const listExcludeInput = ref('')
const pageExcludeInput = ref('')

// Watch exclude inputs
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

const fetchMetadata = async () => {
  if (!form.value.url) return

  fetchingMetadata.value = true
  error.value = null
  metadataFetched.value = false

  try {
    const response = await sourcesApi.fetchMetadata(form.value.url)
    const metadata = response.data

    if (metadata.name) form.value.name = metadata.name
    if (metadata.article_index) form.value.article_index = metadata.article_index
    if (metadata.page_index) form.value.page_index = metadata.page_index

    if (metadata.selectors) {
      if (metadata.selectors.article) {
        form.value.selectors.article = { ...form.value.selectors.article, ...metadata.selectors.article }
      }
      if (metadata.selectors.list) {
        form.value.selectors.list = { ...form.value.selectors.list, ...metadata.selectors.list }
      }
      if (metadata.selectors.page) {
        form.value.selectors.page = { ...form.value.selectors.page, ...metadata.selectors.page }
      }

      if (metadata.selectors.article?.exclude) {
        articleExcludeInput.value = metadata.selectors.article.exclude.join(', ')
      }
      if (metadata.selectors.list?.exclude_from_list) {
        listExcludeInput.value = metadata.selectors.list.exclude_from_list.join(', ')
      }
      if (metadata.selectors.page?.exclude) {
        pageExcludeInput.value = metadata.selectors.page.exclude.join(', ')
      }
    }

    metadataFetched.value = true

    if (metadata.selectors?.article && Object.keys(metadata.selectors.article).length > 0) {
      showArticleSelectors.value = true
    }
    if (metadata.selectors?.list && Object.keys(metadata.selectors.list).length > 0) {
      showListSelectors.value = true
    }
    if (metadata.selectors?.page && Object.keys(metadata.selectors.page).length > 0) {
      showPageSelectors.value = true
    }
  } catch (err) {
    error.value = err.response?.data?.error || err.response?.data?.details || err.message || 'Failed to fetch metadata'
    metadataFetched.value = false
    console.error('[FormView] Error fetching metadata:', err)
  } finally {
    fetchingMetadata.value = false
  }
}

const loadSource = async () => {
  if (!isEdit.value) return

  loading.value = true
  error.value = null
  try {
    const response = await sourcesApi.get(route.params.id)
    const source = response.data

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
    console.error('[FormView] Error loading source:', err)
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
    console.error('[FormView] Error saving source:', err)
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  loadSource()
})
</script>
