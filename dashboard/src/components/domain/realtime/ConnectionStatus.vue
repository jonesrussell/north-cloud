<script setup lang="ts">
import { computed } from 'vue'
import { Wifi, WifiOff, Loader2, AlertTriangle } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Tooltip } from '@/components/ui/tooltip'
import { useRealtimeStore } from '@/stores/realtime'
import type { ConnectionStatus } from '@/types/realtime'

interface Props {
  showLabel?: boolean
  compact?: boolean
}

withDefaults(defineProps<Props>(), {
  showLabel: true,
  compact: false,
})

const realtimeStore = useRealtimeStore()

type BadgeVariant = 'default' | 'secondary' | 'destructive' | 'outline' | 'success' | 'warning'

const statusConfig = computed<{
  icon: typeof Wifi
  label: string
  color: string
  variant: BadgeVariant
  animate: boolean
}>(() => {
  const configs: Record<ConnectionStatus, {
    icon: typeof Wifi
    label: string
    color: string
    variant: BadgeVariant
    animate: boolean
  }> = {
    connected: {
      icon: Wifi,
      label: 'Live',
      color: 'text-green-500',
      variant: 'success',
      animate: false,
    },
    connecting: {
      icon: Loader2,
      label: 'Connecting',
      color: 'text-yellow-500',
      variant: 'warning',
      animate: true,
    },
    disconnected: {
      icon: WifiOff,
      label: 'Offline',
      color: 'text-muted-foreground',
      variant: 'secondary',
      animate: false,
    },
    error: {
      icon: AlertTriangle,
      label: 'Error',
      color: 'text-red-500',
      variant: 'destructive',
      animate: false,
    },
  }

  return configs[realtimeStore.overallStatus]
})

const tooltipContent = computed(() => {
  const status = realtimeStore.overallStatus
  const action = status === 'connected' ? 'Click to disconnect' : 'Click to connect'
  return `Real-time: ${statusConfig.value.label}. ${action}`
})

function handleClick() {
  if (realtimeStore.overallStatus === 'connected') {
    realtimeStore.disconnectAll()
  } else {
    realtimeStore.connectAll()
  }
}
</script>

<template>
  <Tooltip :content="tooltipContent">
    <Button
      v-if="!compact"
      variant="ghost"
      size="sm"
      class="gap-2"
      @click="handleClick"
    >
      <component
        :is="statusConfig.icon"
        :class="[
          'h-4 w-4',
          statusConfig.color,
          statusConfig.animate && 'animate-spin',
        ]"
      />
      <span
        v-if="showLabel"
        class="text-xs"
      >
        {{ statusConfig.label }}
      </span>
    </Button>

    <Badge
      v-else
      :variant="statusConfig.variant"
      class="cursor-pointer gap-1"
      @click="handleClick"
    >
      <component
        :is="statusConfig.icon"
        :class="[
          'h-3 w-3',
          statusConfig.animate && 'animate-spin',
        ]"
      />
      <span
        v-if="showLabel"
        class="text-xs"
      >
        {{ statusConfig.label }}
      </span>
    </Badge>
  </Tooltip>
</template>
