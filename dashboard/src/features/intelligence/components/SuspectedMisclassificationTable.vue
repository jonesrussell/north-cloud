<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { Loader2, ExternalLink } from 'lucide-vue-next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { indexManagerApi } from '@/api/client'
import type { SuspectedMisclassificationDoc } from '@/types/aggregation'

const documents = ref<SuspectedMisclassificationDoc[]>([])
const total = ref(0)
const loading = ref(true)
const hours = ref(24)

async function load(h: number = 24) {
  loading.value = true
  try {
    const res = await indexManagerApi.aggregations.getSuspectedMisclassifications({ hours: h })
    documents.value = res.data?.documents ?? []
    total.value = res.data?.total ?? 0
  } catch {
    documents.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

onMounted(() => load(hours.value))

const hasResults = computed(() => documents.value.length > 0)
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle class="text-base">
        Suspected Misclassifications (page + crime topic)
      </CardTitle>
      <p class="text-xs text-muted-foreground mt-1">
        Documents with content_type=page and topics containing crime or violent_crime (last {{ hours }}h). Total: {{ total }}.
      </p>
    </CardHeader>
    <CardContent>
      <div
        v-if="loading"
        class="flex items-center justify-center py-8"
      >
        <Loader2 class="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
      <template v-else>
        <div
          v-if="!hasResults"
          class="text-sm text-muted-foreground py-4"
        >
          No suspected misclassifications in the last {{ hours }}h.
        </div>
        <div
          v-else
          class="overflow-x-auto"
        >
          <table class="w-full text-sm">
            <thead>
              <tr class="border-b text-left text-muted-foreground">
                <th class="py-2 pr-2 font-medium">
                  Title
                </th>
                <th class="py-2 pr-2 font-medium">
                  URL
                </th>
                <th class="py-2 pr-2 font-medium">
                  content_type
                </th>
                <th class="py-2 pr-2 font-medium">
                  crime_relevance
                </th>
                <th class="py-2 pr-2 font-medium">
                  confidence
                </th>
                <th class="py-2 pr-2 font-medium">
                  crawled_at
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="doc in documents"
                :key="doc.id"
                class="border-b last:border-0"
              >
                <td
                  class="py-2 pr-2 max-w-[200px] truncate"
                  :title="doc.title"
                >
                  {{ doc.title || '—' }}
                </td>
                <td class="py-2 pr-2">
                  <a
                    v-if="doc.canonical_url"
                    :href="doc.canonical_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="text-primary hover:underline inline-flex items-center gap-0.5"
                  >
                    <span class="truncate max-w-[180px]">{{ doc.canonical_url }}</span>
                    <ExternalLink class="h-3 w-3 shrink-0" />
                  </a>
                  <span v-else>—</span>
                </td>
                <td class="py-2 pr-2">
                  {{ doc.content_type || '—' }}
                </td>
                <td class="py-2 pr-2">
                  {{ doc.crime_relevance || '—' }}
                </td>
                <td class="py-2 pr-2">
                  {{ doc.confidence != null ? doc.confidence.toFixed(2) : '—' }}
                </td>
                <td class="py-2 pr-2 text-muted-foreground">
                  {{ doc.crawled_at || '—' }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
    </CardContent>
  </Card>
</template>
