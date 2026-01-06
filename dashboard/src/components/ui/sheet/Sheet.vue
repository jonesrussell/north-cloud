<script setup lang="ts">
import { ref, provide, watch } from 'vue'

interface Props {
  open?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  open: false,
})

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
}>()

const isOpen = ref(props.open)

watch(
  () => props.open,
  (newVal) => {
    isOpen.value = newVal
  }
)

const close = () => {
  isOpen.value = false
  emit('update:open', false)
}

provide('sheet', {
  isOpen,
  close,
})
</script>

<template>
  <slot :is-open="isOpen" :close="close" />
</template>
