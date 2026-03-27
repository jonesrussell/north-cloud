<script setup lang="ts">
defineProps<{
  open: boolean
  title: string
  message: string
  confirmLabel?: string
  danger?: boolean
}>()

const emit = defineEmits<{
  confirm: []
  cancel: []
}>()
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-50 flex items-center justify-center">
      <div class="fixed inset-0 bg-black/60" @click="emit('cancel')" />
      <div class="relative bg-slate-900 border border-slate-700 rounded-lg p-6 w-96 shadow-xl">
        <h3 class="text-lg font-semibold text-slate-100 mb-2">{{ title }}</h3>
        <p class="text-slate-400 text-sm mb-6">{{ message }}</p>
        <div class="flex justify-end gap-3">
          <button
            @click="emit('cancel')"
            class="px-4 py-2 text-sm text-slate-300 border border-slate-600 rounded hover:bg-slate-800"
          >
            Cancel
          </button>
          <button
            @click="emit('confirm')"
            class="px-4 py-2 text-sm text-white rounded"
            :class="danger ? 'bg-red-600 hover:bg-red-500' : 'bg-blue-600 hover:bg-blue-500'"
          >
            {{ confirmLabel || 'Confirm' }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
