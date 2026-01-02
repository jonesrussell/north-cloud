<template>
  <div class="bg-white shadow rounded-lg p-6">
    <div class="flex items-center justify-between mb-4">
      <h2 class="text-lg font-medium text-gray-900">
        {{ title }}
      </h2>
      <div
        v-if="completionPercentage !== null"
        class="text-sm font-medium text-gray-600"
      >
        {{ completionPercentage }}% Complete
      </div>
    </div>

    <!-- Progress bar (if percentage provided) -->
    <div
      v-if="completionPercentage !== null"
      class="mb-4"
    >
      <div class="w-full bg-gray-200 rounded-full h-2">
        <div
          class="h-2 rounded-full transition-all duration-300"
          :class="progressColor"
          :style="{ width: `${completionPercentage}%` }"
        />
      </div>
    </div>

    <!-- Steps/Items List -->
    <div class="space-y-3">
      <div
        v-for="(step, index) in steps"
        :key="index"
        class="flex items-start"
      >
        <!-- Status Icon -->
        <div class="flex-shrink-0 mr-3 mt-0.5">
          <HealthIndicator
            :status="step.status"
            :show-label="false"
            size="sm"
          />
        </div>

        <!-- Step Content -->
        <div class="flex-1 min-w-0">
          <p
            class="text-sm font-medium"
            :class="stepTextColor(step.status)"
          >
            {{ step.label }}
          </p>
          <p
            v-if="step.description"
            class="text-xs text-gray-500 mt-0.5"
          >
            {{ step.description }}
          </p>
          <!-- Sub-warnings/errors -->
          <div
            v-if="step.warning"
            class="mt-1 text-xs text-yellow-600 flex items-start"
          >
            <svg
              class="w-3 h-3 mr-1 mt-0.5 flex-shrink-0"
              fill="currentColor"
              viewBox="0 0 20 20"
            >
              <path
                fill-rule="evenodd"
                d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z"
                clip-rule="evenodd"
              />
            </svg>
            {{ step.warning }}
          </div>
        </div>

        <!-- Action Button (optional) -->
        <button
          v-if="step.action"
          class="flex-shrink-0 ml-3 text-xs font-medium text-blue-600 hover:text-blue-800"
          @click="step.action.handler"
        >
          {{ step.action.label }}
        </button>
      </div>
    </div>

    <!-- Actions Footer -->
    <div
      v-if="actions && actions.length > 0"
      class="mt-6 pt-4 border-t border-gray-200 flex gap-3"
    >
      <button
        v-for="(action, index) in actions"
        :key="index"
        :class="[
          'px-4 py-2 rounded-md text-sm font-medium focus:outline-none focus:ring-2 focus:ring-offset-2',
          action.primary
            ? 'bg-blue-600 text-white hover:bg-blue-700 focus:ring-blue-500'
            : 'border border-gray-300 text-gray-700 bg-white hover:bg-gray-50 focus:ring-blue-500'
        ]"
        @click="action.handler"
      >
        {{ action.label }}
      </button>
    </div>

    <!-- Empty State -->
    <div
      v-if="!steps || steps.length === 0"
      class="text-center py-8 text-gray-500"
    >
      <svg
        class="mx-auto h-12 w-12 text-gray-400"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
        />
      </svg>
      <p class="mt-2 text-sm">
        All set! No action needed.
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import HealthIndicator from './HealthIndicator.vue'
import type { HealthStatus } from './HealthIndicator.vue'

interface SetupStep {
  label: string
  description?: string
  status: HealthStatus
  warning?: string
  action?: {
    label: string
    handler: () => void
  }
}

interface Action {
  label: string
  primary?: boolean
  handler: () => void
}

interface Props {
  title: string
  steps?: SetupStep[]
  actions?: Action[]
  completionPercentage?: number | null
}

const props = withDefaults(defineProps<Props>(), {
  steps: () => [],
  actions: () => [],
  completionPercentage: null,
})

// Progress bar color based on completion
const progressColor = computed(() => {
  if (props.completionPercentage === null) return 'bg-blue-600'
  if (props.completionPercentage === 100) return 'bg-green-600'
  if (props.completionPercentage >= 75) return 'bg-blue-600'
  if (props.completionPercentage >= 50) return 'bg-yellow-500'
  return 'bg-red-500'
})

// Step text color based on status
const stepTextColor = (status: HealthStatus): string => {
  switch (status) {
    case 'healthy':
      return 'text-gray-700'
    case 'warning':
      return 'text-yellow-700'
    case 'error':
      return 'text-red-700'
    case 'pending':
      return 'text-blue-700'
    case 'unknown':
    default:
      return 'text-gray-500'
  }
}
</script>
