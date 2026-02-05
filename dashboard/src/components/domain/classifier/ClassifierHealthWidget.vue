<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { Activity, CircleCheck, CircleX } from 'lucide-vue-next'
import { classifierApi } from '@/api/client'
import type { MLHealthResponse } from '@/types/aggregation'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

const healthPollIntervalMs = 30000

const health = ref<MLHealthResponse | null>(null)
const loading = ref(true)
let pollTimer: ReturnType<typeof setInterval> | null = null

const loadHealth = async () => {
  try {
    const response = await classifierApi.metrics.mlHealth()
    health.value = response.data
  } catch (err) {
    console.error('Failed to load ML health:', err)
  } finally {
    loading.value = false
  }
}

const getPipelineModeVariant = (mode: string): 'default' | 'secondary' | 'outline' => {
  if (mode === 'hybrid') return 'default'
  if (mode === 'rules-only') return 'secondary'
  return 'outline'
}

onMounted(() => {
  loadHealth()
  pollTimer = setInterval(loadHealth, healthPollIntervalMs)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<template>
  <Card v-if="!loading && health">
    <CardHeader class="pb-3">
      <CardTitle class="flex items-center gap-2 text-sm font-medium">
        <Activity class="h-4 w-4" />
        Classifier Health
      </CardTitle>
    </CardHeader>
    <CardContent class="space-y-3">
      <!-- Pipeline Mode -->
      <div class="flex items-center gap-2 flex-wrap">
        <span class="text-xs text-muted-foreground">Pipeline:</span>
        <Badge
          :variant="getPipelineModeVariant(health.pipeline_mode.crime)"
          class="text-xs"
        >
          Crime {{ health.pipeline_mode.crime }}
        </Badge>
        <Badge
          :variant="getPipelineModeVariant(health.pipeline_mode.mining)"
          class="text-xs"
        >
          Mining {{ health.pipeline_mode.mining }}
        </Badge>
      </div>

      <!-- Crime ML -->
      <div
        v-if="health.crime_ml"
        class="flex items-center justify-between text-xs"
      >
        <div class="flex items-center gap-1.5">
          <CircleCheck
            v-if="health.crime_ml.reachable"
            class="h-3.5 w-3.5 text-green-500"
          />
          <CircleX
            v-else
            class="h-3.5 w-3.5 text-red-500"
          />
          <span>crime-ml</span>
        </div>
        <div class="flex items-center gap-2 text-muted-foreground">
          <span v-if="health.crime_ml.model_version">{{ health.crime_ml.model_version }}</span>
          <span v-if="health.crime_ml.latency_ms">{{ health.crime_ml.latency_ms }}ms</span>
        </div>
      </div>

      <!-- Mining ML -->
      <div
        v-if="health.mining_ml"
        class="flex items-center justify-between text-xs"
      >
        <div class="flex items-center gap-1.5">
          <CircleCheck
            v-if="health.mining_ml.reachable"
            class="h-3.5 w-3.5 text-green-500"
          />
          <CircleX
            v-else
            class="h-3.5 w-3.5 text-red-500"
          />
          <span>mining-ml</span>
        </div>
        <div class="flex items-center gap-2 text-muted-foreground">
          <span v-if="health.mining_ml.model_version">{{ health.mining_ml.model_version }}</span>
          <span v-if="health.mining_ml.latency_ms">{{ health.mining_ml.latency_ms }}ms</span>
        </div>
      </div>
    </CardContent>
  </Card>
</template>
