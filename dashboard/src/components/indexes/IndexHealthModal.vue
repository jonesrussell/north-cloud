<template>
  <div
    class="fixed inset-0 z-50 overflow-y-auto"
    @click.self="$emit('close')"
  >
    <div class="flex items-center justify-center min-h-screen px-4">
      <div
        class="fixed inset-0 bg-gray-500 bg-opacity-75 transition-opacity"
        @click="$emit('close')"
      />
      <div class="relative bg-white rounded-lg shadow-xl max-w-2xl w-full p-6">
        <div class="flex justify-between items-center mb-4">
          <h2 class="text-xl font-semibold text-gray-900">
            Index Health: {{ index.name }}
          </h2>
          <button
            class="text-gray-400 hover:text-gray-500"
            @click="$emit('close')"
          >
            <span class="sr-only">Close</span>
            <XMarkIcon class="h-6 w-6" />
          </button>
        </div>

        <LoadingSpinner
          v-if="loading"
          text="Loading health data..."
        />

        <ErrorAlert
          v-else-if="error"
          :message="error"
        />

        <div
          v-else-if="health"
          class="space-y-4"
        >
          <div class="grid grid-cols-2 gap-4">
            <div class="bg-gray-50 p-4 rounded">
              <div class="text-sm text-gray-500">
                Status
              </div>
              <div class="mt-1">
                <StatusBadge
                  :status="health.health.status"
                  :custom-label="health.health.status.toUpperCase()"
                />
              </div>
            </div>

            <div class="bg-gray-50 p-4 rounded">
              <div class="text-sm text-gray-500">
                Total Shards
              </div>
              <div class="mt-1 text-lg font-semibold">
                {{ health.health.number_of_shards }}
              </div>
            </div>

            <div class="bg-gray-50 p-4 rounded">
              <div class="text-sm text-gray-500">
                Active Shards
              </div>
              <div class="mt-1 text-lg font-semibold text-green-600">
                {{ health.health.active_shards }}
              </div>
            </div>

            <div class="bg-gray-50 p-4 rounded">
              <div class="text-sm text-gray-500">
                Unassigned Shards
              </div>
              <div class="mt-1 text-lg font-semibold text-red-600">
                {{ health.health.unassigned_shards }}
              </div>
            </div>

            <div class="bg-gray-50 p-4 rounded">
              <div class="text-sm text-gray-500">
                Replicas
              </div>
              <div class="mt-1 text-lg font-semibold">
                {{ health.health.number_of_replicas }}
              </div>
            </div>

            <div class="bg-gray-50 p-4 rounded">
              <div class="text-sm text-gray-500">
                Initializing Shards
              </div>
              <div class="mt-1 text-lg font-semibold">
                {{ health.health.initializing_shards }}
              </div>
            </div>
          </div>

          <div class="flex justify-end mt-6">
            <button
              class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
              @click="$emit('close')"
            >
              Close
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { XMarkIcon } from '@heroicons/vue/24/outline'
import type { ApiError } from '../../types/common'
import { indexManagerApi } from '../../api/client'
import type { Index, IndexHealthResponse } from '../../types/indexManager'
import LoadingSpinner from '../common/LoadingSpinner.vue'
import ErrorAlert from '../common/ErrorAlert.vue'
import StatusBadge from '../common/StatusBadge.vue'

const props = defineProps<{
  index: Index
}>()

defineEmits(['close'])

const health = ref<IndexHealthResponse | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

const loadHealth = async (): Promise<void> => {
  loading.value = true
  error.value = null
  try {
    const response = await indexManagerApi.indexes.getHealth(props.index.name)
    health.value = response.data
  } catch (err: unknown) {
    const axiosError = err as ApiError
    error.value = axiosError.response?.data?.error || 'Failed to load health data'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadHealth()
})
</script>
