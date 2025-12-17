<template>
  <div class="rounded-md bg-red-50 p-4">
    <div class="flex">
      <div class="flex-shrink-0">
        <ExclamationCircleIcon class="h-5 w-5 text-red-400" aria-hidden="true" />
      </div>
      <div class="ml-3">
        <h3 v-if="title" class="text-sm font-medium text-red-800">{{ title }}</h3>
        <div class="text-sm text-red-700" :class="{ 'mt-2': title }">
          <slot>{{ message }}</slot>
        </div>
        <div v-if="$slots.actions" class="mt-4">
          <div class="-mx-2 -my-1.5 flex">
            <slot name="actions"></slot>
          </div>
        </div>
      </div>
      <div v-if="dismissible" class="ml-auto pl-3">
        <div class="-mx-1.5 -my-1.5">
          <button
            type="button"
            class="inline-flex rounded-md bg-red-50 p-1.5 text-red-500 hover:bg-red-100 focus:outline-none focus:ring-2 focus:ring-red-600 focus:ring-offset-2 focus:ring-offset-red-50"
            @click="$emit('dismiss')"
          >
            <span class="sr-only">Dismiss</span>
            <XMarkIcon class="h-5 w-5" aria-hidden="true" />
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ExclamationCircleIcon, XMarkIcon } from '@heroicons/vue/24/outline'

defineProps({
  title: {
    type: String,
    default: '',
  },
  message: {
    type: String,
    default: '',
  },
  dismissible: {
    type: Boolean,
    default: false,
  },
})

defineEmits(['dismiss'])
</script>
