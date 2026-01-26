<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { HealthOverview } from '@/components/domain/health'
import { useHealthStore } from '@/stores/health'

const healthStore = useHealthStore()

// Auto-refresh interval in milliseconds (30 seconds)
const REFRESH_INTERVAL = 30000

onMounted(() => {
  healthStore.startPolling(REFRESH_INTERVAL)
})

onUnmounted(() => {
  healthStore.stopPolling()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Page Header -->
    <div>
      <h1 class="text-3xl font-bold tracking-tight">
        System Health
      </h1>
      <p class="text-muted-foreground">
        Monitor the status of all platform services
      </p>
    </div>

    <!-- Health Overview Component -->
    <HealthOverview show-refresh />
  </div>
</template>
