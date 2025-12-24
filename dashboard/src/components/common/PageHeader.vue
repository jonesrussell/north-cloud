<template>
  <div class="mb-6">
    <div class="flex items-center justify-between">
      <div>
        <div
          v-if="backLink"
          class="mb-2"
        >
          <router-link
            :to="backLink"
            class="inline-flex items-center text-sm text-gray-500 hover:text-gray-700"
          >
            <ArrowLeftIcon class="mr-1 h-4 w-4" />
            {{ backText }}
          </router-link>
        </div>
        <h1 class="text-2xl font-bold text-gray-900">
          {{ title }}
        </h1>
        <p
          v-if="subtitle"
          class="mt-1 text-sm text-gray-600"
        >
          {{ subtitle }}
        </p>
      </div>
      <div v-if="$slots.actions">
        <slot name="actions" />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ArrowLeftIcon } from '@heroicons/vue/24/outline'

defineProps({
  title: {
    type: String,
    required: true,
  },
  subtitle: {
    type: String,
    default: '',
  },
  backLink: {
    type: [String, Object],
    default: null,
  },
  backText: {
    type: String,
    default: 'Back',
  },
})
</script>
