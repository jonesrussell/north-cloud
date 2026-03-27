<script setup lang="ts">
import { ref } from 'vue'
import { useStartCrawl } from '../composables/useCrawlApi'
import { useToast } from '@/shared/composables/useToast'

const emit = defineEmits<{
  close: []
}>()

const sourceUrl = ref('')
const sourceId = ref('')
const sourceName = ref('')
const scheduleEnabled = ref(false)
const intervalMinutes = ref(360)

const { mutate: startCrawl, isPending } = useStartCrawl()
const toast = useToast()

function handleSubmit() {
  if (!sourceUrl.value || !sourceId.value) return

  startCrawl(
    {
      source_id: sourceId.value,
      url: sourceUrl.value,
      source_name: sourceName.value || undefined,
      schedule_enabled: scheduleEnabled.value,
      interval_minutes: scheduleEnabled.value ? intervalMinutes.value : undefined,
      interval_type: scheduleEnabled.value ? 'minutes' : undefined,
    },
    {
      onSuccess: () => {
        toast.success('Crawl job started successfully')
        emit('close')
      },
      onError: (error) => {
        toast.error(`Failed to start crawl: ${error.message}`)
      },
    },
  )
}
</script>

<template>
  <div class="fixed inset-0 bg-black/50 flex items-center justify-center z-50" @click.self="emit('close')">
    <div class="bg-slate-900 border border-slate-700 rounded-lg p-6 w-full max-w-md">
      <h2 class="text-lg font-semibold text-slate-100 mb-4">Start New Crawl</h2>

      <form @submit.prevent="handleSubmit" class="space-y-4">
        <div>
          <label for="sourceId" class="block text-sm font-medium text-slate-300 mb-1">Source ID</label>
          <input
            id="sourceId"
            v-model="sourceId"
            type="text"
            required
            placeholder="UUID from source-manager"
            class="w-full bg-slate-800 border border-slate-600 rounded px-3 py-2 text-slate-200 placeholder-slate-500 focus:outline-none focus:border-blue-500"
          />
        </div>

        <div>
          <label for="sourceName" class="block text-sm font-medium text-slate-300 mb-1">Source Name</label>
          <input
            id="sourceName"
            v-model="sourceName"
            type="text"
            placeholder="Optional display name"
            class="w-full bg-slate-800 border border-slate-600 rounded px-3 py-2 text-slate-200 placeholder-slate-500 focus:outline-none focus:border-blue-500"
          />
        </div>

        <div>
          <label for="sourceUrl" class="block text-sm font-medium text-slate-300 mb-1">URL</label>
          <input
            id="sourceUrl"
            v-model="sourceUrl"
            type="url"
            required
            placeholder="https://example.com"
            class="w-full bg-slate-800 border border-slate-600 rounded px-3 py-2 text-slate-200 placeholder-slate-500 focus:outline-none focus:border-blue-500"
          />
        </div>

        <div class="flex items-center gap-2">
          <input
            id="scheduleEnabled"
            v-model="scheduleEnabled"
            type="checkbox"
            class="rounded border-slate-600 bg-slate-800 text-blue-500 focus:ring-blue-500"
          />
          <label for="scheduleEnabled" class="text-sm text-slate-300">Enable recurring schedule</label>
        </div>

        <div v-if="scheduleEnabled">
          <label for="intervalMinutes" class="block text-sm font-medium text-slate-300 mb-1">
            Interval (minutes)
          </label>
          <input
            id="intervalMinutes"
            v-model.number="intervalMinutes"
            type="number"
            min="1"
            class="w-full bg-slate-800 border border-slate-600 rounded px-3 py-2 text-slate-200 focus:outline-none focus:border-blue-500"
          />
        </div>

        <div class="flex justify-end gap-3 pt-2">
          <button
            type="button"
            @click="emit('close')"
            class="px-4 py-2 text-sm text-slate-300 hover:text-slate-100 border border-slate-600 rounded hover:border-slate-500"
          >
            Cancel
          </button>
          <button
            type="submit"
            :disabled="isPending || !sourceUrl || !sourceId"
            class="px-4 py-2 text-sm font-medium text-white bg-blue-600 rounded hover:bg-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {{ isPending ? 'Starting...' : 'Start Crawl' }}
          </button>
        </div>
      </form>
    </div>
  </div>
</template>
