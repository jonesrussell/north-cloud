<script setup lang="ts">
defineProps<{
  count: number
}>()

const emit = defineEmits<{
  clear: []
}>()
</script>

<template>
  <div
    v-if="count > 0"
    class="fixed bottom-0 left-56 right-0 bg-slate-900 border-t border-slate-700 p-4 flex items-center gap-4 z-40"
  >
    <span class="text-sm text-slate-300">{{ count }} selected</span>
    <slot />
    <button
      @click="emit('clear')"
      class="ml-auto text-sm text-slate-400 hover:text-slate-200"
    >
      Clear selection
    </button>
  </div>
</template>
