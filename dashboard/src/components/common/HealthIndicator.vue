<template>
  <div class="inline-flex items-center gap-2">
    <!-- Icon with color and accessibility -->
    <span
      :class="[
        'flex items-center justify-center rounded-full',
        sizeClasses,
        iconColors
      ]"
      :aria-label="ariaLabel"
      role="status"
    >
      <component
        :is="iconComponent"
        :class="iconSizeClasses"
        aria-hidden="true"
      />
    </span>

    <!-- Label (optional) -->
    <span
      v-if="showLabel"
      :class="labelClasses"
    >
      {{ labelText }}
    </span>

    <!-- Tooltip (optional) -->
    <div
      v-if="tooltip"
      class="group relative inline-block"
    >
      <svg
        class="w-4 h-4 text-gray-400 hover:text-gray-600 cursor-help"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
        />
      </svg>
      <div class="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-2 px-3 py-2 bg-gray-900 text-white text-xs rounded-lg opacity-0 group-hover:opacity-100 transition-opacity pointer-events-none whitespace-nowrap z-10">
        {{ tooltip }}
        <div class="absolute top-full left-1/2 transform -translate-x-1/2 -mt-1 border-4 border-transparent border-t-gray-900" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import {
  CheckCircleIcon,
  ExclamationTriangleIcon,
  XCircleIcon,
  QuestionMarkCircleIcon,
  ClockIcon,
} from '@heroicons/vue/24/solid'

export type HealthStatus = 'healthy' | 'warning' | 'error' | 'unknown' | 'pending'

interface Props {
  status: HealthStatus
  label?: string
  tooltip?: string
  size?: 'sm' | 'md' | 'lg'
  showLabel?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  label: undefined,
  tooltip: undefined,
  size: 'md',
  showLabel: true,
})

// Icon mapping
const iconComponent = computed(() => {
  switch (props.status) {
    case 'healthy':
      return CheckCircleIcon
    case 'warning':
      return ExclamationTriangleIcon
    case 'error':
      return XCircleIcon
    case 'pending':
      return ClockIcon
    case 'unknown':
    default:
      return QuestionMarkCircleIcon
  }
})

// Size classes for container
const sizeClasses = computed(() => {
  switch (props.size) {
    case 'sm':
      return 'w-5 h-5'
    case 'lg':
      return 'w-8 h-8'
    case 'md':
    default:
      return 'w-6 h-6'
  }
})

// Size classes for icon
const iconSizeClasses = computed(() => {
  switch (props.size) {
    case 'sm':
      return 'w-3 h-3'
    case 'lg':
      return 'w-5 h-5'
    case 'md':
    default:
      return 'w-4 h-4'
  }
})

// Color classes (background + icon color)
const iconColors = computed(() => {
  switch (props.status) {
    case 'healthy':
      return 'bg-green-100 text-green-600'
    case 'warning':
      return 'bg-yellow-100 text-yellow-600'
    case 'error':
      return 'bg-red-100 text-red-600'
    case 'pending':
      return 'bg-blue-100 text-blue-600'
    case 'unknown':
    default:
      return 'bg-gray-100 text-gray-400'
  }
})

// Label text (use provided label or default status text)
const labelText = computed(() => {
  if (props.label) return props.label

  switch (props.status) {
    case 'healthy':
      return 'Healthy'
    case 'warning':
      return 'Warning'
    case 'error':
      return 'Error'
    case 'pending':
      return 'Pending'
    case 'unknown':
    default:
      return 'Unknown'
  }
})

// Label color classes
const labelClasses = computed(() => {
  const baseClasses = 'text-sm font-medium'

  switch (props.status) {
    case 'healthy':
      return `${baseClasses} text-green-700`
    case 'warning':
      return `${baseClasses} text-yellow-700`
    case 'error':
      return `${baseClasses} text-red-700`
    case 'pending':
      return `${baseClasses} text-blue-700`
    case 'unknown':
    default:
      return `${baseClasses} text-gray-500`
  }
})

// Accessibility label
const ariaLabel = computed(() => {
  if (props.tooltip) return props.tooltip
  return `Status: ${labelText.value}`
})
</script>
