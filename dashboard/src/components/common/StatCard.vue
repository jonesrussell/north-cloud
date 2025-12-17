<template>
  <div class="bg-white overflow-hidden shadow rounded-lg">
    <div class="p-5">
      <div class="flex items-center">
        <div v-if="icon" class="flex-shrink-0">
          <component
            :is="icon"
            class="h-6 w-6"
            :class="iconColorClass"
            aria-hidden="true"
          />
        </div>
        <div :class="{ 'ml-5': icon, 'w-full': !icon }">
          <dl>
            <dt class="text-sm font-medium text-gray-500 truncate">{{ label }}</dt>
            <dd class="mt-1 text-3xl font-semibold" :class="valueColorClass">
              {{ formattedValue }}
            </dd>
          </dl>
        </div>
      </div>
      <div v-if="$slots.footer" class="mt-4 pt-4 border-t border-gray-200">
        <slot name="footer"></slot>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  label: {
    type: String,
    required: true,
  },
  value: {
    type: [String, Number],
    required: true,
  },
  icon: {
    type: [Object, Function],
    default: null,
  },
  color: {
    type: String,
    default: 'gray',
    validator: (value) => ['gray', 'blue', 'green', 'red', 'yellow'].includes(value),
  },
  format: {
    type: String,
    default: 'number',
    validator: (value) => ['number', 'percent', 'text'].includes(value),
  },
})

const formattedValue = computed(() => {
  if (props.format === 'percent') {
    return `${props.value}%`
  }
  if (props.format === 'number' && typeof props.value === 'number') {
    return props.value.toLocaleString()
  }
  return props.value
})

const iconColorClass = computed(() => {
  const colors = {
    gray: 'text-gray-400',
    blue: 'text-blue-500',
    green: 'text-green-500',
    red: 'text-red-500',
    yellow: 'text-yellow-500',
  }
  return colors[props.color]
})

const valueColorClass = computed(() => {
  const colors = {
    gray: 'text-gray-900',
    blue: 'text-blue-600',
    green: 'text-green-600',
    red: 'text-red-600',
    yellow: 'text-yellow-600',
  }
  return colors[props.color]
})
</script>
