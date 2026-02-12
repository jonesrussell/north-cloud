<script setup lang="ts">
import { computed } from 'vue'
import type { PipelineMetrics } from '../problems/types'

const props = defineProps<{
  metrics: PipelineMetrics
}>()

const crawled24h = computed(() => {
  if (!props.metrics.indexes) return 0
  return props.metrics.indexes.sources.reduce((sum, s) => sum + s.delta24h, 0)
})

const published24h = computed(() => props.metrics.publisher?.publishedToday ?? 0)
const failedJobs = computed(() => props.metrics.crawler?.failedJobs ?? 0)

const emptyIndexes = computed(() => {
  if (!props.metrics.indexes) return 0
  return props.metrics.indexes.sources.filter((s) => s.active && s.classifiedCount === 0).length
})

const pipelineYield = computed(() => {
  const crawled = crawled24h.value
  if (crawled === 0) return null
  return Math.round((published24h.value / crawled) * 100)
})

interface KPI {
  label: string
  value: string
  highlight: 'normal' | 'red' | 'amber'
  visible: boolean
}

const kpis = computed<KPI[]>(() => [
  {
    label: 'Classified (24h)',
    value: crawled24h.value.toLocaleString(),
    highlight: crawled24h.value === 0 ? 'red' : 'normal',
    visible: true,
  },
  {
    label: 'Published (24h)',
    value: published24h.value.toLocaleString(),
    highlight: published24h.value === 0 ? 'red' : 'normal',
    visible: true,
  },
  {
    label: 'Failed Jobs',
    value: failedJobs.value.toLocaleString(),
    highlight: 'red',
    visible: failedJobs.value > 0,
  },
  {
    label: 'Empty Indexes',
    value: emptyIndexes.value.toLocaleString(),
    highlight: 'amber',
    visible: emptyIndexes.value > 0,
  },
  {
    label: 'Pipeline Yield',
    value: pipelineYield.value !== null ? `${pipelineYield.value}%` : '-',
    highlight: pipelineYield.value !== null && pipelineYield.value < 10 ? 'amber' : 'normal',
    visible: pipelineYield.value !== null,
  },
])

const visibleKpis = computed(() => kpis.value.filter((k) => k.visible))
</script>

<template>
  <div class="grid gap-3 grid-cols-2 sm:grid-cols-3 lg:grid-cols-5">
    <div
      v-for="kpi in visibleKpis"
      :key="kpi.label"
      class="rounded-lg border bg-card px-4 py-3"
    >
      <p class="text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
        {{ kpi.label }}
      </p>
      <p
        class="text-xl font-semibold tabular-nums mt-0.5"
        :class="{
          'text-red-500': kpi.highlight === 'red',
          'text-amber-500': kpi.highlight === 'amber',
        }"
      >
        {{ kpi.value }}
      </p>
    </div>
  </div>
</template>
