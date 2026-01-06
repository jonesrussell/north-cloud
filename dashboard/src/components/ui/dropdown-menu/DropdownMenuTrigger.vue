<script setup lang="ts">
import { inject } from 'vue'

interface DropdownContext {
  isOpen: { value: boolean }
  toggle: () => void
}

const dropdown = inject<DropdownContext>('dropdown-menu')
</script>

<template>
  <div @click="dropdown?.toggle">
    <slot />
  </div>
</template>
