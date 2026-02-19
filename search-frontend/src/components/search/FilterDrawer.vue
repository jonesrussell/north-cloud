<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-40 lg:hidden"
      role="dialog"
      aria-modal="true"
      aria-label="Filter options"
    >
      <div
        class="fixed inset-0 bg-[var(--nc-text)]/40 backdrop-blur-sm transition-opacity"
        aria-hidden="true"
        @click="close"
      />

      <div
        id="filter-drawer"
        class="fixed inset-y-0 left-0 flex w-full max-w-sm flex-col bg-[var(--nc-bg-elevated)] shadow-[var(--nc-shadow-lg)]"
        @keydown.esc="close"
      >
        <div class="flex items-center justify-between border-b border-[var(--nc-border)] px-4 py-3">
          <h2 class="text-lg font-semibold text-[var(--nc-text)]">
            Filters
          </h2>
          <button
            type="button"
            class="rounded-lg p-2 text-[var(--nc-text-muted)] hover:bg-[var(--nc-bg-muted)] hover:text-[var(--nc-text)] focus:outline-none focus:ring-2 focus:ring-[var(--nc-primary)] transition-colors duration-[var(--nc-duration)]"
            aria-label="Close filters"
            @click="close"
          >
            <svg
              class="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
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
