<template>
  <div>
    <PageHeader
      title="Cities"
      subtitle="Cities configured for publishing integration"
    />

    <!-- Loading State -->
    <LoadingSpinner v-if="loading" size="lg" text="Loading cities..." :full-page="true" />

    <!-- Error State -->
    <ErrorAlert v-else-if="error" title="Error loading cities" :message="error" class="mb-6" />

    <!-- Empty State -->
    <div v-else-if="cities.length === 0" class="text-center py-12 bg-white rounded-lg border border-gray-200">
      <MapPinIcon class="mx-auto h-12 w-12 text-gray-400" />
      <h3 class="mt-2 text-sm font-medium text-gray-900">No cities</h3>
      <p class="mt-1 text-sm text-gray-500">No enabled sources with city mappings found.</p>
      <div class="mt-6">
        <router-link
          to="/sources/new"
          class="text-sm font-medium text-blue-600 hover:text-blue-500"
        >
          Add a source with city mapping &rarr;
        </router-link>
      </div>
    </div>

    <!-- Cities List -->
    <div v-else class="bg-white shadow overflow-hidden sm:rounded-md">
      <ul class="divide-y divide-gray-200">
        <li v-for="city in cities" :key="city.name" class="px-6 py-4 hover:bg-gray-50">
          <div class="flex items-center justify-between">
            <div class="flex-1 min-w-0">
              <div class="flex items-center">
                <MapPinIcon class="h-5 w-5 text-gray-400 mr-2" />
                <p class="text-sm font-medium text-gray-900">{{ city.name }}</p>
              </div>
              <div class="mt-2 flex items-center text-sm text-gray-500 space-x-4">
                <span class="flex items-center">
                  <span class="text-gray-400 mr-1">Index:</span>
                  <code class="text-xs bg-gray-100 px-1.5 py-0.5 rounded font-mono">{{ city.index }}</code>
                </span>
                <span v-if="city.group_id" class="flex items-center">
                  <span class="text-gray-400 mr-1">Group ID:</span>
                  <code class="text-xs bg-gray-100 px-1.5 py-0.5 rounded font-mono">{{ city.group_id }}</code>
                </span>
              </div>
            </div>
          </div>
        </li>
      </ul>
    </div>

    <!-- Info Section -->
    <div class="mt-6 bg-blue-50 rounded-lg p-4">
      <div class="flex">
        <InformationCircleIcon class="h-5 w-5 text-blue-400 mt-0.5" />
        <div class="ml-3">
          <h3 class="text-sm font-medium text-blue-800">About Cities</h3>
          <p class="mt-1 text-sm text-blue-700">
            Cities are derived from enabled sources that have city mappings configured.
            To add a new city, create or edit a source and set the city name and group ID.
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { MapPinIcon, InformationCircleIcon } from '@heroicons/vue/24/outline'
import { sourcesApi } from '../../api/client'
import { PageHeader, LoadingSpinner, ErrorAlert } from '../../components/common'

const cities = ref([])
const loading = ref(true)
const error = ref(null)

const loadCities = async () => {
  loading.value = true
  error.value = null
  try {
    const response = await sourcesApi.cities.list()
    cities.value = response.data?.cities || response.data || []
  } catch (err) {
    error.value = err.response?.data?.error || err.message || 'Failed to load cities'
    console.error('[CitiesView] Error loading cities:', err)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadCities()
})
</script>
