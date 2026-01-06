<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

interface Props {
  name: string
  status: 'healthy' | 'degraded' | 'unhealthy' | 'unknown'
  compact?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  compact: false,
})

const statusConfig = computed(() => {
  switch (props.status) {
    case 'healthy':
      return {
        label: 'OK',
        dotClass: 'bg-green-500',
        textClass: 'text-green-600 dark:text-green-400',
      }
    case 'degraded':
      return {
        label: 'Degraded',
        dotClass: 'bg-yellow-500',
        textClass: 'text-yellow-600 dark:text-yellow-400',
      }
    case 'unhealthy':
      return {
        label: 'Down',
        dotClass: 'bg-red-500',
        textClass: 'text-red-600 dark:text-red-400',
      }
    default:
      return {
        label: 'Unknown',
        dotClass: 'bg-gray-400',
        textClass: 'text-gray-500',
      }
  }
})
</script>

<template>
  <div
    :class="
      cn(
        'inline-flex items-center gap-2 rounded-md border px-3 py-1.5',
        compact ? 'text-xs' : 'text-sm'
      )
    "
  >
    <span :class="cn('h-2 w-2 rounded-full', statusConfig.dotClass)" />
    <span class="font-medium text-foreground">{{ name }}</span>
    <span :class="cn('font-medium', statusConfig.textClass)">
      {{ statusConfig.label }}
    </span>
  </div>
</template>
