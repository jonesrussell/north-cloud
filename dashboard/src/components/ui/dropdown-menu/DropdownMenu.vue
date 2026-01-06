<script setup lang="ts">
import { ref, provide, readonly } from 'vue'

const isOpen = ref(false)

const toggle = () => {
  isOpen.value = !isOpen.value
}

const close = () => {
  isOpen.value = false
}

provide('dropdown-menu', {
  isOpen: readonly(isOpen),
  toggle,
  close,
})
</script>

<template>
  <div class="relative inline-block text-left">
    <slot :is-open="isOpen" :toggle="toggle" :close="close" />
  </div>
</template>
