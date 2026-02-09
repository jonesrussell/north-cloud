<script setup lang="ts">
import { computed } from 'vue'
import { formatTime } from '@/lib/utils'
import { HeartPulse, RefreshCw, Wifi, WifiOff } from 'lucide-vue-next'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import ServiceHealthCard from './ServiceHealthCard.vue'
import { useHealthStore } from '@/stores/health'
import type { OverallStatus } from '@/types/health'

withDefaults(
  defineProps<{
    /** Show refresh button */
    showRefresh?: boolean
    /** Use compact layout */
    compact?: boolean
  }>(),
  {
    showRefresh: true,
    compact: false,
  }
)

const healthStore = useHealthStore()

const overallStatusConfig = computed(() => {
  const configs: Record<OverallStatus, { label: string; color: string; bgColor: string }> = {
    operational: {
      label: 'All Systems Operational',
      color: 'text-green-500',
      bgColor: 'bg-green-500/10',
    },
    degraded: {
      label: 'Partial Outage',
      color: 'text-yellow-500',
      bgColor: 'bg-yellow-500/10',
    },
    outage: {
      label: 'Major Outage',
      color: 'text-red-500',
      bgColor: 'bg-red-500/10',
    },
  }
  return configs[healthStore.overallStatus]
})

const formatLastUpdate = computed(() => {
  if (!healthStore.lastUpdate) return 'Never'
  return formatTime(healthStore.lastUpdate.toISOString())
})

async function handleRefresh() {
  await healthStore.checkAllServices()
}
</script>

<template>
  <div class="space-y-4">
    <!-- Overall Status Banner -->
    <Card :class="overallStatusConfig.bgColor">
      <CardContent class="py-4">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <HeartPulse :class="['h-6 w-6', overallStatusConfig.color]" />
            <div>
              <span :class="['text-lg font-semibold', overallStatusConfig.color]">
                {{ overallStatusConfig.label }}
              </span>
              <p class="text-sm text-muted-foreground">
                {{ healthStore.healthyCount }}/{{ healthStore.services.length }} services healthy
              </p>
            </div>
          </div>

          <div class="flex items-center gap-3">
            <!-- Polling Status -->
            <div class="flex items-center gap-2 text-sm text-muted-foreground">
              <component
                :is="healthStore.isPolling ? Wifi : WifiOff"
                class="h-4 w-4"
              />
              <span v-if="healthStore.isPolling">
                Auto-refresh
              </span>
              <span v-else>
                Manual
              </span>
            </div>

            <!-- Last Update -->
            <Badge variant="outline">
              Last: {{ formatLastUpdate }}
            </Badge>

            <!-- Refresh Button -->
            <Button
              v-if="showRefresh"
              variant="outline"
              size="sm"
              @click="handleRefresh"
            >
              <RefreshCw class="mr-2 h-4 w-4" />
              Refresh
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>

    <!-- Error Alert -->
    <Card
      v-if="healthStore.error"
      class="border-destructive bg-destructive/5"
    >
      <CardContent class="py-3">
        <p class="text-sm text-destructive">
          {{ healthStore.error }}
        </p>
      </CardContent>
    </Card>

    <!-- Service Cards Grid -->
    <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      <ServiceHealthCard
        v-for="service in healthStore.services"
        :key="service.name"
        :service="service"
        :compact="compact"
      />
    </div>
  </div>
</template>
