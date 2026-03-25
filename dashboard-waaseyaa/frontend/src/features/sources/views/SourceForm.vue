<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import ErrorBanner from '@/shared/components/ErrorBanner.vue'
import LoadingSkeleton from '@/shared/components/LoadingSkeleton.vue'
import { useToast } from '@/shared/composables/useToast'
import {
  useSource,
  useCreateSource,
  useUpdateSource,
  useFetchMetadata,
} from '../composables/useSourceApi'
import MetadataPreview from '../components/MetadataPreview.vue'
import type { SourceFormData, SelectorConfig, SourceMetadata } from '../types'
import { SOURCE_TYPES, INGESTION_MODES, RENDER_MODES } from '../types'

const route = useRoute()
const router = useRouter()
const { success, error: showError } = useToast()

const sourceId = computed(() => (route.params.id as string) ?? '')
const isEdit = computed(() => !!sourceId.value)

const { data: existingSource, isLoading: loadingSource } = useSource(sourceId)
const createMutation = useCreateSource()
const updateMutation = useUpdateSource()
const metadataMutation = useFetchMetadata()

const metadata = ref<SourceMetadata | null>(null)

const defaultSelectors: SelectorConfig = {
  article: { container: '', title: '', body: '' },
  list: { container: '', article_cards: '', article_list: '' },
  page: { container: '', title: '', content: '' },
}

const form = ref<SourceFormData>({
  name: '',
  url: '',
  rate_limit: '10',
  max_depth: 3,
  type: 'news',
  enabled: true,
  ingestion_mode: 'crawl',
  feed_poll_interval_minutes: 60,
  render_mode: 'static',
  allow_source_discovery: false,
  selectors: { ...defaultSelectors },
})

onMounted(() => {
  if (isEdit.value && existingSource.value) {
    populateForm()
  }
})

watch(existingSource, (val) => {
  if (val && isEdit.value) populateForm()
})

function populateForm() {
  const s = existingSource.value
  if (!s) return
  form.value = {
    name: s.name,
    url: s.url,
    rate_limit: s.rate_limit,
    max_depth: s.max_depth,
    type: s.type,
    enabled: s.enabled,
    feed_url: s.feed_url ?? undefined,
    sitemap_url: s.sitemap_url ?? undefined,
    ingestion_mode: s.ingestion_mode,
    feed_poll_interval_minutes: s.feed_poll_interval_minutes,
    render_mode: s.render_mode,
    allow_source_discovery: s.allow_source_discovery,
    selectors: s.selectors ?? { ...defaultSelectors },
  }
}

async function fetchMeta() {
  if (!form.value.url) return
  try {
    const result = await metadataMutation.mutateAsync(form.value.url)
    metadata.value = result
    if (result.title && !form.value.name) {
      form.value.name = result.title
    }
    if (result.feed_url && !form.value.feed_url) {
      form.value.feed_url = result.feed_url
    }
    if (result.sitemap_url && !form.value.sitemap_url) {
      form.value.sitemap_url = result.sitemap_url
    }
  } catch {
    showError('Failed to fetch metadata from URL.')
  }
}

async function handleSubmit() {
  try {
    if (isEdit.value) {
      const updated = await updateMutation.mutateAsync({
        id: sourceId.value,
        data: form.value,
      })
      success('Source updated.')
      router.push({ name: 'source-detail', params: { id: updated.id } })
    } else {
      const created = await createMutation.mutateAsync(form.value)
      success('Source created.')
      router.push({ name: 'source-detail', params: { id: created.id } })
    }
  } catch {
    showError(`Failed to ${isEdit.value ? 'update' : 'create'} source.`)
  }
}

const isSaving = computed(() => createMutation.isPending.value || updateMutation.isPending.value)
</script>

<template>
  <div class="max-w-2xl">
    <div class="mb-6">
      <router-link to="/sources" class="text-sm text-slate-400 hover:text-slate-300">
        &larr; Back to Sources
      </router-link>
    </div>

    <h1 class="text-2xl font-bold mb-6">{{ isEdit ? 'Edit Source' : 'Add Source' }}</h1>

    <LoadingSkeleton v-if="isEdit && loadingSource" :lines="10" />

    <form v-else class="space-y-6" @submit.prevent="handleSubmit">
      <!-- URL + Fetch Metadata -->
      <div>
        <label class="block text-sm font-medium text-slate-300 mb-1">URL</label>
        <div class="flex gap-2">
          <input
            v-model="form.url"
            type="url"
            required
            class="flex-1 bg-slate-800 border border-slate-700 rounded px-3 py-2 text-sm text-slate-200 focus:border-blue-500 focus:outline-none"
            placeholder="https://example.com"
          />
          <button
            type="button"
            class="px-3 py-2 text-sm border border-slate-600 rounded text-slate-300 hover:bg-slate-800"
            :disabled="metadataMutation.isPending.value || !form.url"
            @click="fetchMeta"
          >
            {{ metadataMutation.isPending.value ? 'Fetching...' : 'Fetch Metadata' }}
          </button>
        </div>
      </div>

      <MetadataPreview v-if="metadata" :metadata="metadata" />

      <!-- Name -->
      <div>
        <label class="block text-sm font-medium text-slate-300 mb-1">Name</label>
        <input
          v-model="form.name"
          type="text"
          required
          class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-2 text-sm text-slate-200 focus:border-blue-500 focus:outline-none"
          placeholder="Source name"
        />
      </div>

      <!-- Type + Ingestion Mode -->
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium text-slate-300 mb-1">Type</label>
          <select
            v-model="form.type"
            class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-2 text-sm text-slate-200 focus:border-blue-500 focus:outline-none"
          >
            <option v-for="t in SOURCE_TYPES" :key="t" :value="t">{{ t }}</option>
          </select>
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-300 mb-1">Ingestion Mode</label>
          <select
            v-model="form.ingestion_mode"
            class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-2 text-sm text-slate-200 focus:border-blue-500 focus:outline-none"
          >
            <option v-for="m in INGESTION_MODES" :key="m" :value="m">{{ m }}</option>
          </select>
        </div>
      </div>

      <!-- Rate Limit + Max Depth -->
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium text-slate-300 mb-1">Rate Limit (req/min)</label>
          <input
            v-model="form.rate_limit"
            type="text"
            class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-2 text-sm text-slate-200 focus:border-blue-500 focus:outline-none"
          />
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-300 mb-1">Max Depth</label>
          <input
            v-model.number="form.max_depth"
            type="number"
            min="1"
            max="10"
            class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-2 text-sm text-slate-200 focus:border-blue-500 focus:outline-none"
          />
        </div>
      </div>

      <!-- Render Mode + Feed Poll -->
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="block text-sm font-medium text-slate-300 mb-1">Render Mode</label>
          <select
            v-model="form.render_mode"
            class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-2 text-sm text-slate-200 focus:border-blue-500 focus:outline-none"
          >
            <option v-for="m in RENDER_MODES" :key="m" :value="m">{{ m }}</option>
          </select>
        </div>
        <div>
          <label class="block text-sm font-medium text-slate-300 mb-1">Feed Poll (min)</label>
          <input
            v-model.number="form.feed_poll_interval_minutes"
            type="number"
            min="5"
            class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-2 text-sm text-slate-200 focus:border-blue-500 focus:outline-none"
          />
        </div>
      </div>

      <!-- Feed URL + Sitemap URL -->
      <div>
        <label class="block text-sm font-medium text-slate-300 mb-1">Feed URL (optional)</label>
        <input
          v-model="form.feed_url"
          type="url"
          class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-2 text-sm text-slate-200 focus:border-blue-500 focus:outline-none"
          placeholder="https://example.com/feed.xml"
        />
      </div>

      <div>
        <label class="block text-sm font-medium text-slate-300 mb-1">Sitemap URL (optional)</label>
        <input
          v-model="form.sitemap_url"
          type="url"
          class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-2 text-sm text-slate-200 focus:border-blue-500 focus:outline-none"
          placeholder="https://example.com/sitemap.xml"
        />
      </div>

      <!-- Checkboxes -->
      <div class="flex gap-6">
        <label class="flex items-center gap-2 text-sm text-slate-300">
          <input
            v-model="form.enabled"
            type="checkbox"
            class="rounded bg-slate-800 border-slate-600"
          />
          Enabled
        </label>
        <label class="flex items-center gap-2 text-sm text-slate-300">
          <input
            v-model="form.allow_source_discovery"
            type="checkbox"
            class="rounded bg-slate-800 border-slate-600"
          />
          Allow Source Discovery
        </label>
      </div>

      <!-- Article Selectors -->
      <fieldset class="border border-slate-700 rounded-lg p-4">
        <legend class="text-sm font-medium text-slate-300 px-2">Article Selectors</legend>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="block text-xs text-slate-400 mb-1">Container</label>
            <input
              v-model="form.selectors.article.container"
              type="text"
              class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-sm text-slate-200"
              placeholder="article"
            />
          </div>
          <div>
            <label class="block text-xs text-slate-400 mb-1">Title</label>
            <input
              v-model="form.selectors.article.title"
              type="text"
              class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-sm text-slate-200"
              placeholder="h1"
            />
          </div>
          <div class="col-span-2">
            <label class="block text-xs text-slate-400 mb-1">Body</label>
            <input
              v-model="form.selectors.article.body"
              type="text"
              class="w-full bg-slate-800 border border-slate-700 rounded px-3 py-1.5 text-sm text-slate-200"
              placeholder="article > div"
            />
          </div>
        </div>
      </fieldset>

      <!-- Error -->
      <ErrorBanner
        v-if="createMutation.isError.value || updateMutation.isError.value"
        :message="`Failed to ${isEdit ? 'update' : 'create'} source.`"
        @retry="handleSubmit"
      />

      <!-- Submit -->
      <div class="flex gap-3">
        <button
          type="submit"
          :disabled="isSaving"
          class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded hover:bg-blue-500 disabled:opacity-50"
        >
          {{ isSaving ? 'Saving...' : isEdit ? 'Update Source' : 'Create Source' }}
        </button>
        <button
          type="button"
          class="px-4 py-2 text-sm text-slate-300 border border-slate-600 rounded hover:bg-slate-800"
          @click="router.back()"
        >
          Cancel
        </button>
      </div>
    </form>
  </div>
</template>
