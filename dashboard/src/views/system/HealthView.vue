<script setup lang="ts">
import { onMounted, onUnmounted, computed } from 'vue'
import { HealthOverview } from '@/components/domain/health'
import { LiveUpdateIndicator } from '@/components/domain/realtime'
import { useHealthStore } from '@/stores/health'
import { useHealthRealtime } from '@/composables/useHealthRealtime'

const healthStore = useHealthStore()

// Use realtime composable - automatically handles polling vs SSE
const { isRealtime } = useHealthRealtime()

// Auto-refresh interval in milliseconds (30 seconds) - used when SSE is not available
const REFRESH_INTERVAL = 30000

const updateMode = computed(() => (isRealtime() ? 'Real-time' : 'Polling'))

onMounted(() => {
  // Start polling initially - realtime composable will stop it if SSE connects
  healthStore.startPolling(REFRESH_INTERVAL)
})

onUnmounted(() => {
  healthStore.stopPolling()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Page Header -->
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          System Health
        </h1>
        <p class="text-muted-foreground">
          Monitor the status of all platform services
        </p>
      </div>
      <div class="flex items-center gap-2 text-sm text-muted-foreground">
        <LiveUpdateIndicator :event-types="['health:status']" />
        <span>{{ updateMode }}</span>
      </div>
    </div>

    <!-- Health Overview Component -->
    <HealthOverview show-refresh />
  </div>
</template>
