<script setup lang="ts">
import { computed } from 'vue'

interface QualityDistribution {
  high?: number
  medium?: number
  low?: number
}

interface Props {
  totalDocuments: number
  qualityDistribution: QualityDistribution
  mode: 'compact' | 'full'
}

const props = defineProps<Props>()

const normalized = computed(() => ({
  high: Math.max(0, Number(props.qualityDistribution?.high) || 0),
  medium: Math.max(0, Number(props.qualityDistribution?.medium) || 0),
  low: Math.max(0, Number(props.qualityDistribution?.low) || 0),
}))

const qualityTotal = computed(() => {
  const { high, medium, low } = normalized.value
  return high + medium + low
})

const barWidths = computed(() => {
  const total = qualityTotal.value
  if (total === 0) return { high: 0, medium: 0, low: 0 }
  return {
    high: (normalized.value.high / total) * 100,
    medium: (normalized.value.medium / total) * 100,
    low: (normalized.value.low / total) * 100,
  }
})

const formattedTotal = computed(() => props.totalDocuments.toLocaleString())
</script>

<template>
  <div class="space-y-2">
    <!-- Total: same structure in both modes, different density -->
    <div :class="mode === 'compact' ? 'space-y-0.5' : 'space-y-1'">
      <p
        v-if="mode === 'full'"
        class="text-[10px] font-mono uppercase tracking-widest text-muted-foreground"
      >
        Total Documents
      </p>
      <p
        :class="
          mode === 'compact'
            ? 'text-lg font-semibold font-mono tracking-tight'
            : 'text-2xl font-semibold font-mono tracking-tight'
        "
      >
        {{ formattedTotal }}
      </p>
    </div>

    <!-- Single quality bar: same structure, different height -->
    <div class="space-y-1">
      <p
        v-if="mode === 'full'"
        class="text-[10px] font-mono uppercase tracking-widest text-muted-foreground"
      >
        Quality Distribution
      </p>
      <div
        :class="[
          'flex rounded-sm overflow-hidden bg-muted',
          mode === 'compact' ? 'h-1.5' : 'h-2',
        ]"
      >
        <div
          class="bg-green-500 shrink-0 transition-[width]"
          :style="{ width: `${barWidths.high}%` }"
          :title="`High: ${normalized.high}`"
        />
        <div
          class="bg-amber-500 shrink-0 transition-[width]"
          :style="{ width: `${barWidths.medium}%` }"
          :title="`Medium: ${normalized.medium}`"
        />
        <div
          class="bg-red-500 shrink-0 transition-[width]"
          :style="{ width: `${barWidths.low}%` }"
          :title="`Low: ${normalized.low}`"
        />
      </div>
      <div
        v-if="mode === 'full'"
        class="flex justify-between text-[10px] font-mono text-muted-foreground"
      >
        <span>High ({{ normalized.high }})</span>
        <span>Medium ({{ normalized.medium }})</span>
        <span>Low ({{ normalized.low }})</span>
      </div>
    </div>
  </div>
</template>
