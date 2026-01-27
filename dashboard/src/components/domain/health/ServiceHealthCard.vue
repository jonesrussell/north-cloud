<script setup lang="ts">
import { computed } from 'vue'
import { CheckCircle2, XCircle, AlertTriangle, Loader2, HelpCircle } from 'lucide-vue-next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type { ServiceHealth, ServiceStatus } from '@/types/health'

const props = defineProps<{
  service: ServiceHealth
  compact?: boolean
}>()

type BadgeVariant = 'success' | 'warning' | 'destructive' | 'secondary' | 'default' | 'outline'

const statusConfig = computed(() => {
  const configs: Record<ServiceStatus, { icon: typeof CheckCircle2; color: string; variant: BadgeVariant }> = {
    healthy: { icon: CheckCircle2, color: 'text-green-500', variant: 'success' },
    degraded: { icon: AlertTriangle, color: 'text-yellow-500', variant: 'warning' },
    unhealthy: { icon: XCircle, color: 'text-red-500', variant: 'destructive' },
    checking: { icon: Loader2, color: 'text-muted-foreground', variant: 'secondary' },
    unknown: { icon: HelpCircle, color: 'text-muted-foreground', variant: 'secondary' },
  }
  return configs[props.service.status]
})

const StatusIcon = computed(() => statusConfig.value.icon)
const badgeVariant = computed(() => statusConfig.value.variant)

const formatLatency = (ms?: number) => {
  if (!ms) return '—'
  return `${ms}ms`
}

const formatLastCheck = (dateStr: string | null) => {
  if (!dateStr) return '—'
  return new Date(dateStr).toLocaleTimeString()
}
</script>

<template>
  <Card :class="{ 'opacity-50': service.status === 'checking' }">
    <CardHeader class="pb-2">
      <div class="flex items-center justify-between">
        <CardTitle class="text-base font-medium">
          {{ service.name }}
        </CardTitle>
        <component
          :is="StatusIcon"
          :class="[
            'h-5 w-5 transition-colors',
            statusConfig.color,
            service.status === 'checking' && 'animate-spin'
          ]"
        />
      </div>
    </CardHeader>
    <CardContent>
      <div class="flex items-center justify-between">
        <Badge :variant="badgeVariant">
          {{ service.status }}
        </Badge>
        <span class="text-sm text-muted-foreground">
          {{ formatLatency(service.latency) }}
        </span>
      </div>

      <p
        v-if="service.details && !compact"
        class="mt-2 text-xs text-destructive"
      >
        {{ service.details }}
      </p>

      <p
        v-if="!compact"
        class="mt-2 text-xs text-muted-foreground"
      >
        Last check: {{ formatLastCheck(service.lastCheck) }}
      </p>
    </CardContent>
  </Card>
</template>
