<template>
  <div
    v-if="selectedCount > 0"
    class="fixed bottom-6 left-1/2 transform -translate-x-1/2 z-40 bg-white shadow-lg rounded-lg border border-gray-200 px-6 py-4"
  >
    <div class="flex items-center gap-6">
      <!-- Selection Count -->
      <div class="text-sm font-medium text-gray-700">
        {{ selectedCount }} {{ selectedCount === 1 ? 'item' : 'items' }} selected
      </div>

      <!-- Divider -->
      <div class="h-6 w-px bg-gray-300" />

      <!-- Action Buttons -->
      <div class="flex items-center gap-3">
        <button
          v-for="action in availableActions"
          :key="action.id"
          :class="[
            'inline-flex items-center px-4 py-2 rounded-md text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-offset-2',
            action.variant === 'primary'
              ? 'bg-blue-600 text-white hover:bg-blue-700 focus:ring-blue-500'
              : action.variant === 'danger'
              ? 'bg-red-600 text-white hover:bg-red-700 focus:ring-red-500'
              : action.variant === 'success'
              ? 'bg-green-600 text-white hover:bg-green-700 focus:ring-green-500'
              : 'bg-gray-100 text-gray-700 hover:bg-gray-200 focus:ring-gray-500',
            action.disabled && 'opacity-50 cursor-not-allowed'
          ]"
          :disabled="action.disabled || loading"
          @click="handleAction(action)"
        >
          <component
            v-if="action.icon"
            :is="action.icon"
            class="w-4 h-4 mr-2"
          />
          {{ loading === action.id ? 'Processing...' : action.label }}
        </button>
      </div>

      <!-- Divider -->
      <div class="h-6 w-px bg-gray-300" />

      <!-- Cancel Button -->
      <button
        class="text-sm text-gray-600 hover:text-gray-800 font-medium"
        @click="$emit('cancel')"
      >
        Cancel
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, type Component } from 'vue'

export interface BulkAction {
  id: string
  label: string
  variant?: 'default' | 'primary' | 'danger' | 'success'
  icon?: Component
  disabled?: boolean
  handler: (selectedIds: string[]) => Promise<void> | void
}

interface Props {
  selectedCount: number
  selectedIds: string[]
  availableActions: BulkAction[]
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'cancel'): void
}>()

const loading = ref<string | null>(null)

async function handleAction(action: BulkAction) {
  if (action.disabled || loading.value) return

  try {
    loading.value = action.id
    await action.handler(props.selectedIds)
  } finally {
    loading.value = null
  }
}
</script>
