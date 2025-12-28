<template>
  <div class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
    <h1 class="text-3xl font-bold text-gray-900 mb-8">Advanced Search</h1>

    <div class="bg-white shadow sm:rounded-lg p-6">
      <form @submit.prevent="handleSubmit">
        <div class="space-y-6">
          <!-- Query -->
          <div>
            <label for="query" class="block text-sm font-medium text-gray-700">Search Query</label>
            <input
              id="query"
              v-model="formData.query"
              type="text"
              class="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 sm:text-sm px-4 py-2 border"
              placeholder="Enter your search query"
            />
          </div>

          <!-- Topics -->
          <div>
            <label class="block text-sm font-medium text-gray-700 mb-2">Topics</label>
            <div class="space-y-2">
              <label class="inline-flex items-center mr-4">
                <input v-model="formData.topics" type="checkbox" value="crime" class="rounded border-gray-300 text-blue-600 shadow-sm focus:border-blue-500 focus:ring-blue-500" />
                <span class="ml-2 text-sm text-gray-700">Crime</span>
              </label>
              <label class="inline-flex items-center mr-4">
                <input v-model="formData.topics" type="checkbox" value="local_news" class="rounded border-gray-300 text-blue-600 shadow-sm focus:border-blue-500 focus:ring-blue-500" />
                <span class="ml-2 text-sm text-gray-700">Local News</span>
              </label>
            </div>
          </div>

          <!-- Quality Score -->
          <div>
            <label for="quality" class="block text-sm font-medium text-gray-700">
              Minimum Quality Score: {{ formData.min_quality_score }}
            </label>
            <input
              id="quality"
              v-model.number="formData.min_quality_score"
              type="range"
              min="0"
              max="100"
              step="10"
              class="mt-1 block w-full"
            />
          </div>

          <!-- Submit -->
          <div class="flex justify-end space-x-3">
            <router-link
              to="/"
              class="inline-flex justify-center rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
            >
              Cancel
            </router-link>
            <button
              type="submit"
              class="inline-flex justify-center rounded-md border border-transparent bg-blue-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
            >
              Search
            </button>
          </div>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

interface FormData {
  query: string
  topics: string[]
  min_quality_score: number
}

const router = useRouter()

const formData = ref<FormData>({
  query: '',
  topics: [],
  min_quality_score: 0,
})

const handleSubmit = (): void => {
  const queryParams: Record<string, string | number | undefined> = {
    q: formData.value.query || undefined,
    topics: formData.value.topics.length > 0 ? formData.value.topics.join(',') : undefined,
    min_quality_score: formData.value.min_quality_score > 0 ? formData.value.min_quality_score : undefined,
  }

  // Remove undefined values
  const cleanQuery = Object.fromEntries(
    Object.entries(queryParams).filter(([, v]) => v !== undefined)
  )

  router.push({ path: '/search', query: cleanQuery })
}
</script>
