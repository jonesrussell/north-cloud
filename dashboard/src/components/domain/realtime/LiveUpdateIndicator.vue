<script setup lang="ts">
import { ref, watch, onUnmounted } from 'vue'
import { useRealtimeStore } from '@/stores/realtime'

interface Props {
  /** Event types to watch for (e.g., 'job:status', 'health:status') */
  eventTypes?: string[]
  /** Duration to show the pulse animation in ms (default: 2000) */
  pulseDuration?: number
  /** Size of the indicator */
  size?: 'sm' | 'md' | 'lg'
}

const props = withDefaults(defineProps<Props>(), {
  eventTypes: () => ['*'],
  pulseDuration: 2000,
  size: 'sm',
})

const realtimeStore = useRealtimeStore()
const isPulsing = ref(false)
let pulseTimer: ReturnType<typeof setTimeout> | null = null
let unsubscribes: Array<() => void> = []

const sizeClasses = {
  sm: 'h-2 w-2',
  md: 'h-3 w-3',
  lg: 'h-4 w-4',
}

function triggerPulse() {
  isPulsing.value = true

  if (pulseTimer) {
    clearTimeout(pulseTimer)
  }

  pulseTimer = setTimeout(() => {
    isPulsing.value = false
    pulseTimer = null
  }, props.pulseDuration)
}

// Subscribe to events
watch(
  () => realtimeStore.enabled,
  (enabled) => {
    // Clean up existing subscriptions
    for (const unsub of unsubscribes) {
      unsub()
    }
    unsubscribes = []

    if (enabled) {
      for (const eventType of props.eventTypes) {
        const unsub = realtimeStore.subscribe(eventType, () => {
          triggerPulse()
        })
        unsubscribes.push(unsub)
      }
    }
  },
  { immediate: true }
)

onUnmounted(() => {
  if (pulseTimer) {
    clearTimeout(pulseTimer)
  }
  for (const unsub of unsubscribes) {
    unsub()
  }
})
</script>

<template>
  <span class="relative inline-flex">
    <!-- Base dot -->
    <span
      :class="[
        'rounded-full',
        sizeClasses[size],
        realtimeStore.isConnected ? 'bg-green-500' : 'bg-gray-400',
      ]"
    />

    <!-- Pulse ring -->
    <span
      v-if="isPulsing && realtimeStore.isConnected"
      :class="[
        'absolute inline-flex rounded-full bg-green-400 opacity-75 animate-ping',
        sizeClasses[size],
      ]"
    />
  </span>
</template>
