<template>
  <div>
    <div class="mb-6">
      <h2 class="text-2xl font-bold text-gray-900">Cities</h2>
      <p class="mt-1 text-sm text-gray-600">Cities configured for gopost integration</p>
    </div>

    <div v-if="loading" class="text-center py-12">
      <div class="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
      <p class="mt-2 text-sm text-gray-600">Loading cities...</p>
    </div>

    <div v-else-if="error" class="rounded-md bg-red-50 p-4 mb-4">
      <div class="flex">
        <ExclamationCircleIcon class="h-5 w-5 text-red-400" />
        <div class="ml-3">
          <h3 class="text-sm font-medium text-red-800">Error loading cities</h3>
          <div class="mt-2 text-sm text-red-700">{{ error }}</div>
        </div>
      </div>
    </div>

    <div v-else-if="cities.length === 0" class="text-center py-12 bg-white rounded-lg border border-gray-200">
      <MapPinIcon class="mx-auto h-12 w-12 text-gray-400" />
      <h3 class="mt-2 text-sm font-medium text-gray-900">No cities</h3>
      <p class="mt-1 text-sm text-gray-500">No enabled sources with city mappings found.</p>
    </div>

    <div v-else class="bg-white shadow overflow-hidden sm:rounded-md">
      <ul class="divide-y divide-gray-200">
        <li v-for="city in cities" :key="city.name" class="px-6 py-4">
          <div class="flex items-center justify-between">
            <div class="flex-1 min-w-0">
              <div class="flex items-center">
                <p class="text-sm font-medium text-gray-900">{{ city.name }}</p>
              </div>
              <div class="mt-1 flex items-center text-sm text-gray-500">
                <span>Index: {{ city.index }}</span>
                <span v-if="city.group_id" class="ml-4">
                  Group ID: <code class="text-xs bg-gray-100 px-1.5 py-0.5 rounded">{{ city.group_id }}</code>
                </span>
              </div>
            </div>
          </div>
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { citiesApi } from '../api/client'
import { ExclamationCircleIcon, MapPinIcon } from '@heroicons/vue/24/outline'

const cities = ref([])
const loading = ref(true)
const error = ref(null)

const loadCities = async () => {
  loading.value = true
  error.value = null
  try {
    cities.value = await citiesApi.list()
  } catch (err) {
    error.value = err.response?.data?.error || err.message || 'Failed to load cities'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadCities()
})
</script>

