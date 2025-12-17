<template>
  <div class="flex items-center justify-center" :class="containerClass">
    <div
      class="inline-block animate-spin rounded-full border-b-2 border-blue-600"
      :class="sizeClass"
    ></div>
    <span v-if="text" class="ml-3 text-gray-600">{{ text }}</span>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  size: {
    type: String,
    default: 'md',
    validator: (value) => ['sm', 'md', 'lg'].includes(value),
  },
  text: {
    type: String,
    default: '',
  },
  fullPage: {
    type: Boolean,
    default: false,
  },
})

const sizeClass = computed(() => {
  const sizes = {
    sm: 'h-4 w-4',
    md: 'h-8 w-8',
    lg: 'h-12 w-12',
  }
  return sizes[props.size]
})

const containerClass = computed(() => {
  return props.fullPage ? 'min-h-64' : ''
})
</script>
