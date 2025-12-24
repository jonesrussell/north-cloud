<template>
  <div class="border-t border-gray-200 pt-6">
    <button
      type="button"
      class="flex w-full items-center justify-between text-left"
      @click="$emit('update:open', !open)"
    >
      <h3 class="text-lg font-medium text-gray-900">
        {{ title }}
      </h3>
      <ChevronDownIcon
        :class="['h-5 w-5 text-gray-500 transition-transform', open ? 'rotate-180' : '']"
      />
    </button>
    <div
      v-if="open"
      class="mt-4"
    >
      <slot />
    </div>
  </div>
</template>

<script setup>
import { ChevronDownIcon } from '@heroicons/vue/24/outline'

defineProps({
  title: {
    type: String,
    required: true,
  },
  open: {
    type: Boolean,
    default: false,
  },
})

defineEmits(['update:open'])
</script>

