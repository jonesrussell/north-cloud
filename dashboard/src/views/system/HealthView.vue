<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { Loader2, HeartPulse, RefreshCw, CheckCircle2, XCircle, AlertTriangle } from 'lucide-vue-next'
import { crawlerApi, publisherApi, classifierApi, indexManagerApi } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'

interface ServiceHealth {
  name: string
  status: 'healthy' | 'degraded' | 'unhealthy' | 'checking'
  latency?: number
  lastCheck?: string
  details?: string
}

const loading = ref(true)
const services = ref<ServiceHealth[]>([
  { name: 'Crawler', status: 'checking' },
  { name: 'Classifier', status: 'checking' },
  { name: 'Publisher', status: 'checking' },
  { name: 'Index Manager', status: 'checking' },
  { name: 'Elasticsearch', status: 'checking' },
  { name: 'Redis', status: 'checking' },
])

let refreshInterval: ReturnType<typeof setInterval> | null = null

const checkHealth = async () => {
  const checks: Array<{ name: string; fn: () => Promise<unknown> }> = [
    { name: 'Crawler', fn: () => crawlerApi.getHealth() },
    { name: 'Classifier', fn: () => classifierApi.getHealth() },
    { name: 'Publisher', fn: () => publisherApi.getHealth() },
    { name: 'Index Manager', fn: () => indexManagerApi.getHealth() },
  ]

  for (const check of checks) {
    const serviceIndex = services.value.findIndex((s) => s.name === check.name)
    if (serviceIndex === -1) continue

    const start = Date.now()
    try {
      await check.fn()
      services.value[serviceIndex] = {
        name: check.name,
        status: 'healthy',
        latency: Date.now() - start,
        lastCheck: new Date().toISOString(),
      }
    } catch (err) {
      services.value[serviceIndex] = {
        name: check.name,
        status: 'unhealthy',
        latency: Date.now() - start,
        lastCheck: new Date().toISOString(),
        details: 'Connection failed',
      }
    }
  }

  // Infer Elasticsearch and Redis health from other services
  const healthyServices = services.value.filter((s) => s.status === 'healthy').length
  services.value[4] = {
    name: 'Elasticsearch',
    status: healthyServices >= 2 ? 'healthy' : 'unhealthy',
    lastCheck: new Date().toISOString(),
  }
  services.value[5] = {
    name: 'Redis',
    status: services.value.find((s) => s.name === 'Publisher')?.status === 'healthy' ? 'healthy' : 'unhealthy',
    lastCheck: new Date().toISOString(),
  }

  loading.value = false
}

const getStatusIcon = (status: string) => {
  switch (status) {
    case 'healthy': return CheckCircle2
    case 'degraded': return AlertTriangle
    case 'unhealthy': return XCircle
    default: return Loader2
  }
}

const getStatusVariant = (status: string) => {
  switch (status) {
    case 'healthy': return 'success'
    case 'degraded': return 'warning'
    case 'unhealthy': return 'destructive'
    default: return 'secondary'
  }
}

const formatLatency = (ms?: number) => ms ? `${ms}ms` : '—'
const formatDate = (date?: string) => date ? new Date(date).toLocaleTimeString() : '—'

const overallHealth = computed(() => {
  const healthy = services.value.filter((s) => s.status === 'healthy').length
  const total = services.value.length
  if (healthy === total) return 'All Systems Operational'
  if (healthy >= total / 2) return 'Partial Outage'
  return 'Major Outage'
})

import { computed } from 'vue'

onMounted(() => {
  checkHealth()
  refreshInterval = setInterval(checkHealth, 30000)
})

onUnmounted(() => {
  if (refreshInterval) clearInterval(refreshInterval)
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-3xl font-bold tracking-tight">System Health</h1>
        <p class="text-muted-foreground">Monitor the status of all platform services</p>
      </div>
      <Button variant="outline" @click="checkHealth">
        <RefreshCw class="mr-2 h-4 w-4" />
        Refresh
      </Button>
    </div>

    <!-- Overall Status -->
    <Card>
      <CardContent class="py-6">
        <div class="flex items-center justify-center gap-4">
          <HeartPulse 
            :class="[
              'h-8 w-8',
              services.filter(s => s.status === 'healthy').length === services.length ? 'text-green-500' :
              services.filter(s => s.status === 'healthy').length >= services.length / 2 ? 'text-yellow-500' : 'text-red-500'
            ]" 
          />
          <span class="text-2xl font-bold">{{ overallHealth }}</span>
        </div>
      </CardContent>
    </Card>

    <!-- Service Cards -->
    <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      <Card v-for="service in services" :key="service.name">
        <CardHeader class="pb-2">
          <div class="flex items-center justify-between">
            <CardTitle class="text-base">{{ service.name }}</CardTitle>
            <component 
              :is="getStatusIcon(service.status)" 
              :class="[
                'h-5 w-5',
                service.status === 'healthy' ? 'text-green-500' :
                service.status === 'degraded' ? 'text-yellow-500' :
                service.status === 'unhealthy' ? 'text-red-500' : 'text-muted-foreground animate-spin'
              ]"
            />
          </div>
        </CardHeader>
        <CardContent>
          <div class="flex items-center justify-between">
            <Badge :variant="getStatusVariant(service.status)">{{ service.status }}</Badge>
            <span class="text-sm text-muted-foreground">{{ formatLatency(service.latency) }}</span>
          </div>
          <p v-if="service.details" class="mt-2 text-xs text-destructive">{{ service.details }}</p>
          <p class="mt-2 text-xs text-muted-foreground">Last check: {{ formatDate(service.lastCheck) }}</p>
        </CardContent>
      </Card>
    </div>
  </div>
</template>
