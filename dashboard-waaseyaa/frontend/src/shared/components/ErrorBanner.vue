<script setup lang="ts">
defineProps<{
  message: string
}>()

const emit = defineEmits<{
  retry: []
}>()
</script>

<template>
  <div class="bg-red-900/30 border border-red-800 rounded-lg p-4 flex items-center justify-between">
    <p class="text-red-300 text-sm">{{ message }}</p>
    <button
      @click="emit('retry')"
      class="text-red-400 hover:text-red-300 text-sm font-medium px-3 py-1 rounded border border-red-700 hover:border-red-600"
    >
      Retry
    </button>
  </div>
</template>
