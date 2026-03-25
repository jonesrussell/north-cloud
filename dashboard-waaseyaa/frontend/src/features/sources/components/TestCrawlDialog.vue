<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useTestCrawl } from '../composables/useSourceApi'
import type { Source } from '../types'

const props = defineProps<{
  source: Source
}>()

const emit = defineEmits<{
  close: []
}>()

const testCrawl = useTestCrawl()

onMounted(() => {
  testCrawl.mutate({
    url: props.source.url,
    selectors: props.source.selectors as unknown as Record<string, unknown>,
  })
})

const result = computed(() => testCrawl.data.value)
</script>

<template>
  <Teleport to="body">
    <div class="fixed inset-0 z-50 flex items-center justify-center">
      <div class="fixed inset-0 bg-black/60" @click="emit('close')" />
      <div class="relative bg-slate-900 border border-slate-700 rounded-lg p-6 w-[600px] max-h-[80vh] overflow-y-auto shadow-xl">
        <div class="flex items-center justify-between mb-4">
          <h3 class="text-lg font-semibold text-slate-100">Test Crawl Results</h3>
          <button class="text-slate-400 hover:text-slate-200" @click="emit('close')">
            &times;
          </button>
        </div>

        <!-- Loading -->
        <div v-if="testCrawl.isPending.value" class="py-8 text-center text-slate-400">
          <p>Running test crawl...</p>
        </div>

        <!-- Error -->
        <div v-else-if="testCrawl.isError.value" class="py-4">
          <div class="bg-red-900/30 border border-red-800 rounded p-3 text-sm text-red-300">
            Test crawl failed. The URL may be unreachable or selectors invalid.
          </div>
        </div>

        <!-- Results -->
        <template v-else-if="result">
          <div class="grid grid-cols-2 gap-4 mb-4">
            <div class="bg-slate-800 rounded p-3">
              <p class="text-xs text-slate-400">Articles Found</p>
              <p class="text-2xl font-bold text-slate-100">{{ result.articles_found }}</p>
            </div>
            <div class="bg-slate-800 rounded p-3">
              <p class="text-xs text-slate-400">Success Rate</p>
              <p class="text-2xl font-bold text-slate-100">{{ result.success_rate }}%</p>
            </div>
          </div>

          <!-- Warnings -->
          <div v-if="result.warnings.length > 0" class="mb-4">
            <h4 class="text-sm font-medium text-amber-400 mb-2">Warnings</h4>
            <ul class="space-y-1">
              <li
                v-for="(warning, idx) in result.warnings"
                :key="idx"
                class="text-xs text-amber-300 bg-amber-900/20 rounded px-2 py-1"
              >
                {{ warning }}
              </li>
            </ul>
          </div>

          <!-- Sample Articles -->
          <div v-if="result.sample_articles.length > 0">
            <h4 class="text-sm font-medium text-slate-300 mb-2">Sample Articles</h4>
            <div class="space-y-3">
              <div
                v-for="(article, idx) in result.sample_articles"
                :key="idx"
                class="bg-slate-800 rounded p-3"
              >
                <p class="text-sm font-medium text-slate-200">{{ article.title }}</p>
                <p class="text-xs text-slate-400 mt-1 line-clamp-2">{{ article.body }}</p>
                <div class="flex gap-4 mt-2 text-xs text-slate-500">
                  <span v-if="article.author">By {{ article.author }}</span>
                  <span v-if="article.published_date">{{ article.published_date }}</span>
                  <span>Quality: {{ article.quality_score }}%</span>
                </div>
              </div>
            </div>
          </div>
        </template>

        <div class="mt-6 flex justify-end">
          <button
            class="px-4 py-2 text-sm text-slate-300 border border-slate-600 rounded hover:bg-slate-800"
            @click="emit('close')"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
