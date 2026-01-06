<script setup lang="ts">
import { inject } from 'vue'

interface SheetContext {
  isOpen: { value: boolean }
}

const emit = defineEmits<{
  (e: 'click'): void
}>()

inject<SheetContext>('sheet')

const handleClick = () => {
  emit('click')
}
</script>

<template>
  <div @click="handleClick">
    <slot />
  </div>
</template>
