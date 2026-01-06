<script setup lang="ts">
import { inject, onMounted, onUnmounted, computed } from 'vue'
import { cn } from '@/lib/utils'

interface DropdownContext {
  isOpen: { value: boolean }
  close: () => void
}

interface Props {
  align?: 'start' | 'center' | 'end'
  class?: string
}

const props = withDefaults(defineProps<Props>(), {
  align: 'end',
  class: '',
})

const dropdown = inject<DropdownContext>('dropdown-menu')

const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (!target.closest('[data-dropdown-menu]')) {
    dropdown?.close()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})

const contentClass = computed(() =>
  cn(
    'absolute z-50 min-w-[8rem] overflow-hidden rounded-md border bg-popover p-1 text-popover-foreground shadow-md',
    'animate-in fade-in-0 zoom-in-95',
    {
      'right-0': props.align === 'end',
      'left-0': props.align === 'start',
      'left-1/2 -translate-x-1/2': props.align === 'center',
    },
    props.class
  )
)
</script>

<template>
  <Transition
    enter-active-class="transition ease-out duration-100"
    enter-from-class="transform opacity-0 scale-95"
    enter-to-class="transform opacity-100 scale-100"
    leave-active-class="transition ease-in duration-75"
    leave-from-class="transform opacity-100 scale-100"
    leave-to-class="transform opacity-0 scale-95"
  >
    <div
      v-if="dropdown?.isOpen.value"
      :class="contentClass"
      data-dropdown-menu
    >
      <slot />
    </div>
  </Transition>
</template>
