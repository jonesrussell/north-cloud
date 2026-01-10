<template>
  <div>
    <PageHeader
      :title="document?.title || 'Document Details'"
      :subtitle="document ? `Document ID: ${truncateId(document.id)}` : 'Loading...'"
      :back-link="backLink"
      back-text="Back to Index"
    />

    <!-- Loading State -->
    <LoadingSpinner
      v-if="loading"
      size="lg"
      text="Loading document details..."
      :full-page="true"
    />

    <!-- Error State -->
    <ErrorAlert
      v-else-if="error"
      :message="error"
      class="mb-6"
    />

    <!-- Document Details -->
    <div
      v-else-if="document"
      class="space-y-6"
    >
      <!-- Basic Info Card -->
      <div class="bg-white shadow rounded-lg p-6">
        <h2 class="text-lg font-medium text-gray-900 mb-4">
          Basic Information
        </h2>
        <dl class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Document ID
            </dt>
            <dd class="mt-1 text-sm text-gray-900 font-mono break-all">
              {{ document.id }}
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Title
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ document.title || 'N/A' }}
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              URL
            </dt>
            <dd class="mt-1 text-sm">
              <a
                v-if="document.url"
                :href="document.url"
                target="_blank"
                rel="noopener noreferrer"
                class="text-blue-600 hover:text-blue-800 break-all"
              >
                {{ document.url }}
              </a>
              <span
                v-else
                class="text-gray-500"
              >N/A</span>
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Source Name
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ document.source_name || 'N/A' }}
            </dd>
          </div>
        </dl>
      </div>

      <!-- Metadata Card -->
      <div class="bg-white shadow rounded-lg p-6">
        <h2 class="text-lg font-medium text-gray-900 mb-4">
          Metadata
        </h2>
        <dl class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Content Type
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ document.content_type || (document.meta?.content_type as string) || (document.meta?.og_type as string) || 'N/A' }}
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Quality Score
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ document.quality_score ?? (document.meta?.quality_score as number) ?? 'N/A' }}
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Word Count
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ (document.meta?.word_count as number) ?? 'N/A' }}
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Published Date
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ formatDate(document.published_date || (document.meta?.published_date as string)) }}
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Crawled At
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ formatDate(document.crawled_at) }}
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              Classification Status
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ (document.meta?.classification_status as string) || 'N/A' }}
            </dd>
          </div>
          <div>
            <dt class="text-sm font-medium text-gray-500">
              OG Type
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ (document.meta?.og_type as string) || 'N/A' }}
            </dd>
          </div>
          <div
            v-if="document.meta?.article_section"
          >
            <dt class="text-sm font-medium text-gray-500">
              Article Section
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ document.meta.article_section as string }}
            </dd>
          </div>
          <div
            v-if="document.meta?.canonical_url"
          >
            <dt class="text-sm font-medium text-gray-500">
              Canonical URL
            </dt>
            <dd class="mt-1 text-sm">
              <a
                :href="document.meta.canonical_url as string"
                target="_blank"
                rel="noopener noreferrer"
                class="text-blue-600 hover:text-blue-800 break-all"
              >
                {{ document.meta.canonical_url as string }}
              </a>
            </dd>
          </div>
          <div
            v-if="document.meta?.twitter_card"
          >
            <dt class="text-sm font-medium text-gray-500">
              Twitter Card
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ document.meta.twitter_card as string }}
            </dd>
          </div>
          <div
            v-if="document.meta?.twitter_site"
          >
            <dt class="text-sm font-medium text-gray-500">
              Twitter Site
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ document.meta.twitter_site as string }}
            </dd>
          </div>
          <div
            v-if="document.meta?.og_image"
          >
            <dt class="text-sm font-medium text-gray-500">
              OG Image
            </dt>
            <dd class="mt-1 text-sm">
              <a
                :href="document.meta.og_image as string"
                target="_blank"
                rel="noopener noreferrer"
                class="text-blue-600 hover:text-blue-800 break-all"
              >
                {{ document.meta.og_image as string }}
              </a>
              <span
                v-if="document.meta.og_image_width || document.meta.og_image_height"
                class="ml-2 text-gray-500"
              >
                ({{ document.meta.og_image_width as number }} × {{ document.meta.og_image_height as number }})
              </span>
            </dd>
          </div>
          <div
            v-else-if="document.meta?.og_image_width || document.meta?.og_image_height"
          >
            <dt class="text-sm font-medium text-gray-500">
              OG Image Dimensions
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ document.meta.og_image_width as number }} × {{ document.meta.og_image_height as number }}
            </dd>
          </div>
          <div
            v-if="document.meta?.og_site_name"
          >
            <dt class="text-sm font-medium text-gray-500">
              OG Site Name
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ document.meta.og_site_name as string }}
            </dd>
          </div>
          <div
            v-if="document.created_at || document.meta?.created_at"
          >
            <dt class="text-sm font-medium text-gray-500">
              Created At
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ formatDate((document.created_at || document.meta?.created_at) as string) }}
            </dd>
          </div>
          <div
            v-if="document.updated_at || document.meta?.updated_at"
          >
            <dt class="text-sm font-medium text-gray-500">
              Updated At
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ formatDate((document.updated_at || document.meta?.updated_at) as string) }}
            </dd>
          </div>
          <div
            v-if="document.topics && document.topics.length > 0"
          >
            <dt class="text-sm font-medium text-gray-500">
              Topics
            </dt>
            <dd class="mt-1">
              <div class="flex flex-wrap gap-2">
                <span
                  v-for="topic in document.topics"
                  :key="topic"
                  class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800"
                >
                  {{ topic }}
                </span>
              </div>
            </dd>
          </div>
          <div
            v-if="document.meta?.og_title"
          >
            <dt class="text-sm font-medium text-gray-500">
              OG Title
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ document.meta.og_title as string }}
            </dd>
          </div>
          <div
            v-if="document.meta?.meta_description"
          >
            <dt class="text-sm font-medium text-gray-500">
              Meta Description
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ document.meta.meta_description as string }}
            </dd>
          </div>
          <div
            v-if="document.meta?.og_description"
          >
            <dt class="text-sm font-medium text-gray-500">
              OG Description
            </dt>
            <dd class="mt-1 text-sm text-gray-900 line-clamp-2">
              {{ document.meta.og_description as string }}
            </dd>
          </div>
          <div
            v-if="document.meta?.author || (document.meta?.json_ld_data && (document.meta.json_ld_data as Record<string, unknown>)?.jsonld_author)"
          >
            <dt class="text-sm font-medium text-gray-500">
              Author
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ (document.meta?.author as string) || ((document.meta?.json_ld_data as Record<string, unknown>)?.jsonld_author as string) }}
            </dd>
          </div>
          <div
            v-if="document.meta?.json_ld_data && (document.meta.json_ld_data as Record<string, unknown>)?.jsonld_keywords"
          >
            <dt class="text-sm font-medium text-gray-500">
              Keywords (JSON-LD)
            </dt>
            <dd class="mt-1">
              <div class="flex flex-wrap gap-2">
                <span
                  v-for="keyword in ((document.meta.json_ld_data as Record<string, unknown>)?.jsonld_keywords as string[] || [])"
                  :key="keyword"
                  class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-purple-100 text-purple-800"
                >
                  {{ keyword }}
                </span>
              </div>
            </dd>
          </div>
          <div
            v-if="document.meta?.json_ld_data && (document.meta.json_ld_data as Record<string, unknown>)?.jsonld_date_created"
          >
            <dt class="text-sm font-medium text-gray-500">
              Date Created (JSON-LD)
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ formatDate((document.meta.json_ld_data as Record<string, unknown>)?.jsonld_date_created as string) }}
            </dd>
          </div>
          <div
            v-if="document.meta?.json_ld_data && (document.meta.json_ld_data as Record<string, unknown>)?.jsonld_date_modified"
          >
            <dt class="text-sm font-medium text-gray-500">
              Date Modified (JSON-LD)
            </dt>
            <dd class="mt-1 text-sm text-gray-900">
              {{ formatDate((document.meta.json_ld_data as Record<string, unknown>)?.jsonld_date_modified as string) }}
            </dd>
          </div>
        </dl>
      </div>

      <!-- JSON-LD Data Card -->
      <div
        v-if="document.meta?.json_ld_data && Object.keys(document.meta.json_ld_data as Record<string, unknown>).length > 0"
        class="bg-white shadow rounded-lg p-6"
      >
        <h2 class="text-lg font-medium text-gray-900 mb-4">
          JSON-LD Structured Data
        </h2>
        <div class="bg-gray-50 rounded-lg p-4 overflow-x-auto">
          <pre class="text-sm text-gray-900 font-mono">{{ formatJSON(document.meta.json_ld_data as Record<string, unknown>) }}</pre>
        </div>
      </div>

      <!-- Content Card -->
      <div
        v-if="document.body || document.raw_text"
        class="bg-white shadow rounded-lg p-6"
      >
        <h2 class="text-lg font-medium text-gray-900 mb-4">
          Content
        </h2>
        <div class="bg-gray-50 rounded-lg p-4 max-h-96 overflow-y-auto">
          <pre class="whitespace-pre-wrap text-sm text-gray-900 font-sans">{{ document.body || document.raw_text }}</pre>
        </div>
      </div>

      <!-- Raw Content Card -->
      <div
        v-if="document.raw_text || document.raw_html"
        class="bg-white shadow rounded-lg p-6"
      >
        <h2 class="text-lg font-medium text-gray-900 mb-4">
          Raw Content
        </h2>
        <div class="space-y-4">
          <div
            v-if="document.raw_text"
          >
            <h3 class="text-sm font-medium text-gray-700 mb-2">
              Raw Text
            </h3>
            <div class="bg-gray-50 rounded-lg p-4 max-h-64 overflow-y-auto">
              <pre class="whitespace-pre-wrap text-sm text-gray-900 font-sans">{{ document.raw_text }}</pre>
            </div>
          </div>
          <div
            v-if="document.raw_html"
          >
            <h3 class="text-sm font-medium text-gray-700 mb-2">
              Raw HTML
            </h3>
            <div class="bg-gray-50 rounded-lg p-4 max-h-64 overflow-y-auto">
              <pre class="text-xs text-gray-900 font-mono whitespace-pre-wrap break-words">{{ document.raw_html }}</pre>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { indexManagerApi } from '../../api/client'
import type { Document } from '../../types/indexManager'
import type { ApiError } from '../../types/common'
import PageHeader from '../../components/common/PageHeader.vue'
import LoadingSpinner from '../../components/common/LoadingSpinner.vue'
import ErrorAlert from '../../components/common/ErrorAlert.vue'

const route = useRoute()
const indexName = computed(() => route.params.index_name as string)
const documentId = computed(() => route.params.document_id as string)
const backLink = computed(() => `/indexes/${indexName.value}`)

const document = ref<Document | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

const loadDocument = async (): Promise<void> => {
  loading.value = true
  error.value = null
  try {
    const response = await indexManagerApi.documents.get(indexName.value, documentId.value)
    document.value = response.data
  } catch (err: unknown) {
    const axiosError = err as ApiError
    if (axiosError.response?.status === 404) {
      error.value = 'Document not found. It may have been deleted or the ID may be incorrect.'
    } else {
      error.value = axiosError.response?.data?.error || 'Failed to load document'
    }
    console.error('[DocumentDetailView] Error loading document:', err)
  } finally {
    loading.value = false
  }
}

const truncateId = (id: string | undefined): string => {
  if (!id) return 'N/A'
  if (id.length <= 16) return id
  return `${id.substring(0, 8)}...${id.substring(id.length - 8)}`
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

const formatJSON = (obj: Record<string, unknown>): string => {
  try {
    return JSON.stringify(obj, null, 2)
  } catch {
    return String(obj)
  }
}

onMounted(() => {
  loadDocument()
})
</script>
