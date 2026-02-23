<script setup lang="ts">
import { computed } from 'vue'
import { formatTime } from '@/lib/utils'
import { CheckCircle2, XCircle, AlertTriangle, Loader2, HelpCircle, Clock, Tag } from 'lucide-vue-next'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type { ServiceHealth, ServiceStatus, HealthCheckDetail } from '@/types/health'

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

const formatLastCheck = (dateStr: string | null): string => {
  if (!dateStr) return '—'
  return formatTime(dateStr)
}

const checkStatusColor = (check: HealthCheckDetail): string => {
  if (check.status === 'healthy') return 'text-green-500'
  if (check.status === 'degraded') return 'text-yellow-500'
  return 'text-red-500'
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

      <!-- Uptime & Version -->
      <div
        v-if="!compact && (service.uptime || service.version)"
        class="mt-2 flex items-center gap-3 text-xs text-muted-foreground"
      >
        <span
          v-if="service.uptime"
          class="flex items-center gap-1"
        >
          <Clock class="h-3 w-3" />
          {{ service.uptime }}
        </span>
        <span
          v-if="service.version"
          class="flex items-center gap-1"
        >
          <Tag class="h-3 w-3" />
          {{ service.version }}
        </span>
      </div>

      <!-- Dependency Checks -->
      <div
        v-if="!compact && service.checks && Object.keys(service.checks).length > 0"
        class="mt-2 space-y-1"
      >
        <div
          v-for="(check, name) in service.checks"
          :key="name"
          class="flex items-center justify-between text-xs"
        >
          <span class="text-muted-foreground capitalize">{{ name }}</span>
          <span :class="checkStatusColor(check)">
            {{ check.status }}
            <span
              v-if="check.latency"
              class="text-muted-foreground ml-1"
            >
              {{ check.latency }}
            </span>
          </span>
        </div>
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
