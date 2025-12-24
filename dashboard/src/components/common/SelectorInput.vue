<template>
  <div>
    <label
      :for="inputId"
      class="block text-sm font-medium text-gray-700"
    >
      {{ label }}
    </label>
    <input
      :id="inputId"
      :value="modelValue"
      type="text"
      :placeholder="placeholder"
      class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm"
      @input="$emit('update:modelValue', $event.target.value)"
    >
    <p
      v-if="hint"
      class="mt-1 text-xs text-gray-500"
    >
      {{ hint }}
    </p>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  modelValue: {
    type: String,
    default: '',
  },
  label: {
    type: String,
    default: '',
  },
  placeholder: {
    type: String,
    default: '',
  },
  hint: {
    type: String,
    default: '',
  },
})

defineEmits(['update:modelValue'])

const inputId = computed(() => `selector-${props.label.toLowerCase().replace(/\s+/g, '-')}`)
</script>

