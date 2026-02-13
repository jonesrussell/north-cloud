<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loader2 } from 'lucide-vue-next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { indexManagerApi, publisherApi } from '@/api/client'
import type { SourceHealthResponse } from '@/types/aggregation'

interface FunnelStage {
  label: string
  value: number
  sub?: string
}

const stages = ref<FunnelStage[]>([
  { label: 'Crawled', value: 0 },
  { label: 'Classified as article', value: 0 },
  { label: 'Published to Redis', value: 0 },
  { label: 'Received by StreetCode', value: 0, sub: 'N/A' },
  { label: 'Passed core_street_crime gate', value: 0, sub: 'N/A' },
  { label: 'Persisted', value: 0, sub: 'N/A' },
])
const loading = ref(true)

onMounted(async () => {
  try {
    const [sourceHealthRes, driftRes, publishRes] = await Promise.allSettled([
      indexManagerApi.aggregations.getSourceHealth(),
      indexManagerApi.aggregations.getClassificationDrift({ hours: 24 }),
      publisherApi.stats.publishVolume({ hours: 24 }),
    ])

    let crawled = 0
    if (sourceHealthRes.status === 'fulfilled' && sourceHealthRes.value.data) {
      const d = sourceHealthRes.value.data as SourceHealthResponse
      crawled = (d.sources ?? []).reduce((sum, s) => sum + (s.raw_count ?? 0), 0)
    }

    let classifiedArticle = 0
    if (driftRes.status === 'fulfilled' && driftRes.value.data) {
      const d = driftRes.value.data
      classifiedArticle = d.by_content_type?.article ?? 0
    }

    let published = 0
    if (publishRes.status === 'fulfilled' && publishRes.value.data) {
      published = publishRes.value.data.messages_total ?? 0
    }

    stages.value = [
      { label: 'Crawled', value: crawled },
      { label: 'Classified as article', value: classifiedArticle },
      { label: 'Published to Redis', value: published },
      { label: 'Received by StreetCode', value: 0, sub: 'N/A' },
      { label: 'Passed core_street_crime gate', value: 0, sub: 'N/A' },
      { label: 'Persisted', value: 0, sub: 'N/A' },
    ]
  } catch {
    // keep defaults
  } finally {
    loading.value = false
  }
})

const maxVal = (): number => {
  const n = Math.max(...stages.value.map((s) => s.value), 1)
  return n
}
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle class="text-base">
        Publishability Funnel (24h)
      </CardTitle>
      <p class="text-xs text-muted-foreground mt-1">
        Where items drop out of the pipeline. Last three stages require StreetCode metrics.
      </p>
    </CardHeader>
    <CardContent>
      <div
        v-if="loading"
        class="flex items-center justify-center py-8"
      >
        <Loader2 class="h-5 w-5 animate-spin text-muted-foreground" />
      </div>
      <div
        v-else
        class="space-y-2"
      >
        <div
          v-for="stage in stages"
          :key="stage.label"
          class="flex items-center gap-3"
        >
          <span class="w-48 shrink-0 text-sm">{{ stage.label }}</span>
          <div class="flex-1 h-6 rounded overflow-hidden bg-muted">
            <div
              v-if="stage.sub === 'N/A'"
              class="h-full flex items-center justify-center text-xs text-muted-foreground"
            >
              N/A
            </div>
            <div
              v-else
              class="h-full bg-primary transition-all"
              :style="{ width: `${(stage.value / maxVal()) * 100}%`, minWidth: stage.value > 0 ? '2%' : '0' }"
            />
          </div>
          <span class="w-20 shrink-0 text-right text-sm tabular-nums">
            {{ stage.sub ?? stage.value.toLocaleString() }}
          </span>
        </div>
      </div>
    </CardContent>
  </Card>
</template>
