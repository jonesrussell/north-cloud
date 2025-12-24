<template>
  <span
    class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium"
    :class="badgeClass"
  >
    <span
      v-if="showDot"
      class="mr-1.5 h-2 w-2 rounded-full"
      :class="dotClass"
    />
    {{ label }}
  </span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: {
    type: String,
    required: true,
  },
  showDot: {
    type: Boolean,
    default: true,
  },
  customLabel: {
    type: String,
    default: '',
  },
})

const statusConfig = {
  // Job statuses
  pending: { bg: 'bg-yellow-100', text: 'text-yellow-800', dot: 'bg-yellow-400', label: 'Pending' },
  processing: { bg: 'bg-blue-100', text: 'text-blue-800', dot: 'bg-blue-400', label: 'Processing' },
  completed: { bg: 'bg-green-100', text: 'text-green-800', dot: 'bg-green-400', label: 'Completed' },
  failed: { bg: 'bg-red-100', text: 'text-red-800', dot: 'bg-red-400', label: 'Failed' },

  // Source statuses
  enabled: { bg: 'bg-green-100', text: 'text-green-800', dot: 'bg-green-400', label: 'Enabled' },
  disabled: { bg: 'bg-gray-100', text: 'text-gray-800', dot: 'bg-gray-400', label: 'Disabled' },

  // Generic statuses
  active: { bg: 'bg-green-100', text: 'text-green-800', dot: 'bg-green-400', label: 'Active' },
  inactive: { bg: 'bg-gray-100', text: 'text-gray-800', dot: 'bg-gray-400', label: 'Inactive' },
  error: { bg: 'bg-red-100', text: 'text-red-800', dot: 'bg-red-400', label: 'Error' },
  warning: { bg: 'bg-yellow-100', text: 'text-yellow-800', dot: 'bg-yellow-400', label: 'Warning' },
  info: { bg: 'bg-blue-100', text: 'text-blue-800', dot: 'bg-blue-400', label: 'Info' },

  // Default
  default: { bg: 'bg-gray-100', text: 'text-gray-800', dot: 'bg-gray-400', label: 'Unknown' },
}

const config = computed(() => {
  return statusConfig[props.status.toLowerCase()] || statusConfig.default
})

const badgeClass = computed(() => {
  return `${config.value.bg} ${config.value.text}`
})

const dotClass = computed(() => {
  return config.value.dot
})

const label = computed(() => {
  return props.customLabel || config.value.label
})
</script>
