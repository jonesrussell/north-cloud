<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-40 lg:hidden"
      role="dialog"
      aria-modal="true"
      aria-label="Filter options"
    >
      <!-- Backdrop -->
      <div
        class="fixed inset-0 bg-gray-500/75 transition-opacity"
        aria-hidden="true"
        @click="close"
      />

      <!-- Drawer panel -->
      <div
        id="filter-drawer"
        class="fixed inset-y-0 left-0 flex w-full max-w-sm flex-col bg-white shadow-xl"
        @keydown.esc="close"
      >
        <div class="flex items-center justify-between border-b border-gray-200 px-4 py-3">
          <h2 class="text-lg font-semibold text-gray-900">
            Filters
          </h2>
          <button
            type="button"
            class="rounded-md p-2 text-gray-400 hover:bg-gray-100 hover:text-gray-600 focus:outline-none focus:ring-2 focus:ring-blue-500"
            aria-label="Close filters"
            @click="close"
          >
            <svg
              class="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        </div>
        <div class="flex-1 overflow-y-auto px-4 py-4">
          <slot />
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
defineProps<{
  open: boolean
}>()

const emit = defineEmits<{
  close: []
}>()

function close(): void {
  emit('close')
}
</script>
