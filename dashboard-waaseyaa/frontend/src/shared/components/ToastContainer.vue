<script setup lang="ts">
import { useToast } from '../composables/useToast'

const { toasts } = useToast()

function dismiss(id: number) {
  const { toasts: t } = useToast()
  t.value = t.value.filter((toast) => toast.id !== id)
}

function iconFor(type: 'success' | 'error' | 'info'): string {
  switch (type) {
    case 'success':
      return '\u2713'
    case 'error':
      return '\u2717'
    case 'info':
      return '\u2139'
  }
}
</script>

<template>
  <Teleport to="body">
    <div class="fixed top-4 right-4 z-50 flex flex-col gap-3 pointer-events-none">
      <TransitionGroup
        enter-active-class="transition duration-300 ease-out"
        enter-from-class="translate-x-full opacity-0"
        enter-to-class="translate-x-0 opacity-100"
        leave-active-class="transition duration-200 ease-in"
        leave-from-class="translate-x-0 opacity-100"
        leave-to-class="translate-x-full opacity-0"
      >
        <div
          v-for="toast in toasts"
          :key="toast.id"
          class="pointer-events-auto flex items-center gap-3 min-w-72 max-w-sm rounded-lg border px-4 py-3 shadow-lg"
          :class="{
            'bg-green-900/30 border-green-800 text-green-300': toast.type === 'success',
            'bg-red-900/30 border-red-800 text-red-300': toast.type === 'error',
            'bg-blue-900/30 border-blue-800 text-blue-300': toast.type === 'info',
          }"
        >
          <span class="text-base font-bold shrink-0">{{ iconFor(toast.type) }}</span>
          <p class="text-sm flex-1">{{ toast.message }}</p>
          <button
            @click="dismiss(toast.id)"
            class="shrink-0 opacity-60 hover:opacity-100 text-sm font-medium"
            aria-label="Dismiss notification"
          >
            &times;
          </button>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>
