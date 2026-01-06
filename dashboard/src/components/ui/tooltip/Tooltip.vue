<script setup lang="ts">
import { ref, computed } from 'vue'
import { cn } from '@/lib/utils'

interface Props {
  content: string
  side?: 'top' | 'right' | 'bottom' | 'left'
  class?: string
  delayDuration?: number
}

const props = withDefaults(defineProps<Props>(), {
  side: 'top',
  class: '',
  delayDuration: 200,
})

const isVisible = ref(false)
let timeoutId: ReturnType<typeof setTimeout> | null = null

const show = () => {
  timeoutId = setTimeout(() => {
    isVisible.value = true
  }, props.delayDuration)
}

const hide = () => {
  if (timeoutId) {
    clearTimeout(timeoutId)
    timeoutId = null
  }
  isVisible.value = false
}

const tooltipClass = computed(() =>
  cn(
    'absolute z-50 overflow-hidden rounded-md border bg-popover px-3 py-1.5 text-sm text-popover-foreground shadow-md animate-in fade-in-0 zoom-in-95',
    {
      'bottom-full left-1/2 -translate-x-1/2 mb-2': props.side === 'top',
      'top-1/2 left-full -translate-y-1/2 ml-2': props.side === 'right',
      'top-full left-1/2 -translate-x-1/2 mt-2': props.side === 'bottom',
      'top-1/2 right-full -translate-y-1/2 mr-2': props.side === 'left',
    },
    props.class
  )
)
</script>

<template>
  <div class="relative inline-block" @mouseenter="show" @mouseleave="hide" @focus="show" @blur="hide">
    <slot />
    <div v-if="isVisible" :class="tooltipClass">
      {{ content }}
    </div>
  </div>
</template>
