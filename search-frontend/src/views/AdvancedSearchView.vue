<template>
  <div class="max-w-2xl mx-auto px-4 sm:px-6 lg:px-8 py-8 sm:py-12">
    <h1 class="font-display text-3xl font-normal text-[var(--nc-text)] mb-8">
      Advanced Search
    </h1>

    <div class="rounded-xl border border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] p-6 shadow-[var(--nc-shadow-sm)]">
      <form @submit.prevent="handleSubmit">
        <div class="space-y-6">
          <div>
            <label
              for="query"
              class="block text-sm font-medium text-[var(--nc-text)]"
            >
              Search query
            </label>
            <input
              id="query"
              v-model="formData.query"
              type="text"
              class="mt-1 block w-full rounded-lg border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] px-4 py-2.5 text-[var(--nc-text)] placeholder-[var(--nc-text-muted)] focus:border-[var(--nc-primary)] focus:ring-[var(--nc-primary)]"
              placeholder="Enter your search query"
            >
          </div>

          <div>
            <label class="block text-sm font-medium text-[var(--nc-text)] mb-2">Topics</label>
            <div class="space-y-2">
              <label class="inline-flex items-center mr-4 cursor-pointer">
                <input
                  v-model="formData.topics"
                  type="checkbox"
                  value="crime"
                  class="rounded border-[var(--nc-border)] text-[var(--nc-primary)] focus:ring-[var(--nc-primary)]"
                >
                <span class="ml-2 text-sm text-[var(--nc-text)]">Crime</span>
              </label>
              <label class="inline-flex items-center mr-4 cursor-pointer">
                <input
                  v-model="formData.topics"
                  type="checkbox"
                  value="local_news"
                  class="rounded border-[var(--nc-border)] text-[var(--nc-primary)] focus:ring-[var(--nc-primary)]"
                >
                <span class="ml-2 text-sm text-[var(--nc-text)]">Local News</span>
              </label>
            </div>
          </div>

          <div>
            <label
              for="quality"
              class="block text-sm font-medium text-[var(--nc-text)]"
            >
              Minimum quality score: {{ formData.min_quality_score }}
            </label>
            <input
              id="quality"
              v-model.number="formData.min_quality_score"
              type="range"
              min="0"
              max="100"
              step="10"
              class="mt-1 block w-full accent-[var(--nc-primary)]"
            >
          </div>

          <div class="flex justify-end gap-3 pt-2">
            <router-link
              to="/"
              class="inline-flex justify-center rounded-lg border border-[var(--nc-border)] bg-[var(--nc-bg-elevated)] px-4 py-2 text-sm font-medium text-[var(--nc-text)] hover:bg-[var(--nc-bg-muted)] focus:outline-none focus:ring-2 focus:ring-[var(--nc-primary)] transition-colors duration-[var(--nc-duration)]"
            >
              Cancel
            </router-link>
            <button
              type="submit"
              class="inline-flex justify-center rounded-lg border border-transparent bg-[var(--nc-accent)] px-4 py-2 text-sm font-medium text-white hover:bg-[var(--nc-accent-hover)] focus:outline-none focus:ring-2 focus:ring-[var(--nc-accent)] focus:ring-offset-2 transition-colors duration-[var(--nc-duration)]"
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

  const cleanQuery = Object.fromEntries(
    Object.entries(queryParams).filter(([, v]) => v !== undefined)
  )

  router.push({ path: '/search', query: cleanQuery })
}
</script>
