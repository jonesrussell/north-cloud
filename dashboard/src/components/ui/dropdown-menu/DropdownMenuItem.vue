<script setup lang="ts">
import { inject, computed } from 'vue'
import { cn } from '@/lib/utils'

interface DropdownContext {
  close: () => void
}

interface Props {
  class?: string
  disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  class: '',
  disabled: false,
})

const emit = defineEmits<{
  (e: 'select'): void
}>()

const dropdown = inject<DropdownContext>('dropdown-menu')

const handleClick = () => {
  if (!props.disabled) {
    emit('select')
    dropdown?.close()
  }
}

const itemClass = computed(() =>
  cn(
    'relative flex cursor-pointer select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none transition-colors',
    'hover:bg-accent hover:text-accent-foreground focus:bg-accent focus:text-accent-foreground',
    props.disabled && 'pointer-events-none opacity-50',
    props.class
  )
)
</script>

<template>
  <div
    :class="itemClass"
    @click="handleClick"
  >
    <slot />
  </div>
</template>
