<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loader2 } from 'lucide-vue-next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { indexManagerApi } from '@/api/client'
import type { ClassificationDriftTimeseriesBucket } from '@/types/aggregation'

const buckets = ref<ClassificationDriftTimeseriesBucket[]>([])
const loading = ref(true)
const days = 7

onMounted(async () => {
  try {
    const res = await indexManagerApi.aggregations.getClassificationDriftTimeseries({ days })
    buckets.value = res.data?.buckets ?? []
  } catch {
    buckets.value = []
  } finally {
    loading.value = false
  }
})

function pct(total: number, part: number): number {
  if (total <= 0) return 0
  return Math.round((part / total) * 100)
}
</script>

<template>
  <Card>
    <CardHeader>
      <CardTitle class="text-base">
        Content Type Drift (Last {{ days }} Days)
      </CardTitle>
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
          v-if="buckets.length === 0"
          class="text-sm text-muted-foreground py-4"
        >
          No data in the last {{ days }} days.
        </div>
        <div
          v-else
          class="space-y-3"
        >
          <div class="grid grid-cols-4 gap-2 text-xs font-medium text-muted-foreground border-b pb-1">
            <span>Date</span>
            <span class="text-right">Article %</span>
            <span class="text-right">Page %</span>
            <span class="text-right">Total</span>
          </div>
          <div
            v-for="b in buckets"
            :key="b.date"
            class="grid grid-cols-4 gap-2 text-sm items-center"
          >
            <span>{{ b.date }}</span>
            <span class="text-right">{{ pct(b.total, b.article_count) }}%</span>
            <span class="text-right">{{ pct(b.total, b.page_count) }}%</span>
            <span class="text-right">{{ b.total.toLocaleString() }}</span>
            <div class="col-span-4 h-2 flex rounded overflow-hidden bg-muted">
              <div
                class="bg-primary"
                :style="{ width: `${pct(b.total, b.article_count)}%` }"
              />
              <div
                class="bg-amber-500/80"
                :style="{ width: `${pct(b.total, b.page_count)}%` }"
              />
              <div
                class="bg-muted-foreground/30"
                :style="{ width: `${pct(b.total, b.other_count)}%` }"
              />
            </div>
          </div>
        </div>
      </template>
    </CardContent>
  </Card>
</template>
