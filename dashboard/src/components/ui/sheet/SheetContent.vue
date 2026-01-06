<script setup lang="ts">
import { inject, computed, onMounted, onUnmounted } from 'vue'
import { cn } from '@/lib/utils'

interface SheetContext {
  isOpen: { value: boolean }
  close: () => void
}

interface Props {
  side?: 'top' | 'right' | 'bottom' | 'left'
  class?: string
}

const props = withDefaults(defineProps<Props>(), {
  side: 'right',
  class: '',
})

const sheet = inject<SheetContext>('sheet')

const handleEscape = (event: KeyboardEvent) => {
  if (event.key === 'Escape') {
    sheet?.close()
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleEscape)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleEscape)
})

const overlayClass = computed(() =>
  cn('fixed inset-0 z-50 bg-black/80', sheet?.isOpen.value ? 'animate-in fade-in-0' : 'animate-out fade-out-0')
)

const contentClass = computed(() =>
  cn(
    'fixed z-50 gap-4 bg-background p-6 shadow-lg transition ease-in-out',
    {
      'inset-x-0 top-0 border-b': props.side === 'top',
      'inset-y-0 right-0 h-full w-3/4 border-l sm:max-w-sm': props.side === 'right',
      'inset-x-0 bottom-0 border-t': props.side === 'bottom',
      'inset-y-0 left-0 h-full w-3/4 border-r sm:max-w-sm': props.side === 'left',
    },
    sheet?.isOpen.value
      ? 'animate-in duration-300'
      : 'animate-out duration-300',
    sheet?.isOpen.value && props.side === 'right' && 'slide-in-from-right',
    sheet?.isOpen.value && props.side === 'left' && 'slide-in-from-left',
    sheet?.isOpen.value && props.side === 'top' && 'slide-in-from-top',
    sheet?.isOpen.value && props.side === 'bottom' && 'slide-in-from-bottom',
    props.class
  )
)
</script>

<template>
  <Teleport to="body">
    <Transition name="sheet">
      <template v-if="sheet?.isOpen.value">
        <!-- Overlay -->
        <div :class="overlayClass" @click="sheet?.close" />
      </template>
    </Transition>
    <Transition name="sheet-content">
      <template v-if="sheet?.isOpen.value">
        <div :class="contentClass">
          <slot />
        </div>
      </template>
    </Transition>
  </Teleport>
</template>

<style scoped>
.sheet-enter-active,
.sheet-leave-active {
  transition: opacity 0.3s ease;
}

.sheet-enter-from,
.sheet-leave-to {
  opacity: 0;
}

.sheet-content-enter-active,
.sheet-content-leave-active {
  transition: transform 0.3s ease;
}

.sheet-content-enter-from,
.sheet-content-leave-to {
  transform: translateX(100%);
}
</style>
