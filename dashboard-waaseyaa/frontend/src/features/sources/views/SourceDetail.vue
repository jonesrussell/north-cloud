<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import StatusBadge from '@/shared/components/StatusBadge.vue'
import ErrorBanner from '@/shared/components/ErrorBanner.vue'
import LoadingSkeleton from '@/shared/components/LoadingSkeleton.vue'
import GrafanaEmbed from '@/shared/components/GrafanaEmbed.vue'
import ConfirmDialog from '@/shared/components/ConfirmDialog.vue'
import { useToast } from '@/shared/composables/useToast'
import { useSource, useToggleSource, useDeleteSource } from '../composables/useSourceApi'
import { getSourceStatus, formatDateTime, getErrorMessage } from '../utils'
import TestCrawlDialog from '../components/TestCrawlDialog.vue'

const route = useRoute()
const router = useRouter()
const { success, error: showError } = useToast()

const sourceId = computed(() => route.params.id as string)
const { data: source, isLoading, isError, error, refetch } = useSource(sourceId)
const toggleMutation = useToggleSource()
const deleteMutation = useDeleteSource()

const showTestCrawl = ref(false)
const showDeleteDialog = ref(false)

function getStatus(): string {
  if (!source.value) return 'pending'
  return getSourceStatus(source.value)
}

async function handleToggle() {
  if (!source.value) return
  const enabling = !source.value.enabled
  try {
    await toggleMutation.mutateAsync({ id: source.value.id, enabled: enabling })
    success(`Source ${enabling ? 'enabled' : 'disabled'}.`)
  } catch {
    showError(`Failed to ${enabling ? 'enable' : 'disable'} source.`)
  }
}

async function handleDelete() {
  if (!source.value) return
  try {
    await deleteMutation.mutateAsync(source.value.id)
    success('Source deleted.')
    router.push({ name: 'sources' })
  } catch {
    showError('Failed to delete source.')
  } finally {
    showDeleteDialog.value = false
  }
}

const errorMessage = computed(() => getErrorMessage(error.value, 'Failed to load source.'))
</script>

<template>
  <div>
    <div class="mb-6">
      <router-link to="/sources" class="text-sm text-slate-400 hover:text-slate-300">
        &larr; Back to Sources
      </router-link>
    </div>

    <LoadingSkeleton v-if="isLoading" :lines="8" />

    <ErrorBanner v-else-if="isError" :message="errorMessage" @retry="refetch()" />

    <template v-else-if="source">
      <div class="flex items-center justify-between mb-6">
        <div>
          <h1 class="text-2xl font-bold">{{ source.name }}</h1>
          <p class="text-slate-400 text-sm mt-1">{{ source.url }}</p>
        </div>
        <div class="flex gap-2">
          <button
            class="px-3 py-1.5 text-sm border border-slate-600 rounded text-slate-300 hover:bg-slate-800"
            @click="showTestCrawl = true"
          >
            Test Crawl
          </button>
          <button
            class="px-3 py-1.5 text-sm border border-slate-600 rounded text-slate-300 hover:bg-slate-800"
            @click="router.push({ name: 'source-edit', params: { id: source.id } })"
          >
            Edit
          </button>
          <button
            class="px-3 py-1.5 text-sm rounded"
            :class="
              source.enabled
                ? 'bg-amber-600 hover:bg-amber-500 text-white'
                : 'bg-green-600 hover:bg-green-500 text-white'
            "
            @click="handleToggle"
          >
            {{ source.enabled ? 'Disable' : 'Enable' }}
          </button>
          <button
            class="px-3 py-1.5 text-sm bg-red-600 hover:bg-red-500 text-white rounded"
            @click="showDeleteDialog = true"
          >
            Delete
          </button>
        </div>
      </div>

      <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <!-- Info Panel -->
        <div class="bg-slate-900 border border-slate-800 rounded-lg p-6">
          <h2 class="text-lg font-semibold mb-4">Details</h2>
          <dl class="grid grid-cols-2 gap-y-3 text-sm">
            <dt class="text-slate-400">Status</dt>
            <dd><StatusBadge :status="getStatus()" /></dd>

            <dt class="text-slate-400">Type</dt>
            <dd class="text-slate-200 capitalize">{{ source.type }}</dd>

            <dt class="text-slate-400">Rate Limit</dt>
            <dd class="text-slate-200">{{ source.rate_limit }} req/min</dd>

            <dt class="text-slate-400">Max Depth</dt>
            <dd class="text-slate-200">{{ source.max_depth }}</dd>

            <dt class="text-slate-400">Ingestion Mode</dt>
            <dd class="text-slate-200 capitalize">{{ source.ingestion_mode }}</dd>

            <dt class="text-slate-400">Render Mode</dt>
            <dd class="text-slate-200 capitalize">{{ source.render_mode }}</dd>

            <template v-if="source.feed_url">
              <dt class="text-slate-400">Feed URL</dt>
              <dd class="text-slate-200 truncate">{{ source.feed_url }}</dd>
            </template>

            <template v-if="source.sitemap_url">
              <dt class="text-slate-400">Sitemap URL</dt>
              <dd class="text-slate-200 truncate">{{ source.sitemap_url }}</dd>
            </template>

            <dt class="text-slate-400">Discovery</dt>
            <dd class="text-slate-200">{{ source.allow_source_discovery ? 'Yes' : 'No' }}</dd>

            <dt class="text-slate-400">Created</dt>
            <dd class="text-slate-200">{{ formatDateTime(source.created_at) }}</dd>

            <dt class="text-slate-400">Updated</dt>
            <dd class="text-slate-200">{{ formatDateTime(source.updated_at) }}</dd>
          </dl>
        </div>

        <!-- Grafana Panel -->
        <div class="bg-slate-900 border border-slate-800 rounded-lg p-6">
          <h2 class="text-lg font-semibold mb-4">Metrics</h2>
          <GrafanaEmbed panel-id="source-health" :vars="{ source: source.name }" height="280px" />
        </div>
      </div>

      <TestCrawlDialog
        v-if="showTestCrawl"
        :source="source"
        @close="showTestCrawl = false"
      />

      <ConfirmDialog
        :open="showDeleteDialog"
        title="Delete Source"
        :message="`Are you sure you want to delete '${source.name}'? This action cannot be undone.`"
        confirm-label="Delete"
        :danger="true"
        @confirm="handleDelete"
        @cancel="showDeleteDialog = false"
      />
    </template>
  </div>
</template>
